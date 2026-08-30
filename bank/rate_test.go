package bank

import (
	"testing"

	"github.com/ekzyis/lnpilot/lightning/lntypes"
)

func TestMsatsToSats(t *testing.T) {
	cases := []struct {
		msats lntypes.MilliSatoshi
		want  int
	}{
		{0, 0},
		{1000, 1},       // exactly 1 sat
		{2500000, 2500}, // whole sats, no rounding
		{1, 1},          // sub-sat rounds up so the bot isn't shorted
		{1500, 2},       // 1.5 sats rounds up
	}
	for _, c := range cases {
		if got := MsatsToSats(c.msats); got != c.want {
			t.Errorf("MsatsToSats(%d) = %d, want %d", c.msats, got, c.want)
		}
	}
}

func TestCappedPricer_Quote(t *testing.T) {
	p := NewPricer()

	// Far from the target: full per-exchange max.
	rate, max, ok := p.Quote(0)
	if !ok || rate != DefaultRate || max != MaxSats {
		t.Fatalf("Quote(0) = (rate %d, max %d, ok %v), want (%d, %d, true)", rate, max, ok, DefaultRate, MaxSats)
	}

	// Near the target: max shrinks to remaining credits / rate.
	reduced := MaxSats / 2
	credits := TreasuryTarget - reduced*DefaultRate/RateScale
	if _, max, ok = p.Quote(credits); !ok || max != reduced {
		t.Fatalf("Quote(%d) = (max %d, ok %v), want (%d, true)", credits, max, ok, reduced)
	}

	// At the target: not accepting.
	if _, _, ok = p.Quote(TreasuryTarget); ok {
		t.Errorf("Quote(target) should not be accepting")
	}

	// Remaining below one sat's worth of credits: not accepting.
	if _, _, ok = p.Quote(TreasuryTarget - 1); ok {
		t.Errorf("Quote(target-1) should not be accepting when remaining < rate")
	}
}

func TestCreditsToSend(t *testing.T) {
	// The bot receives 70% of a zap, so the sender must gross up and we round up
	// so the bot is never shorted.
	cases := []struct {
		received, want int
	}{
		{0, 0},
		{7, 10},      // exact: 7 / 0.7 = 10
		{5000, 7143}, // ceil(5000 * 10 / 7) = ceil(7142.86)
		{1, 2},       // ceil(1 * 10 / 7) = ceil(1.43)
	}
	for _, c := range cases {
		if got := CreditsToSend(c.received); got != c.want {
			t.Errorf("CreditsToSend(%d) = %d, want %d", c.received, got, c.want)
		}
	}
}

func TestCappedPricer_RoundsMaxDown(t *testing.T) {
	// 10000 remaining credits at 3 credits/sat is 3333.33 sats; it must floor to
	// 3333 so the exchange can't overshoot the treasury target.
	p := CappedPricer{rate: 3 * RateScale, target: 10_000, maxSats: 1_000_000}

	_, maxAccepted, ok := p.Quote(0)
	if !ok {
		t.Fatal("expected accepting")
	}
	if maxAccepted != 3333 {
		t.Errorf("max = %d, want 3333 (floor of 10000/3)", maxAccepted)
	}
	if CreditsForSats(maxAccepted, p.rate) > p.target {
		t.Errorf("max %d at rate %s overshoots target %d", maxAccepted, FormatRate(p.rate), p.target)
	}
}

func TestFormatRate(t *testing.T) {
	cases := []struct {
		rate int
		want string
	}{
		{2000, "2.000"},
		{2125, "2.125"},
		{500, "0.500"},
		{10, "0.010"},
	}
	for _, c := range cases {
		if got := FormatRate(c.rate); got != c.want {
			t.Errorf("FormatRate(%d) = %q, want %q", c.rate, got, c.want)
		}
	}
}

func TestCreditsForSats(t *testing.T) {
	cases := []struct {
		sats, rate, want int
	}{
		{2500, 2000, 5000}, // 2.000 credits/sat, exact
		{2500, 2125, 5313}, // 2.125 credits/sat: 5312.5 rounds up
		{1, 2125, 3},       // 2.125 credits for one sat rounds up
	}
	for _, c := range cases {
		if got := CreditsForSats(c.sats, c.rate); got != c.want {
			t.Errorf("CreditsForSats(%d, %d) = %d, want %d", c.sats, c.rate, got, c.want)
		}
	}
}
