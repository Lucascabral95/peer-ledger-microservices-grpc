package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromLookup_Defaults(t *testing.T) {
	cfg, err := LoadFromLookup(func(string) string { return "" })
	if err != nil {
		t.Fatalf("LoadFromLookup() unexpected error: %v", err)
	}

	if cfg.GRPCPort != "50052" {
		t.Fatalf("expected default port 50052, got %s", cfg.GRPCPort)
	}
	if cfg.PerTxLimitCents != 2000000 {
		t.Fatalf("expected per-tx 2000000 cents, got %d", cfg.PerTxLimitCents)
	}
	if cfg.DailyLimitCents != 5000000 {
		t.Fatalf("expected daily 5000000 cents, got %d", cfg.DailyLimitCents)
	}
	if cfg.VelocityWindow != 10*time.Minute {
		t.Fatalf("expected velocity window 10m, got %s", cfg.VelocityWindow)
	}
}

func TestLoadFromLookup_InvalidValues(t *testing.T) {
	env := map[string]string{
		"FRAUD_PER_TX_LIMIT":       "bad-number",
		"FRAUD_DAILY_LIMIT":        "-1",
		"FRAUD_VELOCITY_MAX_COUNT": "bad-int",
		"FRAUD_VELOCITY_WINDOW":    "bad-duration",
		"FRAUD_TIMEZONE":           "Mars/Phobos",
	}

	_, err := LoadFromLookup(func(key string) string { return env[key] })
	if err == nil {
		t.Fatalf("expected error but got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "FRAUD_PER_TX_LIMIT must be numeric") {
		t.Fatalf("expected per-tx parse error, got: %s", msg)
	}
	if !strings.Contains(msg, "FRAUD_DAILY_LIMIT must be > 0") {
		t.Fatalf("expected daily validation error, got: %s", msg)
	}
	if !strings.Contains(msg, "FRAUD_VELOCITY_MAX_COUNT must be int") {
		t.Fatalf("expected velocity int error, got: %s", msg)
	}
	if !strings.Contains(msg, "FRAUD_VELOCITY_WINDOW must be duration") {
		t.Fatalf("expected duration error, got: %s", msg)
	}
	if !strings.Contains(msg, "FRAUD_TIMEZONE invalid") {
		t.Fatalf("expected timezone error, got: %s", msg)
	}
}
