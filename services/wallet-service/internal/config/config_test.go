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

	if cfg.GRPCPort != "50053" {
		t.Fatalf("expected default grpc port 50053, got %s", cfg.GRPCPort)
	}
	if cfg.DBConnectMaxRetries != 8 {
		t.Fatalf("expected default retries 8, got %d", cfg.DBConnectMaxRetries)
	}
	if cfg.DBConnectTimeout != 3*time.Second {
		t.Fatalf("expected default timeout 3s, got %s", cfg.DBConnectTimeout)
	}
}

func TestLoadFromLookup_InvalidValues(t *testing.T) {
	env := map[string]string{
		"WALLET_DB_MAX_OPEN_CONNS":          "bad-int",
		"WALLET_DB_CONNECT_TIMEOUT":         "bad-duration",
		"WALLET_DB_CONNECT_MAX_RETRIES":     "-1",
		"WALLET_DB_CONNECT_INITIAL_BACKOFF": "9s",
		"WALLET_DB_CONNECT_MAX_BACKOFF":     "1s",
	}

	_, err := LoadFromLookup(func(key string) string { return env[key] })
	if err == nil {
		t.Fatalf("expected error but got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "WALLET_DB_MAX_OPEN_CONNS must be int") {
		t.Fatalf("expected int parse error, got: %s", msg)
	}
	if !strings.Contains(msg, "WALLET_DB_CONNECT_TIMEOUT must be duration") {
		t.Fatalf("expected duration parse error, got: %s", msg)
	}
	if !strings.Contains(msg, "WALLET_DB_CONNECT_MAX_RETRIES must be >= 0") {
		t.Fatalf("expected retries validation error, got: %s", msg)
	}
}
