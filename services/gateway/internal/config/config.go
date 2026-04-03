package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type LookupFunc func(string) string

type Config struct {
	HTTPPort                   string
	UserServiceGRPCAddr        string
	FraudServiceGRPCAddr       string
	WalletServiceGRPCAddr      string
	TransactionServiceAddr     string
	JWTSecret                  string
	JWTIssuer                  string
	JWTTTL                     time.Duration
	GRPCDialTimeout            time.Duration
	GRPCMaxAttempts            int
	RateLimitEnabled           bool
	RateLimitDefaultRequests   int
	RateLimitDefaultWindow     time.Duration
	RateLimitTransfersRequests int
	RateLimitTransfersWindow   time.Duration
	RateLimitCleanup           time.Duration
	RateLimitTrustProxy        bool
	RateLimitExemptPaths       []string
	GracefulShutdownTimeout    time.Duration
}

func Load() (*Config, error) {
	return LoadFromLookup(os.Getenv)
}

func LoadFromLookup(lookup LookupFunc) (*Config, error) {
	cfg := &Config{
		HTTPPort:                   getString(lookup, "PORT", "8080"),
		UserServiceGRPCAddr:        getString(lookup, "USER_SERVICE_GRPC_ADDR", "user-service:50051"),
		FraudServiceGRPCAddr:       getString(lookup, "FRAUD_SERVICE_GRPC_ADDR", "fraud-service:50052"),
		WalletServiceGRPCAddr:      getString(lookup, "WALLET_SERVICE_GRPC_ADDR", "wallet-service:50053"),
		TransactionServiceAddr:     getString(lookup, "TRANSACTION_SERVICE_GRPC_ADDR", "transaction-service:50054"),
		JWTSecret:                  getString(lookup, "AUTH_JWT_SECRET", "change-this-secret-to-at-least-32-characters"),
		JWTIssuer:                  getString(lookup, "AUTH_JWT_ISSUER", "peer-ledger-gateway"),
		JWTTTL:                     24 * time.Hour,
		GRPCDialTimeout:            3 * time.Second,
		GRPCMaxAttempts:            10,
		RateLimitEnabled:           true,
		RateLimitDefaultRequests:   120,  // (Burst inicial) Algoritmo Token Bucket
		RateLimitDefaultWindow:     time.Minute,
		RateLimitTransfersRequests: 20,
		RateLimitTransfersWindow:   time.Minute,
		RateLimitCleanup:           2 * time.Minute, 
		RateLimitTrustProxy:        false,
		RateLimitExemptPaths:       []string{"/health", "/ping"},
		GracefulShutdownTimeout:    10 * time.Second,
	}

	var errs []string

	if v, err := getDuration(lookup, "GATEWAY_GRPC_DIAL_TIMEOUT", cfg.GRPCDialTimeout); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.GRPCDialTimeout = v
	}

	if v, err := getDuration(lookup, "AUTH_JWT_TTL", cfg.JWTTTL); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.JWTTTL = v
	}

	if v, err := getInt(lookup, "GATEWAY_GRPC_MAX_ATTEMPTS", cfg.GRPCMaxAttempts); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.GRPCMaxAttempts = v
	}

	if v, err := getBool(lookup, "GATEWAY_RATE_LIMIT_ENABLED", cfg.RateLimitEnabled); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.RateLimitEnabled = v
	}

	if v, err := getInt(lookup, "GATEWAY_RATE_LIMIT_DEFAULT_REQUESTS", cfg.RateLimitDefaultRequests); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.RateLimitDefaultRequests = v
	}

	if v, err := getDuration(lookup, "GATEWAY_RATE_LIMIT_DEFAULT_WINDOW", cfg.RateLimitDefaultWindow); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.RateLimitDefaultWindow = v
	}

	if v, err := getInt(lookup, "GATEWAY_RATE_LIMIT_TRANSFERS_REQUESTS", cfg.RateLimitTransfersRequests); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.RateLimitTransfersRequests = v
	}

	if v, err := getDuration(lookup, "GATEWAY_RATE_LIMIT_TRANSFERS_WINDOW", cfg.RateLimitTransfersWindow); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.RateLimitTransfersWindow = v
	}

	if v, err := getDuration(lookup, "GATEWAY_RATE_LIMIT_CLEANUP_INTERVAL", cfg.RateLimitCleanup); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.RateLimitCleanup = v
	}

	if v, err := getBool(lookup, "GATEWAY_RATE_LIMIT_TRUST_PROXY", cfg.RateLimitTrustProxy); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.RateLimitTrustProxy = v
	}

	cfg.RateLimitExemptPaths = getCSV(lookup, "GATEWAY_RATE_LIMIT_EXEMPT_PATHS", cfg.RateLimitExemptPaths)

	if v, err := getDuration(lookup, "GATEWAY_GRACEFUL_SHUTDOWN_TIMEOUT", cfg.GracefulShutdownTimeout); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.GracefulShutdownTimeout = v
	}

	if err := cfg.Validate(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid gateway config: %s", strings.Join(errs, "; "))
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	var errs []string

	if strings.TrimSpace(c.HTTPPort) == "" {
		errs = append(errs, "PORT cannot be empty")
	}
	if strings.TrimSpace(c.UserServiceGRPCAddr) == "" {
		errs = append(errs, "USER_SERVICE_GRPC_ADDR cannot be empty")
	}
	if strings.TrimSpace(c.FraudServiceGRPCAddr) == "" {
		errs = append(errs, "FRAUD_SERVICE_GRPC_ADDR cannot be empty")
	}
	if strings.TrimSpace(c.WalletServiceGRPCAddr) == "" {
		errs = append(errs, "WALLET_SERVICE_GRPC_ADDR cannot be empty")
	}
	if strings.TrimSpace(c.TransactionServiceAddr) == "" {
		errs = append(errs, "TRANSACTION_SERVICE_GRPC_ADDR cannot be empty")
	}
	if len(strings.TrimSpace(c.JWTSecret)) < 32 {
		errs = append(errs, "AUTH_JWT_SECRET must be at least 32 characters")
	}
	if strings.TrimSpace(c.JWTIssuer) == "" {
		errs = append(errs, "AUTH_JWT_ISSUER cannot be empty")
	}
	if c.JWTTTL <= 0 {
		errs = append(errs, "AUTH_JWT_TTL must be > 0")
	}
	if c.GRPCDialTimeout <= 0 {
		errs = append(errs, "GATEWAY_GRPC_DIAL_TIMEOUT must be > 0")
	}
	if c.GRPCMaxAttempts <= 0 {
		errs = append(errs, "GATEWAY_GRPC_MAX_ATTEMPTS must be > 0")
	}
	if c.RateLimitEnabled {
		if c.RateLimitDefaultRequests <= 0 {
			errs = append(errs, "GATEWAY_RATE_LIMIT_DEFAULT_REQUESTS must be > 0")
		}
		if c.RateLimitDefaultWindow <= 0 {
			errs = append(errs, "GATEWAY_RATE_LIMIT_DEFAULT_WINDOW must be > 0")
		}
		if c.RateLimitTransfersRequests <= 0 {
			errs = append(errs, "GATEWAY_RATE_LIMIT_TRANSFERS_REQUESTS must be > 0")
		}
		if c.RateLimitTransfersWindow <= 0 {
			errs = append(errs, "GATEWAY_RATE_LIMIT_TRANSFERS_WINDOW must be > 0")
		}
		if c.RateLimitCleanup <= 0 {
			errs = append(errs, "GATEWAY_RATE_LIMIT_CLEANUP_INTERVAL must be > 0")
		}
	}
	if c.GracefulShutdownTimeout <= 0 {
		errs = append(errs, "GATEWAY_GRACEFUL_SHUTDOWN_TIMEOUT must be > 0")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func getString(lookup LookupFunc, key, fallback string) string {
	value := strings.TrimSpace(lookup(key))
	if value == "" {
		return fallback
	}
	return value
}

func getCSV(lookup LookupFunc, key string, fallback []string) []string {
	value := strings.TrimSpace(lookup(key))
	if value == "" {
		out := make([]string, 0, len(fallback))
		for _, item := range fallback {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getInt(lookup LookupFunc, key string, fallback int) (int, error) {
	value := strings.TrimSpace(lookup(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be int: %w", key, err)
	}
	return parsed, nil
}

func getBool(lookup LookupFunc, key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(lookup(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be bool: %w", key, err)
	}
	return parsed, nil
}

func getDuration(lookup LookupFunc, key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(lookup(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be duration: %w", key, err)
	}
	return parsed, nil
}
