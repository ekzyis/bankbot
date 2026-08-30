package bank

import (
	"fmt"
	"math"

	"github.com/ekzyis/lnpilot/lightning/lntypes"
)

// RateScale expresses credit-per-sat rates as integers with 3 decimal places
// (2.125 credits/sat -> 2125) so pricing math stays exact.
const RateScale = 1000

const (
	DefaultRate    = 2 * RateScale // 2.000 credits per sat
	MaxSats        = 10_000        // cap on sats per exchange
	TreasuryTarget = 100_000       // stop accepting credits once the treasury reaches this
)

// FormatRate renders a scaled rate as a decimal string, e.g. 2125 -> "2.125".
func FormatRate(rate int) string {
	return fmt.Sprintf("%d.%03d", rate/RateScale, rate%RateScale)
}

// CreditsForSats returns how many credits the bot must receive to cover a payout
// of sats at the given scaled rate, rounding up so the bot is never shorted.
func CreditsForSats(sats, rate int) int {
	return (sats*rate + RateScale - 1) / RateScale
}

// CreditsToSend includes the SN sybil fee in the received amount so a sender knows how much to zap.
func CreditsToSend(received int) int {
	return int(math.Ceil(float64(received) * 10 / 7))
}

// MsatsToSats converts a lightning amount from millisats to whole sats (rounding up)
// TODO: return typed int instead of plain int
func MsatsToSats(msats lntypes.MilliSatoshi) int {
	const msatsPerSat = 1000
	return int(math.Ceil(float64(msats) / msatsPerSat))
}

type Pricer interface {
	// Quote returns the scaled credits-per-sat rate, the max sats accepted, and
	// whether the bank still takes credits.
	Quote(credits int) (rate, maxAccepted int, accepting bool)
}

type CappedPricer struct {
	// Credits per sat, scaled by RateScale.
	rate int
	// Treasury target in credits.
	target int
	// Max sats accepted in one exchange.
	maxSats int
}

func (p CappedPricer) Quote(credits int) (rate, maxAccepted int, accepting bool) {
	remaining := p.target - credits
	if remaining <= 0 {
		return 0, 0, false
	}
	// floor so one exchange can't overshoot the treasury target
	maxAccepted = min(p.maxSats, remaining*RateScale/p.rate)
	if maxAccepted <= 0 {
		return 0, 0, false
	}
	return p.rate, maxAccepted, true
}

func NewPricer() Pricer {
	return CappedPricer{rate: DefaultRate, target: TreasuryTarget, maxSats: MaxSats}
}
