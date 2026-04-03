package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/peer-ledger/services/transaction-service/internal/config"
)

func TestConnectWithRetry_NilConfig(t *testing.T) {
	connector := NewConnectorWithDeps(nil, nil)
	_, err := connector.ConnectWithRetry(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestConnectWithRetry_OpenFailsThenSucceeds(t *testing.T) {
	attempts := 0
	connector := NewConnectorWithDeps(
		func(driverName, dsn string) (*sql.DB, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("boom")
			}
			return &sql.DB{}, nil
		},
		func(context.Context, time.Duration) error { return nil },
	)

	cfg := &config.Config{
		GRPCPort:                "50054",
		TransactionDBDSN:        "postgres://x",
		DBMaxOpenConns:          25,
		DBMaxIdleConns:          10,
		DBConnMaxLifetime:       time.Minute,
		DBConnMaxIdleTime:       time.Minute,
		DBConnectTimeout:        time.Millisecond,
		DBConnectMaxRetries:     1,
		DBConnectInitialBackoff: time.Millisecond,
		DBConnectMaxBackoff:     time.Millisecond,
		GracefulShutdownTimeout: time.Second,
	}

	_, err := connector.ConnectWithRetry(context.Background(), cfg)
	if err == nil {
		t.Fatalf("expected ping error because sql.DB is not initialized")
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestBackoffDuration(t *testing.T) {
	got := BackoffDuration(3, time.Second, 5*time.Second)
	if got != 5*time.Second {
		t.Fatalf("expected capped backoff 5s, got %s", got)
	}
}

func TestWaitForCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitFor(ctx, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
