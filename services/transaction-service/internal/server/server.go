package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	transactionpb "github.com/peer-ledger/gen/transaction"
	"github.com/peer-ledger/services/transaction-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TransactionStore interface {
	Record(ctx context.Context, input repository.RecordInput) error
	GetHistory(ctx context.Context, userID string) ([]repository.HistoryRecord, error)
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

	err := s.store.Record(ctx, repository.RecordInput{
		TransactionID:  transactionID,
		SenderID:       senderID,
		ReceiverID:     receiverID,
		AmountCents:    amountCents,
		IdempotencyKey: idempotencyKey,
		Status:         "completed",
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
			TransactionId: record.TransactionID,
			SenderId:      record.SenderID,
			ReceiverId:    record.ReceiverID,
			Amount:        float64(record.AmountCents) / 100.0,
			Status:        record.Status,
			CreatedAt:     record.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
	}

	return &transactionpb.GetHistoryResponse{
		Records: responseRecords,
	}, nil
}
