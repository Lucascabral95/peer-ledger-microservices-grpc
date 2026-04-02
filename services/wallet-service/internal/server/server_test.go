package server

import (
	"context"
	"errors"
	"testing"

	walletpb "github.com/peer-ledger/gen/wallet"
	"github.com/peer-ledger/services/wallet-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockWalletStore struct {
	getBalanceFn func(ctx context.Context, userID string) (int64, error)
	transferFn   func(ctx context.Context, input repository.TransferInput) (repository.TransferResult, error)
}

func (m mockWalletStore) GetBalance(ctx context.Context, userID string) (int64, error) {
	if m.getBalanceFn == nil {
		return 0, errors.New("getBalanceFn not configured")
	}
	return m.getBalanceFn(ctx, userID)
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
		transferFn: func(_ context.Context, input repository.TransferInput) (repository.TransferResult, error) {
			if input.AmountCents != 100032 {
				t.Fatalf("expected amount cents 100032, got %d", input.AmountCents)
			}

			return repository.TransferResult{
				TransactionID:      expectedTxID,
				SenderBalanceCents: 9899968,
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
}
