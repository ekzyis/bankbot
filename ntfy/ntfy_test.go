package ntfy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNotifier_SendsRequest(t *testing.T) {
	var (
		gotPath, gotTitle, gotClick, gotTags, gotBody, gotAuth string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotTitle = r.Header.Get("Title")
		gotClick = r.Header.Get("Click")
		gotTags = r.Header.Get("Tags")
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewNotifier(srv.URL, "secret-topic", "tk_secret")
	if err := n.Notify("hi", "the body", "https://stacker.news/items/1", "bank", "zap"); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	if gotPath != "/secret-topic" {
		t.Errorf("path = %q, want /secret-topic", gotPath)
	}
	if gotAuth != "Bearer tk_secret" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer tk_secret")
	}
	if gotTitle != "hi" {
		t.Errorf("Title = %q", gotTitle)
	}
	if gotClick != "https://stacker.news/items/1" {
		t.Errorf("Click = %q", gotClick)
	}
	if gotTags != "bank,zap" {
		t.Errorf("Tags = %q", gotTags)
	}
	if gotBody != "the body" {
		t.Errorf("Body = %q", gotBody)
	}
}

func TestNotifier_HasTimeout(t *testing.T) {
	n := NewNotifier("https://ntfy.example", "topic", "")
	if n.http.Timeout <= 5*time.Second {
		t.Errorf("http client timeout = %v, want > 5s", n.http.Timeout)
	}
}

func TestNotifier_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	n := NewNotifier(srv.URL, "topic", "")
	if err := n.Notify("t", "b", ""); err == nil {
		t.Fatal("expected error on non-2xx status")
	}
}
