package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ekzyis/ccbank/bank"
	"github.com/ekzyis/ccbank/bot"
	"github.com/ekzyis/ccbank/ntfy"
	"github.com/ekzyis/ccbank/sn"
)

const pollInterval = 5 * time.Second

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := loadEnvFile(".env"); err != nil {
		logger.Error("failed to load .env", "err", err)
		os.Exit(1)
	}

	cfg, err := LoadConfig()
	if err != nil {
		logger.Error("invalid config", "err", err)
		os.Exit(1)
	}

	client := sn.NewClient(cfg.Nsec, cfg.BaseURL)
	notify := ntfy.NewNotifier(cfg.NtfyURL, cfg.NtfyTopic, cfg.NtfyToken)

	b, err := bot.NewBot(cfg.BaseURL, client, notify, bank.NewPricer(), sn.Balancer{Client: client}, logger)
	if err != nil {
		logger.Error("startup failed", "err", err)
		os.Exit(1)
	}

	logger.Info("ccbank started",
		"base_url", cfg.BaseURL,
		"ntfy_url", cfg.NtfyURL,
		"poll_interval", pollInterval,
		"rate", bank.FormatRate(bank.DefaultRate),
		"max_sats", bank.MaxSats,
		"treasury_target", bank.TreasuryTarget,
	)

	startBody := fmt.Sprintf(
		"base_url=%s rate=%s max_sats=%d treasury_target=%d",
		cfg.BaseURL, bank.FormatRate(bank.DefaultRate), bank.MaxSats, bank.TreasuryTarget,
	)
	if err := notify.Notify("ccbank started", startBody, "", "sparkles"); err != nil {
		logger.Warn("startup notification failed", "err", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	poll(b, logger)
	for {
		select {
		case <-ticker.C:
			poll(b, logger)
		case <-stop:
			logger.Info("shutting down")
			return
		}
	}
}

func poll(b *bot.Bot, logger *slog.Logger) {
	if err := b.Poll(); err != nil {
		logger.Error("poll failed", "err", err)
	}
}
