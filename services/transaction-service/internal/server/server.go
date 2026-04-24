package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	_ "time/tzdata"

	transactionpb "github.com/peer-ledger/gen/transaction"
	"github.com/peer-ledger/services/transaction-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TransactionStore interface {
	Record(ctx context.Context, input repository.RecordInput) error
	GetHistory(ctx context.Context, userID string) ([]repository.HistoryRecord, error)
	GetTransferSummary(ctx context.Context, userID, timezone string) (repository.TransferSummary, error)
	ListTransfers(ctx context.Context, userID string, direction repository.TransferDirection, from, to *time.Time, limit int) ([]repository.HistoryRecord, bool, error)
}

type TransactionGRPCServer struct {
	transactionpb.UnimplementedTransactionServiceServer
	store TransactionStore
}

func NewTransactionGRPCServer(store TransactionStore) (*TransactionGRPCServer, error) {
	if store == nil {
		return nil, fmt.Errorf("transaction store cannot be nil")
	}

	return &TransactionGRPCServer{store: store}, nil
}

func (s *TransactionGRPCServer) Record(ctx context.Context, req *transactionpb.RecordRequest) (*transactionpb.RecordResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	transactionID := strings.TrimSpace(req.GetTransactionId())
	senderID := strings.TrimSpace(req.GetSenderId())
	receiverID := strings.TrimSpace(req.GetReceiverId())
	idempotencyKey := strings.TrimSpace(req.GetIdempotencyKey())

	if transactionID == "" {
		return nil, status.Error(codes.InvalidArgument, "transaction_id is required")
	}
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
	if req.GetSenderBalanceAfter() < 0 || req.GetReceiverBalanceAfter() < 0 {
		return nil, status.Error(codes.InvalidArgument, "balance_after cannot be negative")
	}
	senderBalanceAfterCents := int64(math.Round(req.GetSenderBalanceAfter() * 100))
	receiverBalanceAfterCents := int64(math.Round(req.GetReceiverBalanceAfter() * 100))

	err := s.store.Record(ctx, repository.RecordInput{
		TransactionID:             transactionID,
		SenderID:                  senderID,
		ReceiverID:                receiverID,
		AmountCents:               amountCents,
		IdempotencyKey:            idempotencyKey,
		Status:                    "completed",
		SenderBalanceAfterCents:   senderBalanceAfterCents,
		ReceiverBalanceAfterCents: receiverBalanceAfterCents,
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInvalidRecordInput):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, repository.ErrIdempotencyPayloadMismatch):
			return nil, status.Error(codes.InvalidArgument, "idempotency key reused with different payload")
		case errors.Is(err, repository.ErrTransactionIDConflict):
			return nil, status.Error(codes.AlreadyExists, "transaction id conflict")
		default:
			return nil, status.Error(codes.Internal, "transaction record failed")
		}
	}

	return &transactionpb.RecordResponse{Success: true}, nil
}

func (s *TransactionGRPCServer) GetHistory(ctx context.Context, req *transactionpb.GetHistoryRequest) (*transactionpb.GetHistoryResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	userID := strings.TrimSpace(req.GetUserId())
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	records, err := s.store.GetHistory(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidRecordInput) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "transaction history read failed")
	}

	responseRecords := make([]*transactionpb.TransactionRecord, 0, len(records))
	for _, record := range records {
		responseRecords = append(responseRecords, &transactionpb.TransactionRecord{
			TransactionId:        record.TransactionID,
			SenderId:             record.SenderID,
			ReceiverId:           record.ReceiverID,
			Amount:               float64(record.AmountCents) / 100.0,
			Status:               record.Status,
			CreatedAt:            record.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
			SenderBalanceAfter:   float64(record.SenderBalanceAfterCents) / 100.0,
			ReceiverBalanceAfter: float64(record.ReceiverBalanceAfterCents) / 100.0,
		})
	}

	return &transactionpb.GetHistoryResponse{
		Records: responseRecords,
	}, nil
}

func (s *TransactionGRPCServer) GetTransferSummary(ctx context.Context, req *transactionpb.GetTransferSummaryRequest) (*transactionpb.GetTransferSummaryResponse, error) {
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

	summary, err := s.store.GetTransferSummary(ctx, userID, timezone)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidRecordInput) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "transaction summary read failed")
	}

	return &transactionpb.GetTransferSummaryResponse{
		UserId:              summary.UserID,
		SentTotal:           float64(summary.SentTotalCents) / 100.0,
		ReceivedTotal:       float64(summary.ReceivedTotalCents) / 100.0,
		SentCountTotal:      summary.SentCountTotal,
		ReceivedCountTotal:  summary.ReceivedCountTotal,
		SentCountToday:      summary.SentCountToday,
		ReceivedCountToday:  summary.ReceivedCountToday,
		SentAmountToday:     float64(summary.SentAmountTodayCents) / 100.0,
		ReceivedAmountToday: float64(summary.ReceivedAmountTodayCents) / 100.0,
	}, nil
}

func (s *TransactionGRPCServer) ListTransfers(ctx context.Context, req *transactionpb.ListTransfersRequest) (*transactionpb.ListTransfersResponse, error) {
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

	direction, err := mapTransferDirection(req.GetDirection())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	records, hasMore, err := s.store.ListTransfers(ctx, userID, direction, from, to, int(req.GetLimit()))
	if err != nil {
		if errors.Is(err, repository.ErrInvalidRecordInput) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "transaction transfer list failed")
	}

	responseRecords := make([]*transactionpb.TransactionRecord, 0, len(records))
	for _, record := range records {
		responseRecords = append(responseRecords, &transactionpb.TransactionRecord{
			TransactionId:        record.TransactionID,
			SenderId:             record.SenderID,
			ReceiverId:           record.ReceiverID,
			Amount:               float64(record.AmountCents) / 100.0,
			Status:               record.Status,
			CreatedAt:            record.CreatedAt.UTC().Format(time.RFC3339Nano),
			SenderBalanceAfter:   float64(record.SenderBalanceAfterCents) / 100.0,
			ReceiverBalanceAfter: float64(record.ReceiverBalanceAfterCents) / 100.0,
		})
	}

	return &transactionpb.ListTransfersResponse{
		Records: responseRecords,
		HasMore: hasMore,
	}, nil
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

func mapTransferDirection(direction transactionpb.TransferDirection) (repository.TransferDirection, error) {
	switch direction {
	case transactionpb.TransferDirection_TRANSFER_DIRECTION_ALL:
		return repository.TransferDirectionAll, nil
	case transactionpb.TransferDirection_TRANSFER_DIRECTION_SENT:
		return repository.TransferDirectionSent, nil
	case transactionpb.TransferDirection_TRANSFER_DIRECTION_RECEIVED:
		return repository.TransferDirectionReceived, nil
	default:
		return repository.TransferDirectionAll, errors.New("direction is invalid")
	}
}
