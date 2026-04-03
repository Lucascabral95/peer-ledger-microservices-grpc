package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromLookup_Defaults(t *testing.T) {
	cfg, err := LoadFromLookup(func(string) string { return "" })
	if err != nil {
		t.Fatalf("LoadFromLookup() error: %v", err)
	}

	if cfg.GRPCPort != "50054" {
		t.Fatalf("expected grpc port 50054, got %q", cfg.GRPCPort)
	}
	if cfg.TransactionDBDSN == "" {
		t.Fatalf("expected default dsn")
	}
	if cfg.DBConnectTimeout != 3*time.Second {
		t.Fatalf("expected default connect timeout 3s, got %s", cfg.DBConnectTimeout)
	}
}

func TestLoadFromLookup_InvalidValues(t *testing.T) {
	lookup := func(key string) string {
		switch key {
		case "TRANSACTION_DB_MAX_OPEN_CONNS":
			return "abc"
		case "TRANSACTION_DB_CONNECT_TIMEOUT":
			return "invalid"
		default:
			return ""
		}
	}

	_, err := LoadFromLookup(lookup)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "TRANSACTION_DB_MAX_OPEN_CONNS must be int") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "TRANSACTION_DB_CONNECT_TIMEOUT must be duration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := &Config{
		GRPCPort:                "",
		TransactionDBDSN:        "",
		DBMaxOpenConns:          0,
		DBMaxIdleConns:          2,
		DBConnMaxLifetime:       0,
		DBConnMaxIdleTime:       0,
		DBConnectTimeout:        0,
		DBConnectMaxRetries:     -1,
		DBConnectInitialBackoff: 5 * time.Second,
		DBConnectMaxBackoff:     1 * time.Second,
		GracefulShutdownTimeout: 0,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "TRANSACTION_GRPC_PORT cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "TRANSACTION_DB_DSN cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}
