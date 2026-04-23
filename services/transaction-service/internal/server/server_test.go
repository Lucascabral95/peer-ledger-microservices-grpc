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
	recordFn             func(ctx context.Context, input repository.RecordInput) error
	getHistoryFn         func(ctx context.Context, userID string) ([]repository.HistoryRecord, error)
	getTransferSummaryFn func(ctx context.Context, userID, timezone string) (repository.TransferSummary, error)
	listTransfersFn      func(ctx context.Context, userID string, direction repository.TransferDirection, from, to *time.Time, limit int) ([]repository.HistoryRecord, bool, error)
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

func (m mockTransactionStore) GetTransferSummary(ctx context.Context, userID, timezone string) (repository.TransferSummary, error) {
	if m.getTransferSummaryFn == nil {
		return repository.TransferSummary{}, errors.New("getTransferSummaryFn not configured")
	}
	return m.getTransferSummaryFn(ctx, userID, timezone)
}

func (m mockTransactionStore) ListTransfers(ctx context.Context, userID string, direction repository.TransferDirection, from, to *time.Time, limit int) ([]repository.HistoryRecord, bool, error) {
	if m.listTransfersFn == nil {
		return nil, false, errors.New("listTransfersFn not configured")
	}
	return m.listTransfersFn(ctx, userID, direction, from, to, limit)
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
		getTransferSummaryFn: func(context.Context, string, string) (repository.TransferSummary, error) {
			return repository.TransferSummary{}, nil
		},
		listTransfersFn: func(context.Context, string, repository.TransferDirection, *time.Time, *time.Time, int) ([]repository.HistoryRecord, bool, error) {
			return nil, false, nil
		},
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
		getTransferSummaryFn: func(context.Context, string, string) (repository.TransferSummary, error) {
			return repository.TransferSummary{}, nil
		},
		listTransfersFn: func(context.Context, string, repository.TransferDirection, *time.Time, *time.Time, int) ([]repository.HistoryRecord, bool, error) {
			return nil, false, nil
		},
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
		getTransferSummaryFn: func(context.Context, string, string) (repository.TransferSummary, error) {
			return repository.TransferSummary{}, nil
		},
		listTransfersFn: func(context.Context, string, repository.TransferDirection, *time.Time, *time.Time, int) ([]repository.HistoryRecord, bool, error) {
			return nil, false, nil
		},
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
			if input.SenderBalanceAfterCents != 989968 || input.ReceiverBalanceAfterCents != 501032 {
				t.Fatalf("unexpected balance_after cents: %+v", input)
			}
			return nil
		},
		getHistoryFn: func(context.Context, string) ([]repository.HistoryRecord, error) { return nil, nil },
		getTransferSummaryFn: func(context.Context, string, string) (repository.TransferSummary, error) {
			return repository.TransferSummary{}, nil
		},
		listTransfersFn: func(context.Context, string, repository.TransferDirection, *time.Time, *time.Time, int) ([]repository.HistoryRecord, bool, error) {
			return nil, false, nil
		},
	})

	resp, err := srv.Record(context.Background(), &transactionpb.RecordRequest{
		TransactionId:        "tx-1",
		SenderId:             "user-001",
		ReceiverId:           "user-002",
		Amount:               10.32,
		IdempotencyKey:       "k-1",
		SenderBalanceAfter:   9899.68,
		ReceiverBalanceAfter: 5010.32,
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
		getTransferSummaryFn: func(context.Context, string, string) (repository.TransferSummary, error) {
			return repository.TransferSummary{}, nil
		},
		listTransfersFn: func(context.Context, string, repository.TransferDirection, *time.Time, *time.Time, int) ([]repository.HistoryRecord, bool, error) {
			return nil, false, nil
		},
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
					TransactionID:             "tx-1",
					SenderID:                  "user-001",
					ReceiverID:                "user-002",
					AmountCents:               1234,
					Status:                    "completed",
					CreatedAt:                 now,
					SenderBalanceAfterCents:   8766,
					ReceiverBalanceAfterCents: 21234,
				},
			}, nil
		},
		getTransferSummaryFn: func(context.Context, string, string) (repository.TransferSummary, error) {
			return repository.TransferSummary{}, nil
		},
		listTransfersFn: func(context.Context, string, repository.TransferDirection, *time.Time, *time.Time, int) ([]repository.HistoryRecord, bool, error) {
			return nil, false, nil
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
	if resp.GetRecords()[0].GetSenderBalanceAfter() != 87.66 || resp.GetRecords()[0].GetReceiverBalanceAfter() != 212.34 {
		t.Fatalf("unexpected balance_after values: %+v", resp.GetRecords()[0])
	}
}

func TestGetTransferSummary_InvalidTimezone(t *testing.T) {
	srv, _ := NewTransactionGRPCServer(mockTransactionStore{
		recordFn:     func(context.Context, repository.RecordInput) error { return nil },
		getHistoryFn: func(context.Context, string) ([]repository.HistoryRecord, error) { return nil, nil },
		getTransferSummaryFn: func(context.Context, string, string) (repository.TransferSummary, error) {
			return repository.TransferSummary{}, nil
		},
		listTransfersFn: func(context.Context, string, repository.TransferDirection, *time.Time, *time.Time, int) ([]repository.HistoryRecord, bool, error) {
			return nil, false, nil
		},
	})

	_, err := srv.GetTransferSummary(context.Background(), &transactionpb.GetTransferSummaryRequest{
		UserId:   "user-001",
		Timezone: "invalid/timezone",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestListTransfers_Success(t *testing.T) {
	now := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
	srv, _ := NewTransactionGRPCServer(mockTransactionStore{
		recordFn:     func(context.Context, repository.RecordInput) error { return nil },
		getHistoryFn: func(context.Context, string) ([]repository.HistoryRecord, error) { return nil, nil },
		getTransferSummaryFn: func(context.Context, string, string) (repository.TransferSummary, error) {
			return repository.TransferSummary{}, nil
		},
		listTransfersFn: func(context.Context, string, repository.TransferDirection, *time.Time, *time.Time, int) ([]repository.HistoryRecord, bool, error) {
			return []repository.HistoryRecord{
				{
					TransactionID:             "tx-3",
					SenderID:                  "user-001",
					ReceiverID:                "user-002",
					AmountCents:               3000,
					Status:                    "completed",
					CreatedAt:                 now,
					SenderBalanceAfterCents:   7000,
					ReceiverBalanceAfterCents: 23000,
				},
			}, true, nil
		},
	})

	resp, err := srv.ListTransfers(context.Background(), &transactionpb.ListTransfersRequest{
		UserId:    "user-001",
		Direction: transactionpb.TransferDirection_TRANSFER_DIRECTION_ALL,
		Limit:     20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetHasMore() {
		t.Fatalf("expected has_more=true")
	}
	if len(resp.GetRecords()) != 1 {
		t.Fatalf("expected 1 record, got %d", len(resp.GetRecords()))
	}
	if resp.GetRecords()[0].GetTransactionId() != "tx-3" {
		t.Fatalf("unexpected transaction id %q", resp.GetRecords()[0].GetTransactionId())
	}
	if resp.GetRecords()[0].GetSenderBalanceAfter() != 70 || resp.GetRecords()[0].GetReceiverBalanceAfter() != 230 {
		t.Fatalf("unexpected balance_after values: %+v", resp.GetRecords()[0])
	}
}
