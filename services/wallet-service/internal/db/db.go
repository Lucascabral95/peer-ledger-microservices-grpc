package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/peer-ledger/services/wallet-service/internal/config"
)

type OpenFunc func(driverName, dataSourceName string) (*sql.DB, error)
type WaitFunc func(ctx context.Context, d time.Duration) error

type Connector struct {
	open OpenFunc
	wait WaitFunc
}

func NewConnector() *Connector {
	return &Connector{
		open: sql.Open,
		wait: waitFor,
	}
}

func NewConnectorWithDeps(open OpenFunc, wait WaitFunc) *Connector {
	if open == nil {
		open = sql.Open
	}
	if wait == nil {
		wait = waitFor
	}

	return &Connector{
		open: open,
		wait: wait,
	}
}

func ConnectWithRetry(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	return NewConnector().ConnectWithRetry(ctx, cfg)
}

func (c *Connector) ConnectWithRetry(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	var lastErr error

	for attempt := 0; attempt <= cfg.DBConnectMaxRetries; attempt++ {
		dbConn, err := c.open("postgres", cfg.WalletDBDSN)
		if err != nil {
			lastErr = fmt.Errorf("open db: %w", err)
		} else {
			applyPoolConfig(dbConn, cfg)

			pingCtx, cancel := context.WithTimeout(ctx, cfg.DBConnectTimeout)
			err = dbConn.PingContext(pingCtx)
			cancel()

			if err == nil {
				return dbConn, nil
			}

			lastErr = fmt.Errorf("ping db: %w", err)
			_ = dbConn.Close()
		}

		if attempt == cfg.DBConnectMaxRetries {
			break
		}

		backoff := BackoffDuration(attempt, cfg.DBConnectInitialBackoff, cfg.DBConnectMaxBackoff)
		if err := c.wait(ctx, backoff); err != nil {
			return nil, fmt.Errorf("db connect canceled: %w", err)
		}
	}

	return nil, fmt.Errorf("db connect failed after %d retries: %w", cfg.DBConnectMaxRetries, lastErr)
}

func applyPoolConfig(dbConn *sql.DB, cfg *config.Config) {
	dbConn.SetMaxOpenConns(cfg.DBMaxOpenConns)
	dbConn.SetMaxIdleConns(cfg.DBMaxIdleConns)
	dbConn.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	dbConn.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)
}

func BackoffDuration(attempt int, initial, max time.Duration) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	delay := initial
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= max {
			return max
		}
	}
	if delay > max {
		return max
	}
	return delay
}

func waitFor(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
