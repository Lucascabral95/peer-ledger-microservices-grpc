package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
	_ "time/tzdata"

	walletpb "github.com/peer-ledger/gen/wallet"
	"github.com/peer-ledger/services/wallet-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WalletStore interface {
	GetBalance(ctx context.Context, userID string) (int64, error)
	CreateWallet(ctx context.Context, userID string, initialBalanceCents int64) (int64, error)
	TopUp(ctx context.Context, userID string, amountCents int64) (int64, error)
	GetTopUpSummary(ctx context.Context, userID, timezone string) (repository.TopUpSummary, error)
	ListTopUps(ctx context.Context, userID string, from, to *time.Time, limit int) ([]repository.TopUpRecord, bool, error)
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

func (s *WalletGRPCServer) CreateWallet(ctx context.Context, req *walletpb.CreateWalletRequest) (*walletpb.CreateWalletResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	balanceCents, err := s.store.CreateWallet(ctx, userID, 0)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInvalidWalletInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Error(codes.Internal, "wallet create failed")
		}
	}

	return &walletpb.CreateWalletResponse{
		UserId:  userID,
		Balance: centsToAmount(balanceCents),
	}, nil
}

func (s *WalletGRPCServer) TopUp(ctx context.Context, req *walletpb.TopUpRequest) (*walletpb.TopUpResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GetAmount() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than zero")
	}

	amountCents := int64(math.Round(req.GetAmount() * 100))
	if amountCents <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount is invalid after conversion to cents")
	}

	balanceCents, err := s.store.TopUp(ctx, userID, amountCents)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInvalidWalletInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, repository.ErrWalletNotFound):
			return nil, status.Error(codes.NotFound, "wallet not found")
		default:
			return nil, status.Error(codes.Internal, "wallet topup failed")
		}
	}

	return &walletpb.TopUpResponse{
		UserId:  userID,
		Balance: centsToAmount(balanceCents),
	}, nil
}

func (s *WalletGRPCServer) GetTopUpSummary(ctx context.Context, req *walletpb.GetTopUpSummaryRequest) (*walletpb.GetTopUpSummaryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	timezone := strings.TrimSpace(req.GetTimezone())
	if timezone == "" {
		return nil, status.Error(codes.InvalidArgument, "timezone is required")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, status.Error(codes.InvalidArgument, "timezone is invalid")
	}

	summary, err := s.store.GetTopUpSummary(ctx, userID, timezone)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInvalidWalletInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			log.Printf("wallet topup summary failed for user_id=%s: %v", userID, err)
			return nil, status.Error(codes.Internal, "wallet topup summary failed")
		}
	}

	return &walletpb.GetTopUpSummaryResponse{
		UserId:           summary.UserID,
		TopupCountTotal:  summary.TopUpCountTotal,
		TopupAmountTotal: centsToAmount(summary.TopUpAmountTotalCents),
		TopupCountToday:  summary.TopUpCountToday,
		TopupAmountToday: centsToAmount(summary.TopUpAmountTodayCents),
	}, nil
}

func (s *WalletGRPCServer) ListTopUps(ctx context.Context, req *walletpb.ListTopUpsRequest) (*walletpb.ListTopUpsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GetLimit() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "limit must be greater than zero")
	}
	if req.GetLimit() > 5000 {
		return nil, status.Error(codes.InvalidArgument, "limit must be less than or equal to 5000")
	}

	from, err := parseOptionalTimestamp(req.GetFromCreatedAt())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "from_created_at must be RFC3339")
	}
	to, err := parseOptionalTimestamp(req.GetToCreatedAt())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "to_created_at must be RFC3339")
	}

	records, hasMore, err := s.store.ListTopUps(ctx, userID, from, to, int(req.GetLimit()))
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInvalidWalletInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			log.Printf("wallet topup list failed for user_id=%s: %v", userID, err)
			return nil, status.Error(codes.Internal, "wallet topup list failed")
		}
	}

	responseRecords := make([]*walletpb.TopUpRecord, 0, len(records))
	for _, record := range records {
		responseRecords = append(responseRecords, &walletpb.TopUpRecord{
			TopupId:      record.TopUpID,
			UserId:       record.UserID,
			Amount:       centsToAmount(record.AmountCents),
			BalanceAfter: centsToAmount(record.BalanceAfterCents),
			CreatedAt:    record.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}

	return &walletpb.ListTopUpsResponse{
		Records: responseRecords,
		HasMore: hasMore,
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

func parseOptionalTimestamp(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}
