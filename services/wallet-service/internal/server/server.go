package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	walletpb "github.com/peer-ledger/gen/wallet"
	"github.com/peer-ledger/services/wallet-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WalletStore interface {
	GetBalance(ctx context.Context, userID string) (int64, error)
	Transfer(ctx context.Context, input repository.TransferInput) (repository.TransferResult, error)
}

type WalletGRPCServer struct {
	walletpb.UnimplementedWalletServiceServer
	store WalletStore
}

func NewWalletGRPCServer(store WalletStore) (*WalletGRPCServer, error) {
	if store == nil {
		return nil, fmt.Errorf("wallet store cannot be nil")
	}

	return &WalletGRPCServer{
		store: store,
	}, nil
}

func (s *WalletGRPCServer) GetBalance(ctx context.Context, req *walletpb.GetBalanceRequest) (*walletpb.GetBalanceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	balanceCents, err := s.store.GetBalance(ctx, userID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrWalletNotFound):
			return nil, status.Error(codes.NotFound, "wallet not found")
		default:
			return nil, status.Error(codes.Internal, "wallet read failed")
		}
	}

	return &walletpb.GetBalanceResponse{
		UserId:  userID,
		Balance: centsToAmount(balanceCents),
	}, nil
}

func (s *WalletGRPCServer) Transfer(ctx context.Context, req *walletpb.TransferRequest) (*walletpb.TransferResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	senderID := strings.TrimSpace(req.GetSenderId())
	receiverID := strings.TrimSpace(req.GetReceiverId())
	idempotencyKey := strings.TrimSpace(req.GetIdempotencyKey())

	if senderID == "" || receiverID == "" {
		return nil, status.Error(codes.InvalidArgument, "sender_id and receiver_id are required")
	}
	if senderID == receiverID {
		return nil, status.Error(codes.InvalidArgument, "sender_id and receiver_id must be different")
	}
	if idempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}
	if req.GetAmount() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than zero")
	}

	amountCents := int64(math.Round(req.GetAmount() * 100))
	if amountCents <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount is invalid after conversion to cents")
	}

	result, err := s.store.Transfer(ctx, repository.TransferInput{
		SenderID:       senderID,
		ReceiverID:     receiverID,
		AmountCents:    amountCents,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInvalidTransferInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, repository.ErrIdempotencyPayloadMismatch):
			return nil, status.Error(codes.InvalidArgument, "idempotency key reused with different payload")
		case errors.Is(err, repository.ErrInsufficientFunds):
			return nil, status.Error(codes.FailedPrecondition, "insufficient funds")
		case errors.Is(err, repository.ErrWalletNotFound):
			return nil, status.Error(codes.NotFound, "wallet not found")
		default:
			return nil, status.Error(codes.Internal, "wallet transfer failed")
		}
	}

	return &walletpb.TransferResponse{
		TransactionId: result.TransactionID,
		SenderBalance: centsToAmount(result.SenderBalanceCents),
	}, nil
}

func centsToAmount(cents int64) float64 {
	return float64(cents) / 100.0
}
