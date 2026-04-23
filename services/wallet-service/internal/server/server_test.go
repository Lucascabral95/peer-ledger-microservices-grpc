package server

import (
	"context"
	"errors"
	"testing"
	"time"

	walletpb "github.com/peer-ledger/gen/wallet"
	"github.com/peer-ledger/services/wallet-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockWalletStore struct {
	getBalanceFn      func(ctx context.Context, userID string) (int64, error)
	createWalletFn    func(ctx context.Context, userID string, initialBalanceCents int64) (int64, error)
	topUpFn           func(ctx context.Context, userID string, amountCents int64) (int64, error)
	getTopUpSummaryFn func(ctx context.Context, userID, timezone string) (repository.TopUpSummary, error)
	listTopUpsFn      func(ctx context.Context, userID string, from, to *time.Time, limit int) ([]repository.TopUpRecord, bool, error)
	transferFn        func(ctx context.Context, input repository.TransferInput) (repository.TransferResult, error)
}

func (m mockWalletStore) GetBalance(ctx context.Context, userID string) (int64, error) {
	if m.getBalanceFn == nil {
		return 0, errors.New("getBalanceFn not configured")
	}
	return m.getBalanceFn(ctx, userID)
}

func (m mockWalletStore) CreateWallet(ctx context.Context, userID string, initialBalanceCents int64) (int64, error) {
	if m.createWalletFn == nil {
		return 0, errors.New("createWalletFn not configured")
	}
	return m.createWalletFn(ctx, userID, initialBalanceCents)
}

func (m mockWalletStore) TopUp(ctx context.Context, userID string, amountCents int64) (int64, error) {
	if m.topUpFn == nil {
		return 0, errors.New("topUpFn not configured")
	}
	return m.topUpFn(ctx, userID, amountCents)
}

func (m mockWalletStore) GetTopUpSummary(ctx context.Context, userID, timezone string) (repository.TopUpSummary, error) {
	if m.getTopUpSummaryFn == nil {
		return repository.TopUpSummary{}, errors.New("getTopUpSummaryFn not configured")
	}
	return m.getTopUpSummaryFn(ctx, userID, timezone)
}

func (m mockWalletStore) ListTopUps(ctx context.Context, userID string, from, to *time.Time, limit int) ([]repository.TopUpRecord, bool, error) {
	if m.listTopUpsFn == nil {
		return nil, false, errors.New("listTopUpsFn not configured")
	}
	return m.listTopUpsFn(ctx, userID, from, to, limit)
}

func (m mockWalletStore) Transfer(ctx context.Context, input repository.TransferInput) (repository.TransferResult, error) {
	if m.transferFn == nil {
		return repository.TransferResult{}, errors.New("transferFn not configured")
	}
	return m.transferFn(ctx, input)
}

func TestGetBalance_NotFound(t *testing.T) {
	srv, err := NewWalletGRPCServer(mockWalletStore{
		getBalanceFn: func(context.Context, string) (int64, error) {
			return 0, repository.ErrWalletNotFound
		},
		createWalletFn: func(context.Context, string, int64) (int64, error) {
			return 0, nil
		},
		topUpFn: func(context.Context, string, int64) (int64, error) {
			return 0, nil
		},
		getTopUpSummaryFn: func(context.Context, string, string) (repository.TopUpSummary, error) {
			return repository.TopUpSummary{}, nil
		},
		listTopUpsFn: func(context.Context, string, *time.Time, *time.Time, int) ([]repository.TopUpRecord, bool, error) {
			return nil, false, nil
		},
		transferFn: func(context.Context, repository.TransferInput) (repository.TransferResult, error) {
			return repository.TransferResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewWalletGRPCServer() error = %v", err)
	}

	_, err = srv.GetBalance(context.Background(), &walletpb.GetBalanceRequest{
		UserId: "user-001",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (%v)", status.Code(err), err)
	}
}

func TestTransfer_InsufficientFunds(t *testing.T) {
	srv, err := NewWalletGRPCServer(mockWalletStore{
		getBalanceFn: func(context.Context, string) (int64, error) {
			return 0, nil
		},
		createWalletFn: func(context.Context, string, int64) (int64, error) {
			return 0, nil
		},
		topUpFn: func(context.Context, string, int64) (int64, error) {
			return 0, nil
		},
		getTopUpSummaryFn: func(context.Context, string, string) (repository.TopUpSummary, error) {
			return repository.TopUpSummary{}, nil
		},
		listTopUpsFn: func(context.Context, string, *time.Time, *time.Time, int) ([]repository.TopUpRecord, bool, error) {
			return nil, false, nil
		},
		transferFn: func(context.Context, repository.TransferInput) (repository.TransferResult, error) {
			return repository.TransferResult{}, repository.ErrInsufficientFunds
		},
	})
	if err != nil {
		t.Fatalf("NewWalletGRPCServer() error = %v", err)
	}

	_, err = srv.Transfer(context.Background(), &walletpb.TransferRequest{
		SenderId:       "user-001",
		ReceiverId:     "user-002",
		Amount:         1000,
		IdempotencyKey: "k1",
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v (%v)", status.Code(err), err)
	}
}

func TestTransfer_Success(t *testing.T) {
	const expectedTxID = "tx-123"

	srv, err := NewWalletGRPCServer(mockWalletStore{
		getBalanceFn: func(context.Context, string) (int64, error) {
			return 0, nil
		},
		createWalletFn: func(context.Context, string, int64) (int64, error) {
			return 0, nil
		},
		topUpFn: func(context.Context, string, int64) (int64, error) {
			return 0, nil
		},
		getTopUpSummaryFn: func(context.Context, string, string) (repository.TopUpSummary, error) {
			return repository.TopUpSummary{}, nil
		},
		listTopUpsFn: func(context.Context, string, *time.Time, *time.Time, int) ([]repository.TopUpRecord, bool, error) {
			return nil, false, nil
		},
		transferFn: func(_ context.Context, input repository.TransferInput) (repository.TransferResult, error) {
			if input.AmountCents != 100032 {
				t.Fatalf("expected amount cents 100032, got %d", input.AmountCents)
			}

			return repository.TransferResult{
				TransactionID:        expectedTxID,
				SenderBalanceCents:   9899968,
				ReceiverBalanceCents: 501032,
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewWalletGRPCServer() error = %v", err)
	}

	resp, err := srv.Transfer(context.Background(), &walletpb.TransferRequest{
		SenderId:       "user-001",
		ReceiverId:     "user-002",
		Amount:         1000.32,
		IdempotencyKey: "k-success",
	})
	if err != nil {
		t.Fatalf("Transfer() unexpected error = %v", err)
	}
	if resp.GetTransactionId() != expectedTxID {
		t.Fatalf("expected transaction_id %q, got %q", expectedTxID, resp.GetTransactionId())
	}
	if resp.GetSenderBalance() != 98999.68 {
		t.Fatalf("expected sender_balance 98999.68, got %v", resp.GetSenderBalance())
	}
	if resp.GetReceiverBalance() != 5010.32 {
		t.Fatalf("expected receiver_balance 5010.32, got %v", resp.GetReceiverBalance())
	}
}

func TestGetTopUpSummary_InvalidTimezone(t *testing.T) {
	srv, err := NewWalletGRPCServer(mockWalletStore{
		getBalanceFn:   func(context.Context, string) (int64, error) { return 0, nil },
		createWalletFn: func(context.Context, string, int64) (int64, error) { return 0, nil },
		topUpFn:        func(context.Context, string, int64) (int64, error) { return 0, nil },
		getTopUpSummaryFn: func(context.Context, string, string) (repository.TopUpSummary, error) {
			return repository.TopUpSummary{}, nil
		},
		listTopUpsFn: func(context.Context, string, *time.Time, *time.Time, int) ([]repository.TopUpRecord, bool, error) {
			return nil, false, nil
		},
		transferFn: func(context.Context, repository.TransferInput) (repository.TransferResult, error) {
			return repository.TransferResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewWalletGRPCServer() error = %v", err)
	}

	_, err = srv.GetTopUpSummary(context.Background(), &walletpb.GetTopUpSummaryRequest{
		UserId:   "user-001",
		Timezone: "invalid/timezone",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (%v)", status.Code(err), err)
	}
}

func TestListTopUps_Success(t *testing.T) {
	now := time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)
	srv, err := NewWalletGRPCServer(mockWalletStore{
		getBalanceFn:   func(context.Context, string) (int64, error) { return 0, nil },
		createWalletFn: func(context.Context, string, int64) (int64, error) { return 0, nil },
		topUpFn:        func(context.Context, string, int64) (int64, error) { return 0, nil },
		getTopUpSummaryFn: func(context.Context, string, string) (repository.TopUpSummary, error) {
			return repository.TopUpSummary{}, nil
		},
		listTopUpsFn: func(context.Context, string, *time.Time, *time.Time, int) ([]repository.TopUpRecord, bool, error) {
			return []repository.TopUpRecord{
				{
					TopUpID:           "topup-001",
					UserID:            "user-001",
					AmountCents:       5000,
					BalanceAfterCents: 12500050,
					CreatedAt:         now,
				},
			}, true, nil
		},
		transferFn: func(context.Context, repository.TransferInput) (repository.TransferResult, error) {
			return repository.TransferResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewWalletGRPCServer() error = %v", err)
	}

	resp, err := srv.ListTopUps(context.Background(), &walletpb.ListTopUpsRequest{
		UserId: "user-001",
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("ListTopUps() unexpected error = %v", err)
	}
	if !resp.GetHasMore() {
		t.Fatalf("expected has_more=true")
	}
	if len(resp.GetRecords()) != 1 {
		t.Fatalf("expected 1 topup record, got %d", len(resp.GetRecords()))
	}
	if resp.GetRecords()[0].GetTopupId() != "topup-001" {
		t.Fatalf("unexpected topup id %q", resp.GetRecords()[0].GetTopupId())
	}
}

func TestGetTopUpSummary_InvalidTimezone(t *testing.T) {
	srv, err := NewWalletGRPCServer(mockWalletStore{
		getBalanceFn:   func(context.Context, string) (int64, error) { return 0, nil },
		createWalletFn: func(context.Context, string, int64) (int64, error) { return 0, nil },
		topUpFn:        func(context.Context, string, int64) (int64, error) { return 0, nil },
		getTopUpSummaryFn: func(context.Context, string, string) (repository.TopUpSummary, error) {
			return repository.TopUpSummary{}, nil
		},
		listTopUpsFn: func(context.Context, string, *time.Time, *time.Time, int) ([]repository.TopUpRecord, bool, error) {
			return nil, false, nil
		},
		transferFn: func(context.Context, repository.TransferInput) (repository.TransferResult, error) {
			return repository.TransferResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewWalletGRPCServer() error = %v", err)
	}

	_, err = srv.GetTopUpSummary(context.Background(), &walletpb.GetTopUpSummaryRequest{
		UserId:   "user-001",
		Timezone: "invalid/timezone",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (%v)", status.Code(err), err)
	}
}

func TestListTopUps_Success(t *testing.T) {
	now := time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)
	srv, err := NewWalletGRPCServer(mockWalletStore{
		getBalanceFn:   func(context.Context, string) (int64, error) { return 0, nil },
		createWalletFn: func(context.Context, string, int64) (int64, error) { return 0, nil },
		topUpFn:        func(context.Context, string, int64) (int64, error) { return 0, nil },
		getTopUpSummaryFn: func(context.Context, string, string) (repository.TopUpSummary, error) {
			return repository.TopUpSummary{}, nil
		},
		listTopUpsFn: func(context.Context, string, *time.Time, *time.Time, int) ([]repository.TopUpRecord, bool, error) {
			return []repository.TopUpRecord{
				{
					TopUpID:           "topup-001",
					UserID:            "user-001",
					AmountCents:       5000,
					BalanceAfterCents: 12500050,
					CreatedAt:         now,
				},
			}, true, nil
		},
		transferFn: func(context.Context, repository.TransferInput) (repository.TransferResult, error) {
			return repository.TransferResult{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewWalletGRPCServer() error = %v", err)
	}

	resp, err := srv.ListTopUps(context.Background(), &walletpb.ListTopUpsRequest{
		UserId: "user-001",
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("ListTopUps() unexpected error = %v", err)
	}
	if !resp.GetHasMore() {
		t.Fatalf("expected has_more=true")
	}
	if len(resp.GetRecords()) != 1 {
		t.Fatalf("expected 1 topup record, got %d", len(resp.GetRecords()))
	}
	if resp.GetRecords()[0].GetTopupId() != "topup-001" {
		t.Fatalf("unexpected topup id %q", resp.GetRecords()[0].GetTopupId())
	}
}
