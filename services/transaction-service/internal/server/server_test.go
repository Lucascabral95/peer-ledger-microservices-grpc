package server

import (
	"context"
	"errors"
	"testing"
	"time"

	transactionpb "github.com/peer-ledger/gen/transaction"
	"github.com/peer-ledger/services/transaction-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockTransactionStore struct {
	recordFn     func(ctx context.Context, input repository.RecordInput) error
	getHistoryFn func(ctx context.Context, userID string) ([]repository.HistoryRecord, error)
}

func (m mockTransactionStore) Record(ctx context.Context, input repository.RecordInput) error {
	if m.recordFn == nil {
		return errors.New("recordFn not configured")
	}
	return m.recordFn(ctx, input)
}

func (m mockTransactionStore) GetHistory(ctx context.Context, userID string) ([]repository.HistoryRecord, error) {
	if m.getHistoryFn == nil {
		return nil, errors.New("getHistoryFn not configured")
	}
	return m.getHistoryFn(ctx, userID)
}

func TestNewTransactionGRPCServer_NilStore(t *testing.T) {
	_, err := NewTransactionGRPCServer(nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRecord_InvalidRequest(t *testing.T) {
	srv, _ := NewTransactionGRPCServer(mockTransactionStore{
		recordFn:     func(context.Context, repository.RecordInput) error { return nil },
		getHistoryFn: func(context.Context, string) ([]repository.HistoryRecord, error) { return nil, nil },
	})

	_, err := srv.Record(context.Background(), &transactionpb.RecordRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestRecord_Mismatch(t *testing.T) {
	srv, _ := NewTransactionGRPCServer(mockTransactionStore{
		recordFn: func(context.Context, repository.RecordInput) error {
			return repository.ErrIdempotencyPayloadMismatch
		},
		getHistoryFn: func(context.Context, string) ([]repository.HistoryRecord, error) { return nil, nil },
	})

	_, err := srv.Record(context.Background(), &transactionpb.RecordRequest{
		TransactionId:  "tx-1",
		SenderId:       "user-001",
		ReceiverId:     "user-002",
		Amount:         10.32,
		IdempotencyKey: "k-1",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestRecord_Conflict(t *testing.T) {
	srv, _ := NewTransactionGRPCServer(mockTransactionStore{
		recordFn: func(context.Context, repository.RecordInput) error {
			return repository.ErrTransactionIDConflict
		},
		getHistoryFn: func(context.Context, string) ([]repository.HistoryRecord, error) { return nil, nil },
	})

	_, err := srv.Record(context.Background(), &transactionpb.RecordRequest{
		TransactionId:  "tx-1",
		SenderId:       "user-001",
		ReceiverId:     "user-002",
		Amount:         10.32,
		IdempotencyKey: "k-1",
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", status.Code(err))
	}
}

func TestRecord_Success(t *testing.T) {
	srv, _ := NewTransactionGRPCServer(mockTransactionStore{
		recordFn: func(_ context.Context, input repository.RecordInput) error {
			if input.AmountCents != 1032 {
				t.Fatalf("expected 1032 cents, got %d", input.AmountCents)
			}
			return nil
		},
		getHistoryFn: func(context.Context, string) ([]repository.HistoryRecord, error) { return nil, nil },
	})

	resp, err := srv.Record(context.Background(), &transactionpb.RecordRequest{
		TransactionId:  "tx-1",
		SenderId:       "user-001",
		ReceiverId:     "user-002",
		Amount:         10.32,
		IdempotencyKey: "k-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success=true")
	}
}

func TestGetHistory_InvalidRequest(t *testing.T) {
	srv, _ := NewTransactionGRPCServer(mockTransactionStore{
		recordFn:     func(context.Context, repository.RecordInput) error { return nil },
		getHistoryFn: func(context.Context, string) ([]repository.HistoryRecord, error) { return nil, nil },
	})

	_, err := srv.GetHistory(context.Background(), &transactionpb.GetHistoryRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestGetHistory_Success(t *testing.T) {
	now := time.Date(2026, 4, 1, 3, 4, 5, 123000000, time.UTC)
	srv, _ := NewTransactionGRPCServer(mockTransactionStore{
		recordFn: func(context.Context, repository.RecordInput) error { return nil },
		getHistoryFn: func(context.Context, string) ([]repository.HistoryRecord, error) {
			return []repository.HistoryRecord{
				{
					TransactionID: "tx-1",
					SenderID:      "user-001",
					ReceiverID:    "user-002",
					AmountCents:   1234,
					Status:        "completed",
					CreatedAt:     now,
				},
			}, nil
		},
	})

	resp, err := srv.GetHistory(context.Background(), &transactionpb.GetHistoryRequest{
		UserId: "user-001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetRecords()) != 1 {
		t.Fatalf("expected 1 record, got %d", len(resp.GetRecords()))
	}
	if resp.GetRecords()[0].GetAmount() != 12.34 {
		t.Fatalf("expected amount 12.34, got %v", resp.GetRecords()[0].GetAmount())
	}
}
