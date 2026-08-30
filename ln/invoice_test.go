package ln

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ekzyis/ccbank/internal/lntest"
	"github.com/ekzyis/lnpilot/lightning/lntypes"
)

func TestParseInvoice_WithAmount(t *testing.T) {
	inv := lntest.MakeInvoice(t, 2500000) // 2500 sats
	pr, err := ParseInvoice("hey @ccbank " + inv + " please")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Msats != 2500000 {
		t.Errorf("msats = %d, want 2500000", pr.Msats)
	}
}

func TestParseInvoice_Amountless(t *testing.T) {
	inv := lntest.MakeInvoice(t, 0)
	_, err := ParseInvoice("@ccbank " + inv)
	if !errors.Is(err, ErrNoAmount) {
		t.Fatalf("err = %v, want ErrNoAmount", err)
	}
}

func TestParseInvoice_Testnet(t *testing.T) {
	inv := lntest.MakeInvoiceNet(t, 2500000, lntypes.NetworkTestnet)
	_, err := ParseInvoice("@ccbank " + inv)
	if !errors.Is(err, ErrWrongNetwork) {
		t.Errorf("err = %v, want ErrWrongNetwork", err)
	}
}

func TestParseInvoice_Expired(t *testing.T) {
	// Created 2h ago, valid for 1h → expired 1h ago.
	inv := lntest.MakeInvoiceCreatedAt(t, 2500000, lntypes.NetworkMainnet, time.Hour, time.Now().Add(-2*time.Hour))
	if _, err := ParseInvoice("@ccbank " + inv); !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
}

func TestParseInvoice_ExpiresSoon(t *testing.T) {
	inv := lntest.MakeInvoiceExpiry(t, 2500000, lntypes.NetworkMainnet, time.Hour) // 1h < 24h min
	if _, err := ParseInvoice("@ccbank " + inv); !errors.Is(err, ErrExpiresSoon) {
		t.Fatalf("err = %v, want ErrExpiresSoon", err)
	}
}

func TestParseInvoice_NoInvoice(t *testing.T) {
	_, err := ParseInvoice("@ccbank how do I exchange credits?")
	if !errors.Is(err, ErrNoInvoice) {
		t.Fatalf("err = %v, want ErrNoInvoice", err)
	}
}

func TestParseInvoice_Corrupt(t *testing.T) {
	inv := lntest.MakeInvoice(t, 2500000)
	// Corrupt trailing characters to break the checksum/signature.
	bad := inv[:len(inv)-3] + "qqp"
	_, err := ParseInvoice("@ccbank " + bad)
	if !errors.Is(err, ErrInvalidInvoice) {
		t.Fatalf("err = %v, want ErrInvalidInvoice", err)
	}
}

func TestParseInvoice_Uppercase(t *testing.T) {
	// BOLT11 invoices may be all-uppercase (e.g. rendered for QR codes).
	up := strings.ToUpper(lntest.MakeInvoice(t, 2500000))
	if _, err := ParseInvoice("@ccbank " + up); err != nil {
		t.Fatalf("unexpected error for uppercase invoice: %v", err)
	}
}
