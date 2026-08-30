package main

import "testing"

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("SN_NSEC", "nsec1abc")
	t.Setenv("NTFY_TOPIC", "secret-topic")
	t.Setenv("SN_BASE_URL", "") // empty is treated as unset -> default
	t.Setenv("NTFY_URL", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://stacker.news" {
		t.Errorf("BaseURL = %q, want default", cfg.BaseURL)
	}
	if cfg.NtfyURL != "https://ntfy.ekzy.is" {
		t.Errorf("NtfyURL = %q, want default", cfg.NtfyURL)
	}
	if cfg.NtfyTopic != "secret-topic" {
		t.Errorf("NtfyTopic = %q", cfg.NtfyTopic)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	t.Setenv("SN_NSEC", "nsec1abc")
	t.Setenv("NTFY_TOPIC", "secret-topic")
	t.Setenv("SN_BASE_URL", "https://sn.example")
	t.Setenv("NTFY_URL", "https://ntfy.example")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://sn.example" || cfg.NtfyURL != "https://ntfy.example" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoadConfig_TrimsBaseURLTrailingSlash(t *testing.T) {
	t.Setenv("SN_NSEC", "nsec1abc")
	t.Setenv("NTFY_TOPIC", "secret-topic")
	t.Setenv("SN_BASE_URL", "https://sn.example///")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://sn.example" {
		t.Errorf("BaseURL = %q, want trailing slashes trimmed", cfg.BaseURL)
	}
}

func TestLoadConfig_RequiresNsec(t *testing.T) {
	t.Setenv("SN_NSEC", "")
	t.Setenv("NTFY_TOPIC", "secret-topic")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error when SN_NSEC is missing")
	}
}

func TestLoadConfig_RequiresNtfyTopic(t *testing.T) {
	t.Setenv("SN_NSEC", "nsec1abc")
	t.Setenv("NTFY_TOPIC", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error when NTFY_TOPIC is missing")
	}
}
