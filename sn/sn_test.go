package sn

import (
	"errors"
	"testing"
)

type fakeMeClient struct {
	credits int
	err     error
}

func (f fakeMeClient) Me() (*User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &User{Privates: UserPrivates{Credits: f.credits}}, nil
}

func TestBalancer_Credits(t *testing.T) {
	b := Balancer{Client: fakeMeClient{credits: 42_000}}
	got, err := b.Credits()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 42_000 {
		t.Errorf("credits = %d, want 42000", got)
	}
}

func TestBalancer_CreditsError(t *testing.T) {
	b := Balancer{Client: fakeMeClient{err: errors.New("boom")}}
	if _, err := b.Credits(); err == nil {
		t.Fatal("expected error to propagate")
	}
}
