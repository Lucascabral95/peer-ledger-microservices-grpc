package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
)

type mockStarter struct {
	queryRowFn func(ctx context.Context, query string, args ...any) RowScanner
	queryFn    func(ctx context.Context, query string, args ...any) (Rows, error)
	beginTxFn  func(ctx context.Context, opts *sql.TxOptions) (Tx, error)
	beginCalls int
}

func (m *mockStarter) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return m.queryRowFn(ctx, query, args...)
}

func (m *mockStarter) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return m.queryFn(ctx, query, args...)
}

func (m *mockStarter) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	m.beginCalls++
	return m.beginTxFn(ctx, opts)
}

type mockTx struct {
	queryRowFn     func(ctx context.Context, query string, args ...any) RowScanner
	queryFn        func(ctx context.Context, query string, args ...any) (Rows, error)
	execFn         func(ctx context.Context, query string, args ...any) (sql.Result, error)
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
	return nil
}

func (m *mockTx) Rollback() error {
	m.rollbackCalled = true
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
		case *time.Time:
			v, ok := row[i].(time.Time)
			if !ok {
				return errors.New("invalid time destination")
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

func TestRecord_Success(t *testing.T) {
	tx := &mockTx{
		execFn: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return mockResult{rows: 1}, nil
		},
	}

	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error { return sql.ErrNoRows }}
		},
		queryFn: func(ctx context.Context, query string, args ...any) (Rows, error) {
			return &mockRows{}, nil
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return tx, nil
		},
	}

	repo, err := NewTransactionRepository(starter)
	if err != nil {
		t.Fatalf("NewTransactionRepository() error: %v", err)
	}

	err = repo.Record(context.Background(), RecordInput{
		TransactionID:  "tx-1",
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    1000,
		IdempotencyKey: "k-1",
	})
	if err != nil {
		t.Fatalf("Record() unexpected error: %v", err)
	}
	if !tx.commitCalled {
		t.Fatalf("expected commit called")
	}
	if tx.rollbackCalled {
		t.Fatalf("did not expect rollback")
	}
}

func TestRecord_IdempotencyCached_NoTx(t *testing.T) {
	now := time.Now().UTC()
	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "tx-1"
				*(dest[1].(*string)) = "user-001"
				*(dest[2].(*string)) = "user-002"
				*(dest[3].(*string)) = "10.00"
				*(dest[4].(*string)) = "k-1"
				*(dest[5].(*string)) = "completed"
				*(dest[6].(*time.Time)) = now
				*(dest[7].(*string)) = "0.00"
				*(dest[8].(*string)) = "0.00"
				return nil
			}}
		},
		queryFn: func(ctx context.Context, query string, args ...any) (Rows, error) {
			return &mockRows{}, nil
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return nil, errors.New("begin should not be called")
		},
	}

	repo, _ := NewTransactionRepository(starter)
	err := repo.Record(context.Background(), RecordInput{
		TransactionID:  "tx-1",
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    1000,
		IdempotencyKey: "k-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if starter.beginCalls != 0 {
		t.Fatalf("expected no transaction")
	}
}

func TestRecord_IdempotencyMismatch(t *testing.T) {
	now := time.Now().UTC()
	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "tx-1"
				*(dest[1].(*string)) = "user-001"
				*(dest[2].(*string)) = "user-002"
				*(dest[3].(*string)) = "10.00"
				*(dest[4].(*string)) = "k-1"
				*(dest[5].(*string)) = "completed"
				*(dest[6].(*time.Time)) = now
				*(dest[7].(*string)) = "0.00"
				*(dest[8].(*string)) = "0.00"
				return nil
			}}
		},
		queryFn: func(ctx context.Context, query string, args ...any) (Rows, error) {
			return &mockRows{}, nil
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return nil, errors.New("begin should not be called")
		},
	}

	repo, _ := NewTransactionRepository(starter)
	err := repo.Record(context.Background(), RecordInput{
		TransactionID:  "tx-2",
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    1000,
		IdempotencyKey: "k-1",
	})
	if !errors.Is(err, ErrIdempotencyPayloadMismatch) {
		t.Fatalf("expected ErrIdempotencyPayloadMismatch, got %v", err)
	}
}

func TestRecord_UniqueKeyConflict_ReturnsCached(t *testing.T) {
	now := time.Now().UTC()
	call := 0
	tx := &mockTx{
		execFn: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, &pq.Error{Code: "23505", Constraint: "transactions_idempotency_key_key"}
		},
	}

	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			call++
			return mockRow{scanFn: func(dest ...any) error {
				if call == 1 {
					return sql.ErrNoRows
				}
				*(dest[0].(*string)) = "tx-1"
				*(dest[1].(*string)) = "user-001"
				*(dest[2].(*string)) = "user-002"
				*(dest[3].(*string)) = "10.00"
				*(dest[4].(*string)) = "k-1"
				*(dest[5].(*string)) = "completed"
				*(dest[6].(*time.Time)) = now
				*(dest[7].(*string)) = "0.00"
				*(dest[8].(*string)) = "0.00"
				return nil
			}}
		},
		queryFn: func(ctx context.Context, query string, args ...any) (Rows, error) {
			return &mockRows{}, nil
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return tx, nil
		},
	}

	repo, _ := NewTransactionRepository(starter)
	err := repo.Record(context.Background(), RecordInput{
		TransactionID:  "tx-1",
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    1000,
		IdempotencyKey: "k-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tx.rollbackCalled {
		t.Fatalf("expected rollback on conflict path")
	}
}

func TestRecord_TransactionIDConflict(t *testing.T) {
	now := time.Now().UTC()
	call := 0
	tx := &mockTx{
		execFn: func(ctx context.Context, query string, args ...any) (sql.Result, error) {
			return nil, &pq.Error{Code: "23505", Constraint: "transactions_pkey"}
		},
	}

	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			call++
			return mockRow{scanFn: func(dest ...any) error {
				if call == 1 {
					return sql.ErrNoRows
				}
				*(dest[0].(*string)) = "tx-1"
				*(dest[1].(*string)) = "user-001"
				*(dest[2].(*string)) = "user-003"
				*(dest[3].(*string)) = "10.00"
				*(dest[4].(*string)) = "other-key"
				*(dest[5].(*string)) = "completed"
				*(dest[6].(*time.Time)) = now
				*(dest[7].(*string)) = "0.00"
				*(dest[8].(*string)) = "0.00"
				return nil
			}}
		},
		queryFn: func(ctx context.Context, query string, args ...any) (Rows, error) {
			return &mockRows{}, nil
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return tx, nil
		},
	}

	repo, _ := NewTransactionRepository(starter)
	err := repo.Record(context.Background(), RecordInput{
		TransactionID:  "tx-1",
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		AmountCents:    1000,
		IdempotencyKey: "k-1",
	})
	if !errors.Is(err, ErrTransactionIDConflict) {
		t.Fatalf("expected ErrTransactionIDConflict, got %v", err)
	}
}

func TestGetHistory_Empty(t *testing.T) {
	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error { return sql.ErrNoRows }}
		},
		queryFn: func(ctx context.Context, query string, args ...any) (Rows, error) {
			return &mockRows{}, nil
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return nil, errors.New("not expected")
		},
	}

	repo, _ := NewTransactionRepository(starter)
	records, err := repo.GetHistory(context.Background(), "user-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected empty history, got %d", len(records))
	}
}

func TestGetHistory_ReturnsRecords(t *testing.T) {
	now := time.Now().UTC()
	rows := &mockRows{
		data: [][]any{
			{"tx-2", "user-001", "user-002", "12.34", "completed", now, "87.66", "212.34"},
			{"tx-1", "user-003", "user-001", "1.00", "completed", now.Add(-time.Minute), "49.00", "88.66"},
		},
	}

	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error { return sql.ErrNoRows }}
		},
		queryFn: func(ctx context.Context, query string, args ...any) (Rows, error) {
			return rows, nil
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return nil, errors.New("not expected")
		},
	}

	repo, _ := NewTransactionRepository(starter)
	records, err := repo.GetHistory(context.Background(), "user-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].AmountCents != 1234 {
		t.Fatalf("expected amount 1234, got %d", records[0].AmountCents)
	}
	if records[0].SenderBalanceAfterCents != 8766 || records[0].ReceiverBalanceAfterCents != 21234 {
		t.Fatalf("unexpected balance_after values: %+v", records[0])
	}
	if !rows.wasClosed {
		t.Fatalf("expected rows closed")
	}
}

func TestDecimalConversions(t *testing.T) {
	got, err := decimalStringToCents("1000.32")
	if err != nil {
		t.Fatalf("decimalStringToCents() error: %v", err)
	}
	if got != 100032 {
		t.Fatalf("expected 100032, got %d", got)
	}
	if centsToDecimalString(100032) != "1000.32" {
		t.Fatalf("unexpected decimal string")
	}
}

func TestGetTransferSummary_Success(t *testing.T) {
	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "180.00"
				*(dest[1].(*string)) = "242.50"
				*(dest[2].(*int64)) = 12
				*(dest[3].(*int64)) = 16
				*(dest[4].(*int64)) = 1
				*(dest[5].(*int64)) = 2
				*(dest[6].(*string)) = "15.00"
				*(dest[7].(*string)) = "30.00"
				return nil
			}}
		},
		queryFn: func(ctx context.Context, query string, args ...any) (Rows, error) {
			return &mockRows{}, nil
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return nil, errors.New("not expected")
		},
	}

	repo, _ := NewTransactionRepository(starter)
	summary, err := repo.GetTransferSummary(context.Background(), "user-001", "America/Argentina/Buenos_Aires")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.SentTotalCents != 18000 || summary.ReceivedTotalCents != 24250 {
		t.Fatalf("unexpected totals: %+v", summary)
	}
	if summary.SentCountToday != 1 || summary.ReceivedCountToday != 2 {
		t.Fatalf("unexpected today counts: %+v", summary)
	}
}

func TestListTransfers_ReturnsHasMore(t *testing.T) {
	now := time.Now().UTC()
	rows := &mockRows{
		data: [][]any{
			{"tx-3", "user-001", "user-002", "30.00", "completed", now, "70.00", "230.00"},
			{"tx-2", "user-003", "user-001", "20.00", "completed", now.Add(-time.Minute), "30.00", "90.00"},
			{"tx-1", "user-001", "user-004", "10.00", "completed", now.Add(-2 * time.Minute), "100.00", "10.00"},
		},
	}

	starter := &mockStarter{
		queryRowFn: func(ctx context.Context, query string, args ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error { return sql.ErrNoRows }}
		},
		queryFn: func(ctx context.Context, query string, args ...any) (Rows, error) {
			return rows, nil
		},
		beginTxFn: func(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
			return nil, errors.New("not expected")
		},
	}

	repo, _ := NewTransactionRepository(starter)
	records, hasMore, err := repo.ListTransfers(context.Background(), "user-001", TransferDirectionAll, nil, nil, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasMore {
		t.Fatalf("expected hasMore=true")
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].TransactionID != "tx-3" {
		t.Fatalf("expected tx-3 first, got %s", records[0].TransactionID)
	}
}
