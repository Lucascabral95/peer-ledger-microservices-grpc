package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host             string
	Port             int
	Username         string
	Password         string
	SSLMode          string
	ConnectTimeout   time.Duration
	StatementTimeout time.Duration
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Host:             getString("DB_MIGRATOR_HOST", ""),
		Port:             getInt("DB_MIGRATOR_PORT", 5432),
		Username:         getString("DB_MIGRATOR_USERNAME", ""),
		Password:         getString("DB_MIGRATOR_PASSWORD", ""),
		SSLMode:          getString("DB_MIGRATOR_SSLMODE", "require"),
		ConnectTimeout:   getDuration("DB_MIGRATOR_CONNECT_TIMEOUT", 10*time.Second),
		StatementTimeout: getDuration("DB_MIGRATOR_STATEMENT_TIMEOUT", 2*time.Minute),
	}

	var errs []string
	if strings.TrimSpace(cfg.Host) == "" {
		errs = append(errs, "DB_MIGRATOR_HOST cannot be empty")
	}
	if cfg.Port <= 0 {
		errs = append(errs, "DB_MIGRATOR_PORT must be > 0")
	}
	if strings.TrimSpace(cfg.Username) == "" {
		errs = append(errs, "DB_MIGRATOR_USERNAME cannot be empty")
	}
	if strings.TrimSpace(cfg.Password) == "" {
		errs = append(errs, "DB_MIGRATOR_PASSWORD cannot be empty")
	}
	if strings.TrimSpace(cfg.SSLMode) == "" {
		errs = append(errs, "DB_MIGRATOR_SSLMODE cannot be empty")
	}
	if cfg.ConnectTimeout <= 0 {
		errs = append(errs, "DB_MIGRATOR_CONNECT_TIMEOUT must be > 0")
	}
	if cfg.StatementTimeout <= 0 {
		errs = append(errs, "DB_MIGRATOR_STATEMENT_TIMEOUT must be > 0")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid db-migrator config: %s", strings.Join(errs, "; "))
	}

	return cfg, nil
}

func (c *Config) DSN(database string) string {
	connectTimeoutSeconds := int(c.ConnectTimeout / time.Second)
	if connectTimeoutSeconds <= 0 {
		connectTimeoutSeconds = 10
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.Username, c.Password),
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   database,
	}

	query := url.Values{}
	query.Set("sslmode", c.SSLMode)
	query.Set("connect_timeout", strconv.Itoa(connectTimeoutSeconds))
	u.RawQuery = query.Encode()

	return u.String()
}

func getString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
