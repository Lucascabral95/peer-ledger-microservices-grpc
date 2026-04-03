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

	if cfg.HTTPPort != "8080" {
		t.Fatalf("expected port 8080, got %q", cfg.HTTPPort)
	}
	if !cfg.RateLimitEnabled {
		t.Fatalf("expected rate limit enabled by default")
	}
	if cfg.JWTIssuer != "peer-ledger-gateway" {
		t.Fatalf("expected peer-ledger-gateway, got %q", cfg.JWTIssuer)
	}
	if cfg.JWTTTL != 24*time.Hour {
		t.Fatalf("expected 24h JWT TTL, got %s", cfg.JWTTTL)
	}
	if cfg.RateLimitDefaultRequests != 120 {
		t.Fatalf("expected 120 requests, got %d", cfg.RateLimitDefaultRequests)
	}
	if cfg.RateLimitDefaultWindow != time.Minute {
		t.Fatalf("expected 1m window, got %s", cfg.RateLimitDefaultWindow)
	}
	if cfg.RateLimitTransfersRequests != 20 {
		t.Fatalf("expected 20 transfer requests, got %d", cfg.RateLimitTransfersRequests)
	}
}

func TestLoadFromLookup_InvalidValues(t *testing.T) {
	lookup := func(key string) string {
		switch key {
		case "GATEWAY_GRPC_MAX_ATTEMPTS":
			return "abc"
		case "GATEWAY_RATE_LIMIT_DEFAULT_WINDOW":
			return "bad"
		case "AUTH_JWT_TTL":
			return "bad"
		default:
			return ""
		}
	}

	_, err := LoadFromLookup(lookup)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "GATEWAY_GRPC_MAX_ATTEMPTS must be int") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "GATEWAY_RATE_LIMIT_DEFAULT_WINDOW must be duration") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "AUTH_JWT_TTL must be duration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := &Config{
		HTTPPort:                   "",
		UserServiceGRPCAddr:        "",
		FraudServiceGRPCAddr:       "",
		WalletServiceGRPCAddr:      "",
		TransactionServiceAddr:     "",
		JWTSecret:                  "short",
		JWTIssuer:                  "",
		JWTTTL:                     0,
		GRPCDialTimeout:            0,
		GRPCMaxAttempts:            0,
		RateLimitEnabled:           true,
		RateLimitDefaultRequests:   0,
		RateLimitDefaultWindow:     0,
		RateLimitTransfersRequests: 0,
		RateLimitTransfersWindow:   0,
		RateLimitCleanup:           0,
		GracefulShutdownTimeout:    0,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "PORT cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "AUTH_JWT_SECRET must be at least 32 characters") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "GATEWAY_RATE_LIMIT_DEFAULT_REQUESTS must be > 0") {
		t.Fatalf("unexpected error: %v", err)
	}
}
