package ln

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ekzyis/lnpilot/lightning/bolt11"
	"github.com/ekzyis/lnpilot/lightning/lntypes"
)

const MinInvoiceValidity = 1 * time.Hour

var invoiceRe = regexp.MustCompile(`(?i)\bln(?:bcrt|tbs|bc|tb|sb)[a-z0-9]+\b`)

var (
	ErrNoInvoice      = errors.New("no lightning invoice found")
	ErrInvalidInvoice = errors.New("failed to decode invoice")
	ErrNoAmount       = errors.New("invoice must have an amount")
	ErrWrongNetwork   = errors.New("invoice must be mainnet")
	ErrExpired        = errors.New("invoice has expired")
	ErrExpiresSoon    = errors.New("invoice expires too soon")
)

// ParseInvoice extracts and decodes a mainnet BOLT11 invoice from text. The
// invoice must carry an amount so the bot can quote how many credits the sender
// needs to zap to cover it.
func ParseInvoice(text string) (*bolt11.PaymentRequest, error) {
	match := invoiceRe.FindString(text)
	if match == "" {
		return nil, ErrNoInvoice
	}

	inv := strings.ToLower(match)

	pr, err := bolt11.DecodePaymentRequest(inv)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInvoice, err)
	}
	if pr.Msats == 0 {
		return nil, ErrNoAmount
	}
	if pr.Network != lntypes.NetworkMainnet {
		return nil, ErrWrongNetwork
	}
	remaining := time.Until(pr.Timestamp.Add(pr.Expiry))
	if remaining <= 0 {
		return nil, ErrExpired
	}
	if remaining < MinInvoiceValidity {
		return nil, ErrExpiresSoon
	}
	return pr, nil
}
