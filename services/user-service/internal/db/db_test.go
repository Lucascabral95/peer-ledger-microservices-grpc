package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/peer-ledger/services/user-service/internal/config"
)

func TestBackoffDuration(t *testing.T) {
	initial := 500 * time.Millisecond
	max := 8 * time.Second

	if got := BackoffDuration(0, initial, max); got != 500*time.Millisecond {
		t.Fatalf("attempt 0 expected 500ms, got %s", got)
	}
	if got := BackoffDuration(1, initial, max); got != 1*time.Second {
		t.Fatalf("attempt 1 expected 1s, got %s", got)
	}
	if got := BackoffDuration(10, initial, max); got != max {
		t.Fatalf("attempt 10 expected capped max %s, got %s", max, got)
	}
}

func TestConnectWithRetry_NilConfig(t *testing.T) {
	conn := NewConnectorWithDeps(nil, nil)
	_, err := conn.ConnectWithRetry(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected error for nil config")
	}
}

func TestConnectWithRetry_OpenFails(t *testing.T) {
	cfg := &config.Config{
		GRPCPort:                "50051",
		UserDBDSN:               "postgres://example",
		DBMaxOpenConns:          10,
		DBMaxIdleConns:          5,
		DBConnMaxLifetime:       time.Minute,
		DBConnMaxIdleTime:       time.Minute,
		DBConnectTimeout:        time.Second,
		DBConnectMaxRetries:     2,
		DBConnectInitialBackoff: time.Millisecond,
		DBConnectMaxBackoff:     10 * time.Millisecond,
		GracefulShutdownTimeout: time.Second,
	}

	openCalls := 0
	waitCalls := 0

	conn := NewConnectorWithDeps(
		func(driverName, dsn string) (*sql.DB, error) {
			openCalls++
			return nil, errors.New("open failed")
		},
		func(ctx context.Context, d time.Duration) error {
			waitCalls++
			return nil
		},
	)

	_, err := conn.ConnectWithRetry(context.Background(), cfg)
	if err == nil {
		t.Fatalf("expected connection error")
	}
	if !strings.Contains(err.Error(), "db connect failed after 2 retries") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if openCalls != 3 {
		t.Fatalf("expected 3 open attempts, got %d", openCalls)
	}
	if waitCalls != 2 {
		t.Fatalf("expected 2 wait calls, got %d", waitCalls)
	}
}

func TestConnectWithRetry_WaitCanceled(t *testing.T) {
	cfg := &config.Config{
		GRPCPort:                "50051",
		UserDBDSN:               "postgres://example",
		DBMaxOpenConns:          10,
		DBMaxIdleConns:          5,
		DBConnMaxLifetime:       time.Minute,
		DBConnMaxIdleTime:       time.Minute,
		DBConnectTimeout:        time.Second,
		DBConnectMaxRetries:     3,
		DBConnectInitialBackoff: time.Millisecond,
		DBConnectMaxBackoff:     10 * time.Millisecond,
		GracefulShutdownTimeout: time.Second,
	}

	conn := NewConnectorWithDeps(
		func(driverName, dsn string) (*sql.DB, error) {
			return nil, errors.New("open failed")
		},
		func(ctx context.Context, d time.Duration) error {
			return context.Canceled
		},
	)

	_, err := conn.ConnectWithRetry(context.Background(), cfg)
	if err == nil {
		t.Fatalf("expected connection cancel error")
	}
	if !strings.Contains(err.Error(), "db connect canceled") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
