// Package lntest provides helpers for building real BOLT11 invoices in tests.
package lntest

import (
	"testing"
	"time"

	"github.com/ekzyis/lnpilot/lib/secp256k1"
	"github.com/ekzyis/lnpilot/lightning/bolt11"
	"github.com/ekzyis/lnpilot/lightning/lntypes"
)

func Signer(t *testing.T) secp256k1.Signer {
	t.Helper()
	var pk [32]byte
	pk[31] = 0x01
	s, err := secp256k1.NewPrivateKeySigner(pk[:])
	if err != nil {
		t.Fatalf("NewPrivateKeySigner: %v", err)
	}
	return s
}

func MakeInvoiceExpiry(t *testing.T, msats uint64, network lntypes.Network, expiry time.Duration) string {
	t.Helper()
	pr, err := bolt11.NewPaymentRequest(msats, bolt11.WithNetwork(network), bolt11.WithExpiry(expiry))
	if err != nil {
		t.Fatalf("NewPaymentRequest: %v", err)
	}
	inv, err := pr.EncodeBech32(Signer(t))
	if err != nil {
		t.Fatalf("EncodeBech32: %v", err)
	}
	return inv
}

// MakeInvoiceCreatedAt backdates the invoice timestamp so callers can build an
// already-expired invoice (createdAt + expiry in the past).
func MakeInvoiceCreatedAt(t *testing.T, msats uint64, network lntypes.Network, expiry time.Duration, createdAt time.Time) string {
	t.Helper()
	pr, err := bolt11.NewPaymentRequest(msats, bolt11.WithNetwork(network), bolt11.WithExpiry(expiry))
	if err != nil {
		t.Fatalf("NewPaymentRequest: %v", err)
	}
	pr.Timestamp = createdAt
	inv, err := pr.EncodeBech32(Signer(t))
	if err != nil {
		t.Fatalf("EncodeBech32: %v", err)
	}
	return inv
}

func MakeInvoiceNet(t *testing.T, msats uint64, network lntypes.Network) string {
	return MakeInvoiceExpiry(t, msats, network, 48*time.Hour)
}

func MakeInvoice(t *testing.T, msats uint64) string {
	return MakeInvoiceNet(t, msats, lntypes.NetworkMainnet)
}
