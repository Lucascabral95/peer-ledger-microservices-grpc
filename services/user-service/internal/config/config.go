package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/peer-ledger/internal/postgresdsn"
)

type LookupFunc func(string) string

type Config struct {
	GRPCPort                string
	UserDBDSN               string
	PasswordHashIterations  int
	PasswordMinLength       int
	DBMaxOpenConns          int
	DBMaxIdleConns          int
	DBConnMaxLifetime       time.Duration
	DBConnMaxIdleTime       time.Duration
	DBConnectTimeout        time.Duration
	DBConnectMaxRetries     int
	DBConnectInitialBackoff time.Duration
	DBConnectMaxBackoff     time.Duration
	GracefulShutdownTimeout time.Duration
}

func Load() (*Config, error) {
	return LoadFromLookup(os.Getenv)
}

func LoadFromLookup(lookup LookupFunc) (*Config, error) {
	cfg := &Config{
		GRPCPort:                getString(lookup, "GRPC_PORT", "50051"),
		UserDBDSN:               resolveUserDBDSN(lookup),
		PasswordHashIterations:  120000,
		PasswordMinLength:       8,
		DBMaxOpenConns:          25,
		DBMaxIdleConns:          10,
		DBConnMaxLifetime:       30 * time.Minute,
		DBConnMaxIdleTime:       5 * time.Minute,
		DBConnectTimeout:        3 * time.Second,
		DBConnectMaxRetries:     8,
		DBConnectInitialBackoff: 500 * time.Millisecond,
		DBConnectMaxBackoff:     8 * time.Second,
		GracefulShutdownTimeout: 10 * time.Second,
	}

	var errs []string

	if v, err := getInt(lookup, "DB_MAX_OPEN_CONNS", cfg.DBMaxOpenConns); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.DBMaxOpenConns = v
	}

	if v, err := getInt(lookup, "USER_PASSWORD_HASH_ITERATIONS", cfg.PasswordHashIterations); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.PasswordHashIterations = v
	}

	if v, err := getInt(lookup, "USER_PASSWORD_MIN_LENGTH", cfg.PasswordMinLength); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.PasswordMinLength = v
	}

	if v, err := getInt(lookup, "DB_MAX_IDLE_CONNS", cfg.DBMaxIdleConns); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.DBMaxIdleConns = v
	}

	if v, err := getDuration(lookup, "DB_CONN_MAX_LIFETIME", cfg.DBConnMaxLifetime); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.DBConnMaxLifetime = v
	}

	if v, err := getDuration(lookup, "DB_CONN_MAX_IDLE_TIME", cfg.DBConnMaxIdleTime); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.DBConnMaxIdleTime = v
	}

	if v, err := getDuration(lookup, "DB_CONNECT_TIMEOUT", cfg.DBConnectTimeout); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.DBConnectTimeout = v
	}

	if v, err := getInt(lookup, "DB_CONNECT_MAX_RETRIES", cfg.DBConnectMaxRetries); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.DBConnectMaxRetries = v
	}

	if v, err := getDuration(lookup, "DB_CONNECT_INITIAL_BACKOFF", cfg.DBConnectInitialBackoff); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.DBConnectInitialBackoff = v
	}

	if v, err := getDuration(lookup, "DB_CONNECT_MAX_BACKOFF", cfg.DBConnectMaxBackoff); err != nil {
		errs = append(errs, err.Error())
	} else {
		cfg.DBConnectMaxBackoff = v
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
		errs = append(errs, "GRPC_PORT cannot be empty")
	}
	if strings.TrimSpace(c.UserDBDSN) == "" {
		errs = append(errs, "USER_DB_DSN cannot be empty")
	}
	if c.PasswordHashIterations <= 0 {
		errs = append(errs, "USER_PASSWORD_HASH_ITERATIONS must be > 0")
	}
	if c.PasswordMinLength < 8 {
		errs = append(errs, "USER_PASSWORD_MIN_LENGTH must be >= 8")
	}
	if c.DBMaxOpenConns <= 0 {
		errs = append(errs, "DB_MAX_OPEN_CONNS must be > 0")
	}
	if c.DBMaxIdleConns < 0 {
		errs = append(errs, "DB_MAX_IDLE_CONNS must be >= 0")
	}
	if c.DBMaxIdleConns > c.DBMaxOpenConns {
		errs = append(errs, "DB_MAX_IDLE_CONNS cannot be greater than DB_MAX_OPEN_CONNS")
	}
	if c.DBConnectTimeout <= 0 {
		errs = append(errs, "DB_CONNECT_TIMEOUT must be > 0")
	}
	if c.DBConnectMaxRetries < 0 {
		errs = append(errs, "DB_CONNECT_MAX_RETRIES must be >= 0")
	}
	if c.DBConnectInitialBackoff <= 0 {
		errs = append(errs, "DB_CONNECT_INITIAL_BACKOFF must be > 0")
	}
	if c.DBConnectMaxBackoff <= 0 {
		errs = append(errs, "DB_CONNECT_MAX_BACKOFF must be > 0")
	}
	if c.DBConnectInitialBackoff > c.DBConnectMaxBackoff {
		errs = append(errs, "DB_CONNECT_INITIAL_BACKOFF cannot be greater than DB_CONNECT_MAX_BACKOFF")
	}
	if c.GracefulShutdownTimeout <= 0 {
		errs = append(errs, "GRACEFUL_SHUTDOWN_TIMEOUT must be > 0")
	}

	if len(errs) > 0 {
		// return fmt.Errorf(strings.Join(errs, "; "))
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func getString(lookup LookupFunc, key, fallback string) string {
	value := lookup(key)
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func resolveUserDBDSN(lookup LookupFunc) string {
	dsn := strings.TrimSpace(lookup("USER_DB_DSN"))
	if dsn != "" {
		return dsn
	}

	return postgresdsn.Build(
		getString(lookup, "USER_DB_HOST", "postgres"),
		getIntOrFallback(lookup, "USER_DB_PORT", 5432),
		getString(lookup, "USER_DB_USER", "admin"),
		getString(lookup, "USER_DB_PASSWORD", "secret"),
		getString(lookup, "USER_DB_NAME", "users_db"),
		getString(lookup, "USER_DB_SSLMODE", "disable"),
	)
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

func getIntOrFallback(lookup LookupFunc, key string, fallback int) int {
	value := strings.TrimSpace(lookup(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
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
