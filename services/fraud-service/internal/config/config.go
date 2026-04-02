package config

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

type LookupFunc func(string) string

type Config struct {
	GRPCPort                string
	PerTxLimitCents         int64
	DailyLimitCents         int64
	VelocityMaxCount        int
	VelocityWindow          time.Duration
	PairCooldown            time.Duration
	IdempotencyTTL          time.Duration
	Timezone                *time.Location
	CleanupInterval         time.Duration
	GracefulShutdownTimeout time.Duration
}

func Load() (*Config, error) {
	return LoadFromLookup(os.Getenv)
}

func LoadFromLookup(lookup LookupFunc) (*Config, error) {
	var errs []string

	cfg := &Config{
		GRPCPort:                getString(lookup, "FRAUD_GRPC_PORT", "50052"),
		VelocityMaxCount:        5,
		VelocityWindow:          10 * time.Minute,
		PairCooldown:            30 * time.Second,
		IdempotencyTTL:          24 * time.Hour,
		CleanupInterval:         time.Minute,
		GracefulShutdownTimeout: 10 * time.Second,
	}

	perTxCents, err := getMoneyCents(lookup, "FRAUD_PER_TX_LIMIT", "20000")
	if err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.PerTxLimitCents = perTxCents
	}

	dailyCents, err := getMoneyCents(lookup, "FRAUD_DAILY_LIMIT", "50000")
	if err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.DailyLimitCents = dailyCents
	}

	if v, err := getInt(lookup, "FRAUD_VELOCITY_MAX_COUNT", cfg.VelocityMaxCount); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.VelocityMaxCount = v
	}

	if v, err := getDuration(lookup, "FRAUD_VELOCITY_WINDOW", cfg.VelocityWindow); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.VelocityWindow = v
	}

	if v, err := getDuration(lookup, "FRAUD_PAIR_COOLDOWN", cfg.PairCooldown); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.PairCooldown = v
	}

	if v, err := getDuration(lookup, "FRAUD_IDEMPOTENCY_TTL", cfg.IdempotencyTTL); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.IdempotencyTTL = v
	}

	tzName := getString(lookup, "FRAUD_TIMEZONE", "America/Argentina/Buenos_Aires")
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		errs = append(errs, fmt.Sprintf("FRAUD_TIMEZONE invalid: %v", err))
	} else {
		cfg.Timezone = loc
	}

	if v, err := getDuration(lookup, "FRAUD_CLEANUP_INTERVAL", cfg.CleanupInterval); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.CleanupInterval = v
	}

	if v, err := getDuration(lookup, "GRACEFUL_SHUTDOWN_TIMEOUT", cfg.GracefulShutdownTimeout); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.GracefulShutdownTimeout = v
	}

	if err := cfg.Validate(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid config: %s", strings.Join(errs, "; "))
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	var errs []string

	if strings.TrimSpace(c.GRPCPort) == "" {
		errs = append(errs, "FRAUD_GRPC_PORT cannot be empty")
	}
	if c.PerTxLimitCents <= 0 {
		errs = append(errs, "FRAUD_PER_TX_LIMIT must be > 0")
	}
	if c.DailyLimitCents <= 0 {
		errs = append(errs, "FRAUD_DAILY_LIMIT must be > 0")
	}
	if c.DailyLimitCents < c.PerTxLimitCents {
		errs = append(errs, "FRAUD_DAILY_LIMIT must be >= FRAUD_PER_TX_LIMIT")
	}
	if c.VelocityMaxCount <= 0 {
		errs = append(errs, "FRAUD_VELOCITY_MAX_COUNT must be > 0")
	}
	if c.VelocityWindow <= 0 {
		errs = append(errs, "FRAUD_VELOCITY_WINDOW must be > 0")
	}
	if c.PairCooldown <= 0 {
		errs = append(errs, "FRAUD_PAIR_COOLDOWN must be > 0")
	}
	if c.IdempotencyTTL <= 0 {
		errs = append(errs, "FRAUD_IDEMPOTENCY_TTL must be > 0")
	}
	if c.Timezone == nil {
		errs = append(errs, "FRAUD_TIMEZONE must be valid")
	}
	if c.CleanupInterval <= 0 {
		errs = append(errs, "FRAUD_CLEANUP_INTERVAL must be > 0")
	}
	if c.GracefulShutdownTimeout <= 0 {
		errs = append(errs, "GRACEFUL_SHUTDOWN_TIMEOUT must be > 0")
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

func getMoneyCents(lookup LookupFunc, key, fallback string) (int64, error) {
	value := strings.TrimSpace(lookup(key))
	if value == "" {
		value = fallback
	}

	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be numeric: %w", key, err)
	}
	if amount <= 0 {
		return 0, fmt.Errorf("%s must be > 0", key)
	}

	cents := int64(math.Round(amount * 100))
	if cents <= 0 {
		return 0, fmt.Errorf("%s invalid after cents conversion", key)
	}

	return cents, nil
}
