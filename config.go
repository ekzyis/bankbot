package main

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Nsec      string // SN_NSEC (required) — Nostr private key used to log in
	BaseURL   string // SN_BASE_URL (default https://stacker.news)
	NtfyURL   string // NTFY_URL (default https://ntfy.ekzy.is)
	NtfyTopic string // NTFY_TOPIC (required, secret — acts as the access token)
	NtfyToken string // NTFY_TOKEN (optional bearer token for authenticated ntfy access)
}

func LoadConfig() (*Config, error) {
	c := &Config{
		Nsec:      os.Getenv("SN_NSEC"),
		BaseURL:   strings.TrimRight(envDefault("SN_BASE_URL", "https://stacker.news"), "/"),
		NtfyURL:   envDefault("NTFY_URL", "https://ntfy.ekzy.is"),
		NtfyTopic: os.Getenv("NTFY_TOPIC"),
		NtfyToken: os.Getenv("NTFY_TOKEN"),
	}

	if c.Nsec == "" {
		return nil, fmt.Errorf("SN_NSEC is required")
	}
	if c.NtfyTopic == "" {
		return nil, fmt.Errorf("NTFY_TOPIC is required")
	}

	return c, nil
}

func envDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
