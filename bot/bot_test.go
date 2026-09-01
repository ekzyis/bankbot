package bot

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ekzyis/ccbank/bank"
	"github.com/ekzyis/ccbank/internal/lntest"
	"github.com/ekzyis/ccbank/ln"
	"github.com/ekzyis/ccbank/sn"
	"github.com/ekzyis/lnpilot/lightning/lntypes"
	"gopkg.in/guregu/null.v4"
)

type fakeSN struct {
	name        string
	mentions    []sn.Notification
	mentionsErr error
	zaps        []sn.Notification
	zapsErr     error
	meErr       error
	commentErr  error
	replies     []struct {
		parentID int
		text     string
	}
}

func (f *fakeSN) Me() (*sn.User, error) {
	if f.meErr != nil {
		return nil, f.meErr
	}
	name := f.name
	if name == "" {
		name = "ccbank"
	}
	return &sn.User{Name: name}, nil
}

func (f *fakeSN) Mentions() ([]sn.Notification, error) { return f.mentions, f.mentionsErr }

func (f *fakeSN) Zaps() ([]sn.Notification, error) { return f.zaps, f.zapsErr }

func (f *fakeSN) CreateComment(parentID int, text string) (int, error) {
	if f.commentErr != nil {
		return 0, f.commentErr
	}
	f.replies = append(f.replies, struct {
		parentID int
		text     string
	}{parentID, text})
	return len(f.replies), nil
}

type fakeNotifier struct {
	calls []notification
}

func (f *fakeNotifier) Notify(title, body, click string, tags ...string) error {
	f.calls = append(f.calls, notification{title: title, body: body, click: click, tags: tags})
	return nil
}

type fakeBalancer struct {
	credits int
	err     error
}

func (f *fakeBalancer) Credits() (int, error) { return f.credits, f.err }

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func newTestBot(t *testing.T) (*Bot, *fakeSN, *fakeNotifier, *fakeBalancer) {
	t.Helper()
	fsn := &fakeSN{}
	fn := &fakeNotifier{}
	fbal := &fakeBalancer{}
	bot, err := NewBot("https://stacker.news", fsn, fn, bank.NewPricer(), fbal, discardLogger())
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	return bot, fsn, fn, fbal
}

func mention(id, itemID int, author, text string) sn.Notification {
	return sn.Notification{
		Id:   id,
		Type: "Mention",
		Item: sn.Item{Id: itemID, Text: text, User: sn.User{Name: author}},
	}
}

func TestPoll_ValidInvoice_QuotesAndNotifies(t *testing.T) {
	inv := lntest.MakeInvoice(t, 2500000) // 2500 sats
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(1, 100, "alice", "@ccbank sell "+inv)}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	if len(fsn.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(fsn.replies))
	}
	if fsn.replies[0].parentID != 100 {
		t.Errorf("reply parentID = %d, want 100", fsn.replies[0].parentID)
	}
	// 2500 sats * 2 credits/sat = 5000 credits to receive; grossed up for the
	// ~30% SN zap fee = ceil(5000 * 10 / 7) = 7143 credits to send.
	if !strings.Contains(fsn.replies[0].text, "Zap me 7,143 credits") {
		t.Errorf("unexpected quote: %q", fsn.replies[0].text)
	}
	if !strings.Contains(fsn.replies[0].text, "2,500 sats lightning invoice") {
		t.Errorf("quote should reference the invoice amount: %q", fsn.replies[0].text)
	}
	if !strings.Contains(fsn.replies[0].text, "bot receives 5,000 credits") {
		t.Errorf("quote should show the received-credits breakdown: %q", fsn.replies[0].text)
	}
	// Seller's effective rate: 7143 credits / 2500 sats = 2.857 credits/sat.
	if !strings.Contains(fsn.replies[0].text, "2.857 credits/sat") {
		t.Errorf("quote should show the seller's effective rate: %q", fsn.replies[0].text)
	}
	if !strings.Contains(fsn.replies[0].text, "payments can take up to 24h") {
		t.Errorf("quote should warn about the payout delay: %q", fsn.replies[0].text)
	}
	if len(fn.calls) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(fn.calls))
	}
	if !strings.Contains(fn.calls[0].body, "payment_hash=") {
		t.Errorf("notification should include the payment hash; got %q", fn.calls[0].body)
	}
	if !strings.Contains(fn.calls[0].body, "7143/5000 credits") {
		t.Errorf("notification should include the credit figures; got %q", fn.calls[0].body)
	}
	if fn.calls[0].click != "https://stacker.news/items/100" {
		t.Errorf("click = %q", fn.calls[0].click)
	}
}

func votification(id, itemID, sats int) sn.Notification {
	return sn.Notification{Id: id, Type: "Votification", Item: sn.Item{Id: itemID, Sats: sats}}
}

func quoteAndTrack(t *testing.T) (*Bot, *fakeSN, *fakeNotifier, string) {
	t.Helper()
	inv := lntest.MakeInvoice(t, 2500000)
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(1, 100, "alice", "@ccbank sell "+inv)}
	if err := bot.Poll(); err != nil {
		t.Fatalf("quote poll: %v", err)
	}
	if len(fn.calls) != 1 {
		t.Fatalf("expected the quote notification, got %d", len(fn.calls))
	}
	return bot, fsn, fn, inv
}

func TestPoll_Zap_NotifiesWhenFunded(t *testing.T) {
	bot, fsn, fn, _ := quoteAndTrack(t)

	fsn.zaps = []sn.Notification{votification(5, 1, 7143)} // covers the 7143 requested
	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fn.calls) != 2 {
		t.Fatalf("expected a funded-request notification, got %d calls", len(fn.calls))
	}
	paid := fn.calls[1]
	if !strings.Contains(paid.title, "7,143/7,143 credits") {
		t.Errorf("paid title should carry the amounts; got %q", paid.title)
	}
	if !strings.Contains(paid.title, "@alice") {
		t.Errorf("paid title should name payer")
	}
	if !strings.Contains(paid.body, "payment_hash=") {
		t.Errorf("paid notification should include payment hash; got %q", paid.body)
	}
	if paid.click != "https://stacker.news/items/100" {
		t.Errorf("click should link the original item; got %q", paid.click)
	}

	// SN reuses the same zap id; an already-funded, now-untracked reply is not re-notified.
	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll 2: %v", err)
	}
	if len(fn.calls) != 2 {
		t.Errorf("expected the request handled once, got %d calls", len(fn.calls))
	}
}

func TestPoll_Zap_NotifiesAfterTopUp(t *testing.T) {
	bot, fsn, fn, _ := quoteAndTrack(t)

	// Partial zap: one short of the 7143 requested.
	fsn.zaps = []sn.Notification{votification(5, 1, 7142)}
	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll (partial): %v", err)
	}
	if len(fn.calls) != 1 {
		t.Fatalf("expected silence while underfunded, got %d calls", len(fn.calls))
	}

	// Top-up: SN reuses the same notification id, now over the threshold.
	fsn.zaps = []sn.Notification{votification(5, 1, 7143)}
	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll (top-up): %v", err)
	}
	if len(fn.calls) != 2 {
		t.Errorf("expected a notification once funded via top-up, got %d calls", len(fn.calls))
	}
}

func TestPoll_AmountlessInvoice_ErrorsNoNotify(t *testing.T) {
	inv := lntest.MakeInvoice(t, 0)
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(2, 200, "bob", "@ccbank sell "+inv)}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, "must have an amount") {
		t.Errorf("expected amount-required error reply, got %+v", fsn.replies)
	}
	if len(fn.calls) != 0 {
		t.Errorf("expected no notification for invalid request, got %d", len(fn.calls))
	}
}

func TestPoll_SellNonInvoiceArg_Errors(t *testing.T) {
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(3, 300, "carol", "@ccbank sell foo")}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, "invoice not found") {
		t.Errorf("expected no-invoice error reply, got %+v", fsn.replies)
	}
	if len(fn.calls) != 0 {
		t.Errorf("expected no notification, got %d", len(fn.calls))
	}
}

func TestPoll_MalformedSell_RepliesHelp(t *testing.T) {
	inv := lntest.MakeInvoice(t, 0)
	for _, text := range []string{
		"@ccbank sell",               // missing invoice
		"@ccbank sell " + inv + " x", // trailing junk
		"@ccbank " + inv,             // missing command
		"sell " + inv,                // mention not first
		"@notbank sell " + inv,       // wrong account mentioned
	} {
		bot, fsn, fn, _ := newTestBot(t)
		fsn.mentions = []sn.Notification{mention(50, 5000, "peg", text)}
		if err := bot.Poll(); err != nil {
			t.Fatalf("Poll(%q): %v", text, err)
		}
		if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, "Usage:") {
			t.Errorf("text %q: expected help reply, got %+v", text, fsn.replies)
		}
		if len(fn.calls) != 0 {
			t.Errorf("text %q: expected no notification, got %d", text, len(fn.calls))
		}
	}
}

func TestPoll_SellSurroundingWhitespace_OK(t *testing.T) {
	inv := lntest.MakeInvoice(t, 2500000)
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(51, 5100, "quinn", "   @ccbank    sell    "+inv+"   ")}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, "Zap me 7,143 credits") {
		t.Errorf("expected a quote despite extra whitespace, got %+v", fsn.replies)
	}
	if len(fn.calls) != 1 {
		t.Errorf("expected a notification, got %d", len(fn.calls))
	}
}

func TestPoll_ExpiresSoon_Errors(t *testing.T) {
	inv := lntest.MakeInvoiceExpiry(t, 2500000, lntypes.NetworkMainnet, 30*time.Minute)
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(41, 4100, "trent", "@ccbank sell "+inv)}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	wantMin := fmt.Sprintf("at least %dh", int(ln.MinInvoiceValidity.Hours()))
	if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, wantMin) {
		t.Errorf("expected expires-soon reply, got %+v", fsn.replies)
	}
	if len(fn.calls) != 0 {
		t.Errorf("expected no notification, got %d", len(fn.calls))
	}
}

func TestPoll_BareMention_RepliesHelp(t *testing.T) {
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(40, 4000, "oscar", "@ccbank gm")}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, "Usage:") {
		t.Errorf("expected help reply, got %+v", fsn.replies)
	}
	if len(fn.calls) != 0 {
		t.Errorf("expected no notification for a bare mention, got %d", len(fn.calls))
	}
}

func TestPoll_Deduplicates(t *testing.T) {
	inv := lntest.MakeInvoice(t, 2500000)
	bot, fsn, _, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(4, 400, "dave", "@ccbank sell "+inv)}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll 1: %v", err)
	}
	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll 2: %v", err)
	}
	if len(fsn.replies) != 1 {
		t.Errorf("expected mention handled once, got %d replies", len(fsn.replies))
	}
}

func TestPoll_TreasuryFull_Rejects(t *testing.T) {
	inv := lntest.MakeInvoice(t, 2500000)
	bot, fsn, fn, fbal := newTestBot(t)
	fbal.credits = bank.TreasuryTarget
	fsn.mentions = []sn.Notification{mention(8, 800, "heidi", "@ccbank sell "+inv)}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, "not accepting credits") {
		t.Errorf("expected treasury-full reply, got %+v", fsn.replies)
	}
	if len(fn.calls) != 0 {
		t.Errorf("expected no notification when treasury is full, got %d", len(fn.calls))
	}
}

func TestPoll_NearTreasuryLimit_RejectsOverReducedMax(t *testing.T) {
	// Leave only enough treasury headroom for half the per-exchange cap, then ask
	// for an invoice larger than that reduced max.
	reduced := bank.MaxSats / 2
	inv := lntest.MakeInvoice(t, uint64(reduced+1000)*1000)
	bot, fsn, fn, fbal := newTestBot(t)
	fbal.credits = bank.TreasuryTarget - reduced*bank.DefaultRate/bank.RateScale
	fsn.mentions = []sn.Notification{mention(30, 3000, "peggy", "@ccbank sell "+inv)}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(fsn.replies))
	}
	wantMax := "at most " + formatSats(reduced)
	if !strings.Contains(fsn.replies[0].text, wantMax) {
		t.Errorf("reply should state the reduced max %q, got %q", wantMax, fsn.replies[0].text)
	}
	if len(fn.calls) != 0 {
		t.Errorf("expected no notification for an over-cap invoice, got %d", len(fn.calls))
	}
}

func TestPoll_ExceedsMax_Rejects(t *testing.T) {
	inv := lntest.MakeInvoice(t, uint64(bank.MaxSats+5000)*1000) // above the per-exchange cap
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(31, 3100, "rupert", "@ccbank sell "+inv)}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, "at most "+formatSats(bank.MaxSats)) {
		t.Errorf("expected over-cap rejection stating the max, got %+v", fsn.replies)
	}
	if len(fn.calls) != 0 {
		t.Errorf("expected no notification for an over-cap invoice, got %d", len(fn.calls))
	}
}

func TestPoll_BelowMin_Rejects(t *testing.T) {
	inv := lntest.MakeInvoice(t, uint64(bank.MinSats-1)*1000) // below the per-exchange floor
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(32, 3200, "sybil", "@ccbank sell "+inv)}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, "at least "+formatSats(bank.MinSats)) {
		t.Errorf("expected below-min rejection stating the floor, got %+v", fsn.replies)
	}
	if len(fn.calls) != 0 {
		t.Errorf("expected no notification for a below-min invoice, got %d", len(fn.calls))
	}
}

func TestPoll_CreditsError_RetriesLater(t *testing.T) {
	inv := lntest.MakeInvoice(t, 2500000)
	bot, fsn, fn, fbal := newTestBot(t)
	fbal.err = errors.New("boom")
	fsn.mentions = []sn.Notification{mention(9, 900, "ivan", "@ccbank sell "+inv)}

	if err := bot.Poll(); err == nil {
		t.Fatal("expected Poll to return an error when credits fetch fails")
	}
	if len(fsn.replies) != 0 || len(fn.calls) != 0 {
		t.Errorf("expected no action when credits fetch fails")
	}

	// Recover: next poll succeeds and handles the mention.
	fbal.err = nil
	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll after recovery: %v", err)
	}
	if len(fsn.replies) != 1 {
		t.Errorf("expected mention handled after recovery, got %d replies", len(fsn.replies))
	}
}

func TestPoll_InvalidInvoice_Errors(t *testing.T) {
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(20, 2000, "judy", "@ccbank sell lnbc1bogusxyz")}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, "failed to decode invoice") {
		t.Errorf("expected invalid-invoice reply, got %+v", fsn.replies)
	}
	if len(fn.calls) != 0 {
		t.Errorf("expected no notification, got %d", len(fn.calls))
	}
}

func TestPoll_WrongNetwork_Errors(t *testing.T) {
	inv := lntest.MakeInvoiceNet(t, 2500000, lntypes.NetworkTestnet)
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{mention(21, 2100, "mallory", "@ccbank sell "+inv)}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 || !strings.Contains(fsn.replies[0].text, "mainnet") {
		t.Errorf("expected mainnet-only reply, got %+v", fsn.replies)
	}
	if len(fn.calls) != 0 {
		t.Errorf("expected no notification, got %d", len(fn.calls))
	}
}

func TestNewBot_MentionsError(t *testing.T) {
	fsn := &fakeSN{mentionsErr: errors.New("boom")}
	_, err := NewBot("https://stacker.news", fsn, &fakeNotifier{}, bank.NewPricer(), &fakeBalancer{}, discardLogger())
	if err == nil {
		t.Fatal("expected NewBot to fail when the baseline fetch errors")
	}
}

func TestNewBot_MeError(t *testing.T) {
	fsn := &fakeSN{meErr: errors.New("boom")}
	_, err := NewBot("https://stacker.news", fsn, &fakeNotifier{}, bank.NewPricer(), &fakeBalancer{}, discardLogger())
	if err == nil {
		t.Fatal("expected NewBot to fail when fetching the bot identity errors")
	}
}

func TestPoll_MentionsError(t *testing.T) {
	bot, fsn, _, _ := newTestBot(t)
	fsn.mentionsErr = errors.New("boom")
	if err := bot.Poll(); err == nil {
		t.Fatal("expected Poll to return an error when fetching mentions fails")
	}
}

func TestPoll_PostReplyError_RetriesLater(t *testing.T) {
	inv := lntest.MakeInvoice(t, 2500000)
	bot, fsn, _, _ := newTestBot(t)
	fsn.commentErr = errors.New("post failed")
	fsn.mentions = []sn.Notification{mention(22, 2200, "niaj", "@ccbank sell "+inv)}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 0 {
		t.Errorf("expected no successful reply, got %d", len(fsn.replies))
	}

	// The mention was not marked handled, so it is retried once posting recovers.
	fsn.commentErr = nil
	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll after recovery: %v", err)
	}
	if len(fsn.replies) != 1 {
		t.Errorf("expected mention handled after recovery, got %d replies", len(fsn.replies))
	}
}

func TestPoll_SkipsDeleted(t *testing.T) {
	m := mention(5, 500, "erin", "@ccbank whatever")
	m.Item.DeletedAt = null.TimeFrom(m.Item.CreatedAt)
	bot, fsn, fn, _ := newTestBot(t)
	fsn.mentions = []sn.Notification{m}

	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 0 || len(fn.calls) != 0 {
		t.Errorf("expected no action for deleted item")
	}
}

func TestNewBot_BaselinesExistingMentions(t *testing.T) {
	inv := lntest.MakeInvoice(t, 2500000)
	fsn := &fakeSN{mentions: []sn.Notification{mention(6, 600, "frank", "@ccbank sell "+inv)}}
	fn := &fakeNotifier{}

	// The mention exists at construction, so the baseline should mark it handled.
	bot, err := NewBot("https://stacker.news", fsn, fn, bank.NewPricer(), &fakeBalancer{}, discardLogger())
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 0 {
		t.Errorf("pre-existing mention should not be replied to, got %d replies", len(fsn.replies))
	}

	// A mention arriving after the baseline (higher ID) should be handled.
	fsn.mentions = append(fsn.mentions, mention(7, 700, "grace", "@ccbank sell "+inv))
	if err := bot.Poll(); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(fsn.replies) != 1 || fsn.replies[0].parentID != 700 {
		t.Errorf("post-baseline mention should be replied to, got %+v", fsn.replies)
	}
}

func TestCommas(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{12345, "12,345"},
		{1234567, "1,234,567"},
		{-1000, "-1,000"},
	}
	for _, c := range cases {
		if got := commas(c.n); got != c.want {
			t.Errorf("commas(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
