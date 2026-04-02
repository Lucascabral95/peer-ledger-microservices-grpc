package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lib/pq"
)

type mockStarter struct {
	queryRowFn func(ctx context.Context, query string, args ...any) RowScanner
	beginTxFn  func(ctx context.Context, opts *sql.TxOptions) (Tx, error)
	beginCalls int
}

func (m *mockStarter) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return m.queryRowFn(ctx, query, args...)
}

func (m *mockStarter) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	m.beginCalls++
	return m.beginTxFn(ctx, opts)
}

type mockTx struct {
	queryRowFn     func(ctx context.Context, query string, args ...any) RowScanner
	queryFn        func(ctx context.Context, query string, args ...any) (Rows, error)
	execFn         func(ctx context.Context, query string, args ...any) (sql.Result, error)
	commitFn       func() error
	rollbackFn     func() error
	commitCalled   bool
	rollbackCalled bool
}

func (m *mockTx) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return m.queryRowFn(ctx, query, args...)
}

func (m *mockTx) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return m.queryFn(ctx, query, args...)
}

func (m *mockTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return m.execFn(ctx, query, args...)
}

func (m *mockTx) Commit() error {
	m.commitCalled = true
	if m.commitFn != nil {
		return m.commitFn()
	}
	return nil
}

func (m *mockTx) Rollback() error {
	m.rollbackCalled = true
	if m.rollbackFn != nil {
		return m.rollbackFn()
	}
	return nil
}

type mockRow struct {
	scanFn func(dest ...any) error
}

func (m mockRow) Scan(dest ...any) error {
	return m.scanFn(dest...)
}

type mockRows struct {
	data      [][]any
	index     int
	err       error
	wasClosed bool
}

func (m *mockRows) Next() bool {
	return m.index < len(m.data)
}

func (m *mockRows) Scan(dest ...any) error {
	if m.index >= len(m.data) {
		return sql.ErrNoRows
	}

	row := m.data[m.index]
	m.index++
	if len(row) != len(dest) {
		return errors.New("scan size mismatch")
	}

	for i := range dest {
		switch p := dest[i].(type) {
		case *string:
			v, ok := row[i].(string)
			if !ok {
				return errors.New("invalid string destination")
			}
			*p = v
		default:
			return errors.New("unsupported scan destination")
		}
	}

	return nil
}

func (m *mockRows) Err() error { return m.err }

func (m *mockRows) Close() error {
	m.wasClosed = true
	return nil
}

type mockResult struct {
	rows int64
}

func (m mockResult) LastInsertId() (int64, error) { return 0, nil }
func (m mockResult) RowsAffected() (int64, error) { return m.rows, nil }

func TestTransfer_Success_Commit(t *testing.T) {
	tx := &mockTx{}
	tx.queryFn = func(ctx context.Context, query string, args ...any) (Rows, error) {
		return &mockRows{
			data: [][]any{
				{"user-001", "1000.00"},
				{"user-002", "200.00"},
			},
		}, nil
	}
	tx.queryRowFn = func(ctx context.Context, query string, args ...any) RowScanner {
		return mockRow{scanFn: func(dest ...any) error {
			*(dest[0].(*string)) = "tx-1"
			return nil
		}}
	}
	tx.execFn = func(ctx context.Context, query string, args ...any) (sql.Result, error) {
		return mockResult{rows: 1}, nil
	}

	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				return sql.ErrNoRows
			}}
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return tx, nil
		},
	}

	repo, err := NewWalletRepository(starter)
	if err != nil {
		t.Fatalf("NewWalletRepository() error: %v", err)
	}

	res, err := repo.Transfer(context.Background(), TransferInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    10000,
		IdempotencyKey: "k-success",
	})
	if err != nil {
		t.Fatalf("Transfer() unexpected error: %v", err)
	}
	if res.TransactionID != "tx-1" {
		t.Fatalf("expected tx-1, got %s", res.TransactionID)
	}
	if res.SenderBalanceCents != 90000 {
		t.Fatalf("expected sender balance 90000, got %d", res.SenderBalanceCents)
	}
	if !tx.commitCalled {
		t.Fatalf("expected commit called")
	}
	if tx.rollbackCalled {
		t.Fatalf("did not expect rollback on success")
	}
}

func TestTransfer_InsufficientFunds_Rollback(t *testing.T) {
	tx := &mockTx{}
	tx.queryFn = func(ctx context.Context, query string, args ...any) (Rows, error) {
		return &mockRows{
			data: [][]any{
				{"user-001", "50.00"},
				{"user-002", "200.00"},
			},
		}, nil
	}
	tx.queryRowFn = func(ctx context.Context, query string, args ...any) RowScanner {
		return mockRow{scanFn: func(dest ...any) error { return nil }}
	}
	tx.execFn = func(ctx context.Context, query string, args ...any) (sql.Result, error) {
		return mockResult{rows: 1}, nil
	}

	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error { return sql.ErrNoRows }}
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return tx, nil
		},
	}

	repo, err := NewWalletRepository(starter)
	if err != nil {
		t.Fatalf("NewWalletRepository() error: %v", err)
	}

	_, err = repo.Transfer(context.Background(), TransferInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    10000,
		IdempotencyKey: "k-insufficient",
	})
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
	if tx.commitCalled {
		t.Fatalf("did not expect commit on insufficient funds")
	}
	if !tx.rollbackCalled {
		t.Fatalf("expected rollback called")
	}
}

func TestTransfer_IdempotencyCached_NoTx(t *testing.T) {
	payload := idempotencyRecord{
		SenderID:           "user-001",
		ReceiverID:         "user-002",
		AmountCents:        10000,
		TransactionID:      "tx-cached",
		SenderBalanceCents: 55500,
	}
	raw, _ := json.Marshal(payload)

	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = string(raw)
				return nil
			}}
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return nil, errors.New("begin should not be called")
		},
	}

	repo, _ := NewWalletRepository(starter)
	res, err := repo.Transfer(context.Background(), TransferInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    10000,
		IdempotencyKey: "k-cached",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if starter.beginCalls != 0 {
		t.Fatalf("expected no transaction for cached idempotency")
	}
	if res.TransactionID != "tx-cached" {
		t.Fatalf("expected tx-cached, got %s", res.TransactionID)
	}
}

func TestTransfer_IdempotencyMismatch(t *testing.T) {
	payload := idempotencyRecord{
		SenderID:           "user-001",
		ReceiverID:         "user-002",
		AmountCents:        999,
		TransactionID:      "tx-cached",
		SenderBalanceCents: 55500,
	}
	raw, _ := json.Marshal(payload)

	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = string(raw)
				return nil
			}}
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return nil, errors.New("begin should not be called")
		},
	}

	repo, _ := NewWalletRepository(starter)
	_, err := repo.Transfer(context.Background(), TransferInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    10000,
		IdempotencyKey: "k-mismatch",
	})
	if !errors.Is(err, ErrIdempotencyPayloadMismatch) {
		t.Fatalf("expected ErrIdempotencyPayloadMismatch, got %v", err)
	}
	if starter.beginCalls != 0 {
		t.Fatalf("expected no transaction on mismatch")
	}
}

func TestTransfer_UniqueConflict_ReturnsCached(t *testing.T) {
	call := 0
	tx := &mockTx{}
	tx.queryFn = func(ctx context.Context, query string, args ...any) (Rows, error) {
		return &mockRows{
			data: [][]any{
				{"user-001", "1000.00"},
				{"user-002", "100.00"},
			},
		}, nil
	}
	tx.queryRowFn = func(ctx context.Context, query string, args ...any) RowScanner {
		return mockRow{scanFn: func(dest ...any) error {
			*(dest[0].(*string)) = "tx-temp"
			return nil
		}}
	}
	tx.execFn = func(ctx context.Context, query string, args ...any) (sql.Result, error) {
		if strings.Contains(strings.ToLower(query), "insert into idempotency_keys") {
			return nil, &pq.Error{Code: "23505"}
		}
		return mockResult{rows: 1}, nil
	}

	cached := idempotencyRecord{
		SenderID:           "user-001",
		ReceiverID:         "user-002",
		AmountCents:        10000,
		TransactionID:      "tx-cached-after-conflict",
		SenderBalanceCents: 90000,
	}
	raw, _ := json.Marshal(cached)

	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			call++
			return mockRow{scanFn: func(dest ...any) error {
				if call == 1 {
					return sql.ErrNoRows
				}
				*(dest[0].(*string)) = string(raw)
				return nil
			}}
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return tx, nil
		},
	}

	repo, _ := NewWalletRepository(starter)
	res, err := repo.Transfer(context.Background(), TransferInput{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    10000,
		IdempotencyKey: "k-conflict",
	})
	if err != nil {
		t.Fatalf("unexpected error after conflict fallback: %v", err)
	}
	if res.TransactionID != "tx-cached-after-conflict" {
		t.Fatalf("expected cached tx id, got %s", res.TransactionID)
	}
	if !tx.rollbackCalled {
		t.Fatalf("expected rollback on unique conflict path")
	}
}

func TestDecimalConversions(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"1000.32", 100032},
		{"0.01", 1},
		{"5", 500},
		{"-2.50", -250},
	}

	for _, tc := range cases {
		got, err := decimalStringToCents(tc.in)
		if err != nil {
			t.Fatalf("decimalStringToCents(%q) unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("decimalStringToCents(%q) expected %d, got %d", tc.in, tc.want, got)
		}
	}

	if got := centsToDecimalString(100032); got != "1000.32" {
		t.Fatalf("expected 1000.32, got %s", got)
	}
}
