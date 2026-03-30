package server

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	fraudpb "github.com/peer-ledger/gen/fraud"
	"github.com/peer-ledger/services/fraud-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ClockFunc func() time.Time

type FraudEvaluator interface {
	Evaluate(input repository.EvaluateInput) (repository.Decision, error)
}

type FraudGRPCServer struct {
	fraudpb.UnimplementedFraudServiceServer
	evaluator FraudEvaluator
	clock     ClockFunc
}

func NewFraudGRPCServer(evaluator FraudEvaluator, clock ClockFunc) (*FraudGRPCServer, error) {
	if evaluator == nil {
		return nil, fmt.Errorf("fraud evaluator cannot be nil")
	}
	if clock == nil {
		clock = time.Now
	}

	return &FraudGRPCServer{
		evaluator: evaluator,
		clock:     clock,
	}, nil
}

func (s *FraudGRPCServer) EvaluateTransfer(ctx context.Context, req *fraudpb.EvaluateRequest) (*fraudpb.EvaluateResponse, error) {
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

	decision, err := s.evaluator.Evaluate(repository.EvaluateInput{
		SenderID:       senderID,
		ReceiverID:     receiverID,
		AmountCents:    amountCents,
		IdempotencyKey: idempotencyKey,
		Now:            s.clock(),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "fraud evaluation failed")
	}

	return &fraudpb.EvaluateResponse{
		Allowed:  decision.Allowed,
		Reason:   decision.Reason,
		RuleCode: decision.RuleCode,
	}, nil
}
