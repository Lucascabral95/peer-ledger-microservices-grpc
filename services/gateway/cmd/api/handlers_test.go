package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	fraudpb "github.com/peer-ledger/gen/fraud"
	userpb "github.com/peer-ledger/gen/user"
	walletpb "github.com/peer-ledger/gen/wallet"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockUserClient struct {
	getUserFn    func(ctx context.Context, in *userpb.GetUserRequest) (*userpb.GetUserResponse, error)
	userExistsFn func(ctx context.Context, in *userpb.UserExistsRequest) (*userpb.UserExistsResponse, error)
}

func (m mockUserClient) GetUser(ctx context.Context, in *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.GetUserResponse, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, in)
	}
	return &userpb.GetUserResponse{UserId: in.GetId()}, nil
}

func (m mockUserClient) UserExists(ctx context.Context, in *userpb.UserExistsRequest, _ ...grpc.CallOption) (*userpb.UserExistsResponse, error) {
	if m.userExistsFn != nil {
		return m.userExistsFn(ctx, in)
	}
	return &userpb.UserExistsResponse{Exists: true}, nil
}

type mockFraudClient struct {
	evaluateFn func(ctx context.Context, in *fraudpb.EvaluateRequest) (*fraudpb.EvaluateResponse, error)
}

func (m mockFraudClient) EvaluateTransfer(ctx context.Context, in *fraudpb.EvaluateRequest, _ ...grpc.CallOption) (*fraudpb.EvaluateResponse, error) {
	if m.evaluateFn != nil {
		return m.evaluateFn(ctx, in)
	}
	return &fraudpb.EvaluateResponse{Allowed: true, RuleCode: "OK"}, nil
}

type mockWalletClient struct {
	getBalanceFn func(ctx context.Context, in *walletpb.GetBalanceRequest) (*walletpb.GetBalanceResponse, error)
	transferFn   func(ctx context.Context, in *walletpb.TransferRequest) (*walletpb.TransferResponse, error)
}

func (m mockWalletClient) GetBalance(ctx context.Context, in *walletpb.GetBalanceRequest, _ ...grpc.CallOption) (*walletpb.GetBalanceResponse, error) {
	if m.getBalanceFn != nil {
		return m.getBalanceFn(ctx, in)
	}
	return &walletpb.GetBalanceResponse{UserId: in.GetUserId(), Balance: 0}, nil
}

func (m mockWalletClient) Transfer(ctx context.Context, in *walletpb.TransferRequest, _ ...grpc.CallOption) (*walletpb.TransferResponse, error) {
	if m.transferFn != nil {
		return m.transferFn(ctx, in)
	}
	return &walletpb.TransferResponse{TransactionId: "tx-default", SenderBalance: 0}, nil
}

func TestCreateTransfer_FraudDenied(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			userExistsFn: func(context.Context, *userpb.UserExistsRequest) (*userpb.UserExistsResponse, error) {
				return &userpb.UserExistsResponse{Exists: true}, nil
			},
		},
		fraudClient: mockFraudClient{
			evaluateFn: func(context.Context, *fraudpb.EvaluateRequest) (*fraudpb.EvaluateResponse, error) {
				return &fraudpb.EvaluateResponse{
					Allowed:  false,
					Reason:   "cooldown active for sender-receiver pair",
					RuleCode: "COOLDOWN_PAIR",
				}, nil
			},
		},
		walletClient: mockWalletClient{},
	}

	rr := httptest.NewRecorder()
	req := newTransferRequest(t, `{"sender_id":"user-001","receiver_id":"user-002","amount":1000.01,"idempotency_key":"k-1"}`)

	app.CreateTransfer(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestCreateTransfer_WalletInsufficientFunds(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			userExistsFn: func(context.Context, *userpb.UserExistsRequest) (*userpb.UserExistsResponse, error) {
				return &userpb.UserExistsResponse{Exists: true}, nil
			},
		},
		fraudClient: mockFraudClient{
			evaluateFn: func(context.Context, *fraudpb.EvaluateRequest) (*fraudpb.EvaluateResponse, error) {
				return &fraudpb.EvaluateResponse{Allowed: true, RuleCode: "OK"}, nil
			},
		},
		walletClient: mockWalletClient{
			transferFn: func(context.Context, *walletpb.TransferRequest) (*walletpb.TransferResponse, error) {
				return nil, status.Error(codes.FailedPrecondition, "insufficient funds")
			},
		},
	}

	rr := httptest.NewRecorder()
	req := newTransferRequest(t, `{"sender_id":"user-001","receiver_id":"user-002","amount":1000.01,"idempotency_key":"k-2"}`)

	app.CreateTransfer(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rr.Code)
	}
}

func TestCreateTransfer_UserServiceUnavailable(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			userExistsFn: func(context.Context, *userpb.UserExistsRequest) (*userpb.UserExistsResponse, error) {
				return nil, status.Error(codes.Unavailable, "user service down")
			},
		},
		fraudClient:  mockFraudClient{},
		walletClient: mockWalletClient{},
	}

	rr := httptest.NewRecorder()
	req := newTransferRequest(t, `{"sender_id":"user-001","receiver_id":"user-002","amount":1000.01,"idempotency_key":"k-3"}`)

	app.CreateTransfer(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestCreateTransfer_Success(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			userExistsFn: func(context.Context, *userpb.UserExistsRequest) (*userpb.UserExistsResponse, error) {
				return &userpb.UserExistsResponse{Exists: true}, nil
			},
		},
		fraudClient: mockFraudClient{
			evaluateFn: func(context.Context, *fraudpb.EvaluateRequest) (*fraudpb.EvaluateResponse, error) {
				return &fraudpb.EvaluateResponse{Allowed: true, RuleCode: "OK"}, nil
			},
		},
		walletClient: mockWalletClient{
			transferFn: func(context.Context, *walletpb.TransferRequest) (*walletpb.TransferResponse, error) {
				return &walletpb.TransferResponse{
					TransactionId: "tx-200",
					SenderBalance: 98999.99,
				}, nil
			},
		},
	}

	rr := httptest.NewRecorder()
	req := newTransferRequest(t, `{"sender_id":"user-001","receiver_id":"user-002","amount":1000.01,"idempotency_key":"k-4"}`)

	app.CreateTransfer(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var payload struct {
		Error   bool           `json:"error"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Error {
		t.Fatalf("expected error=false, got true")
	}
	if payload.Data["transaction_id"] != "tx-200" {
		t.Fatalf("expected transaction_id tx-200, got %v", payload.Data["transaction_id"])
	}
}

func newTransferRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
