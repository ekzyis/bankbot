package bot

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/ekzyis/ccbank/bank"
	"github.com/ekzyis/ccbank/ln"
	"github.com/ekzyis/ccbank/sn"
)

type SNClient interface {
	Me() (*sn.User, error)
	Mentions() ([]sn.Notification, error)
	Zaps() ([]sn.Notification, error)
	CreateComment(parentID int, text string) (int, error)
}

type Notifier interface {
	Notify(title, body, click string, tags ...string) error
}

type Balancer interface {
	Credits() (int, error)
}

type Bot struct {
	baseURL  string
	client   SNClient
	notify   Notifier
	pricer   bank.Pricer
	balance  Balancer
	log      *slog.Logger
	name     string
	lastSeen int
	// bot replies we're tracking for zaps to notify operator
	tracking map[int]tracked
}

// tracked is a bot reply awaiting payment.
type tracked struct {
	requested   int    // credits the seller must send
	invoice     string // lightning invoice to pay once funded
	author      string // who requested the exchange
	paymentHash string // invoice payment hash, hex
	itemURL     string // the original request item, for the notification click
}

func NewBot(baseURL string, client SNClient, n Notifier, pricer bank.Pricer, balance Balancer, logger *slog.Logger) (*Bot, error) {
	if logger == nil {
		logger = slog.Default()
	}
	b := &Bot{baseURL: baseURL, client: client, notify: n, pricer: pricer, balance: balance, log: logger, tracking: make(map[int]tracked)}

	me, err := b.client.Me()
	if err != nil {
		return nil, fmt.Errorf("fetch me: %w", err)
	}
	b.name = me.Name

	mentions, err := b.client.Mentions()
	if err != nil {
		return nil, fmt.Errorf("fetch mentions: %w", err)
	}
	for _, m := range mentions {
		if m.Id > b.lastSeen {
			b.lastSeen = m.Id
		}
	}

	b.log.Info("bot ready", "name", b.name, "last_mention", b.lastSeen, "mentions", len(mentions))
	return b, nil
}

func (b *Bot) Poll() error {
	mentions, err := b.client.Mentions()
	if err != nil {
		return fmt.Errorf("fetch mentions: %w", err)
	}

	sort.Slice(mentions, func(i, j int) bool { return mentions[i].Id < mentions[j].Id })

	credits, fetched := 0, false
	for _, m := range mentions {
		if m.Id <= b.lastSeen {
			continue
		}
		if !fetched {
			c, err := b.balance.Credits()
			if err != nil {
				// not knowing our treasury balance is a fatal error
				return fmt.Errorf("fetch credits: %w", err)
			}
			credits, fetched = c, true
		}
		if err := b.handleMention(m, credits); err != nil {
			b.log.Error("failed to handle mention", "mention_id", m.Id, "item_id", m.Item.Id, "err", err)
			break
		}
		b.lastSeen = m.Id
	}

	return b.pollZaps()
}

func (b *Bot) pollZaps() error {
	if len(b.tracking) == 0 {
		return nil
	}

	zaps, err := b.client.Zaps()
	if err != nil {
		return fmt.Errorf("fetch zaps: %w", err)
	}

	for _, z := range zaps {
		t, ok := b.tracking[z.Item.Id]
		if !ok {
			continue
		}
		handled, err := b.handleZap(z, t)
		if err != nil {
			b.log.Error("failed to handle zap", "item_id", z.Item.Id, "err", err)
			continue
		}
		if handled {
			delete(b.tracking, z.Item.Id)
		}
	}
	return nil
}

func (b *Bot) handleZap(n sn.Notification, t tracked) (handled bool, err error) {
	if n.Item.Sats < t.requested {
		return false, nil
	}

	title := fmt.Sprintf("Received %d/%d credits from @%s", n.Item.Sats, t.requested, t.author)
	body := fmt.Sprintf("Open link to pay invoice\npayment_hash=%s", t.paymentHash)
	if err := b.notify.Notify(title, body, t.itemURL, "zap"); err != nil {
		return false, fmt.Errorf("notify: %w", err)
	}
	b.log.Info("notified operator of funded request", "item_id", n.Item.Id, "received", n.Item.Sats, "requested", t.requested)
	return true, nil
}

func (b *Bot) handleMention(n sn.Notification, credits int) error {
	item := n.Item
	if item.DeletedAt.Valid {
		return nil
	}

	reply, note := b.evaluate(item, credits)

	replyID, err := b.client.CreateComment(item.Id, reply)
	if err != nil {
		return fmt.Errorf("post reply: %w", err)
	}
	b.log.Info("replied to mention", "mention_id", n.Id, "item_id", item.Id, "author", item.User.Name, "notify", note != nil)

	if note != nil {
		// start tracking reply
		b.tracking[replyID] = tracked{requested: note.requested, invoice: note.invoice, author: note.author, paymentHash: note.paymentHash, itemURL: note.click}
		if err := b.notify.Notify(note.title, note.body, note.click, note.tags...); err != nil {
			// we don't return an error here because we don't want to post again on retry
			b.log.Warn("ntfy notification failed", "item_id", item.Id, "err", err, "details", note.body)
		} else {
			b.log.Info("notified operator", "item_id", item.Id)
		}
	}
	return nil
}

type notification struct {
	title string
	body  string
	click string
	tags  []string
	// requested is the credits the seller must send
	requested int
	// invoice is the lightning invoice to pay once the reply is funded
	invoice string
	// author requested the exchange
	author string
	// paymentHash is the invoice payment hash, hex
	paymentHash string
}

func helpText(name string) string {
	return "```\n" +
		"Usage: @" + name + " sell <invoice>\n" +
		"  Exchange credits for sats. <invoice> must be a mainnet lightning invoice\n" +
		"  with an amount that is valid for at least 24h.\n" +
		"```"
}

func parseSell(text, mention string) (invoice string, ok bool) {
	fields := strings.Fields(text)
	if len(fields) == 3 && strings.EqualFold(fields[0], mention) && strings.EqualFold(fields[1], "sell") {
		return fields[2], true
	}
	return "", false
}

func (b *Bot) evaluate(item sn.Item, credits int) (reply string, note *notification) {
	author := item.User.Name
	if author == "" {
		author = "anon"
	}
	itemURL := fmt.Sprintf("%s/items/%d", b.baseURL, item.Id)

	invoiceArg, ok := parseSell(item.Text, "@"+b.name)
	if !ok {
		return helpText(b.name), nil
	}

	pr, err := ln.ParseInvoice(invoiceArg)
	if err != nil {
		switch {
		case errors.Is(err, ln.ErrNoAmount):
			return fmt.Sprintf("@%s invoice must have an amount", author), nil
		case errors.Is(err, ln.ErrInvalidInvoice):
			return fmt.Sprintf("@%s failed to decode invoice", author), nil
		case errors.Is(err, ln.ErrWrongNetwork):
			return fmt.Sprintf("@%s invoice must be mainnet", author), nil
		case errors.Is(err, ln.ErrExpired):
			return fmt.Sprintf("@%s invoice has expired", author), nil
		case errors.Is(err, ln.ErrExpiresSoon):
			return fmt.Sprintf("@%s invoice must be valid for at least %dh", author, int(ln.MinInvoiceValidity.Hours())), nil
		default: // ErrNoInvoice
			return fmt.Sprintf("@%s invoice not found", author), nil
		}
	}

	// Rate from the bot's perspective
	recvRate, maxAccepted, accepting := b.pricer.Quote(credits)
	if !accepting {
		return fmt.Sprintf("@%s Sorry, the bank is full and not accepting credits right now. Please try again later.", author), nil
	}

	sats := bank.MsatsToSats(pr.Msats)
	if sats > maxAccepted {
		return fmt.Sprintf(
			"@%s Sorry, I can pay at most %d sats right now, but your invoice is for %d sats.",
			author, maxAccepted, sats,
		), nil
	}

	// Credits the bot must receive
	receive := bank.CreditsForSats(sats, recvRate)
	// Credits the user must send
	// TODO: include routing fee estimate to avoid fee siphoning
	send := bank.CreditsToSend(receive)

	// Rate from the user's perspective (scaled)
	sendRate := send * bank.RateScale / sats

	reply = fmt.Sprintf(
		"@%s Zap me %d credits, then I will pay your %d sats lightning invoice.\n\n"+
			"<sub>exchange rate: %s credits/sat · you send %d credits · bot receives %d credits · payments can take up to 24h</sub>",
		author, send, sats, bank.FormatRate(sendRate), send, receive,
	)

	// TODO: include public key
	note = &notification{
		title: fmt.Sprintf("exchange request from @%s", author),
		body: fmt.Sprintf(
			"@%s wants to receive %d sats. We asked for %d/%d credits (rate: %s/%s)\npayment_hash=%x",
			author, sats, send, receive, bank.FormatRate(sendRate), bank.FormatRate(recvRate), pr.PaymentHash,
		),
		click:       itemURL,
		tags:        []string{"incoming_envelope"},
		requested:   send,
		invoice:     invoiceArg,
		author:      author,
		paymentHash: fmt.Sprintf("%x", pr.PaymentHash),
	}
	return reply, note
}
