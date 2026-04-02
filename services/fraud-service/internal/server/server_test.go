package server

import (
	"context"
	"errors"
	"testing"
	"time"

	fraudpb "github.com/peer-ledger/gen/fraud"
	"github.com/peer-ledger/services/fraud-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockEvaluator struct {
	evaluateFn func(input repository.EvaluateInput) (repository.Decision, error)
}

func (m mockEvaluator) Evaluate(input repository.EvaluateInput) (repository.Decision, error) {
	return m.evaluateFn(input)
}

func TestNewFraudGRPCServer_NilEvaluator(t *testing.T) {
	_, err := NewFraudGRPCServer(nil, nil)
	if err == nil {
		t.Fatalf("expected error for nil evaluator")
	}
}

func TestEvaluateTransfer_InvalidRequest(t *testing.T) {
	srv, err := NewFraudGRPCServer(mockEvaluator{
		evaluateFn: func(input repository.EvaluateInput) (repository.Decision, error) {
			return repository.Decision{}, nil
		},
	}, func() time.Time { return time.Unix(0, 0) })
	if err != nil {
		t.Fatalf("NewFraudGRPCServer() error: %v", err)
	}

	_, err = srv.EvaluateTransfer(context.Background(), &fraudpb.EvaluateRequest{
		SenderId:       "user-001",
		ReceiverId:     "user-001",
		Amount:         0,
		IdempotencyKey: "",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (%v)", status.Code(err), err)
	}
}

func TestEvaluateTransfer_InternalError(t *testing.T) {
	srv, err := NewFraudGRPCServer(mockEvaluator{
		evaluateFn: func(input repository.EvaluateInput) (repository.Decision, error) {
			return repository.Decision{}, errors.New("boom")
		},
	}, func() time.Time { return time.Unix(123, 0).UTC() })
	if err != nil {
		t.Fatalf("NewFraudGRPCServer() error: %v", err)
	}

	_, err = srv.EvaluateTransfer(context.Background(), &fraudpb.EvaluateRequest{
		SenderId:       "user-001",
		ReceiverId:     "user-002",
		Amount:         10.25,
		IdempotencyKey: "k1",
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v (%v)", status.Code(err), err)
	}
}

func TestEvaluateTransfer_Success(t *testing.T) {
	expectedNow := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	srv, err := NewFraudGRPCServer(mockEvaluator{
		evaluateFn: func(input repository.EvaluateInput) (repository.Decision, error) {
			if input.AmountCents != 1025 {
				t.Fatalf("expected 1025 cents, got %d", input.AmountCents)
			}
			if !input.Now.Equal(expectedNow) {
				t.Fatalf("unexpected Now in evaluator: %v", input.Now)
			}
			return repository.Decision{
				Allowed:  false,
				Reason:   "transfer velocity limit exceeded",
				RuleCode: repository.RuleCodeLimitVelocity,
			}, nil
		},
	}, func() time.Time { return expectedNow })
	if err != nil {
		t.Fatalf("NewFraudGRPCServer() error: %v", err)
	}

	resp, err := srv.EvaluateTransfer(context.Background(), &fraudpb.EvaluateRequest{
		SenderId:       "user-001",
		ReceiverId:     "user-002",
		Amount:         10.25,
		IdempotencyKey: "k-success",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetRuleCode() != repository.RuleCodeLimitVelocity {
		t.Fatalf("expected LIMIT_VELOCITY, got %s", resp.GetRuleCode())
	}
	if resp.GetAllowed() {
		t.Fatalf("expected allowed=false")
	}
}
