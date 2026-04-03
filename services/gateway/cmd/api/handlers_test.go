package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi"
	fraudpb "github.com/peer-ledger/gen/fraud"
	transactionpb "github.com/peer-ledger/gen/transaction"
	userpb "github.com/peer-ledger/gen/user"
	walletpb "github.com/peer-ledger/gen/wallet"
	"github.com/peer-ledger/internal/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockUserClient struct {
	getUserFn    func(ctx context.Context, in *userpb.GetUserRequest) (*userpb.GetUserResponse, error)
	userExistsFn func(ctx context.Context, in *userpb.UserExistsRequest) (*userpb.UserExistsResponse, error)
	registerFn   func(ctx context.Context, in *userpb.RegisterRequest) (*userpb.RegisterResponse, error)
	loginFn      func(ctx context.Context, in *userpb.LoginRequest) (*userpb.LoginResponse, error)
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

func (m mockUserClient) Register(ctx context.Context, in *userpb.RegisterRequest, _ ...grpc.CallOption) (*userpb.RegisterResponse, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, in)
	}
	return &userpb.RegisterResponse{UserId: "user-new", Name: in.GetName(), Email: strings.ToLower(in.GetEmail())}, nil
}

func (m mockUserClient) Login(ctx context.Context, in *userpb.LoginRequest, _ ...grpc.CallOption) (*userpb.LoginResponse, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, in)
	}
	return &userpb.LoginResponse{UserId: "user-001", Name: "Lucas", Email: strings.ToLower(in.GetEmail())}, nil
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

type mockTransactionClient struct {
	recordFn     func(ctx context.Context, in *transactionpb.RecordRequest) (*transactionpb.RecordResponse, error)
	getHistoryFn func(ctx context.Context, in *transactionpb.GetHistoryRequest) (*transactionpb.GetHistoryResponse, error)
}

func (m mockTransactionClient) Record(ctx context.Context, in *transactionpb.RecordRequest, _ ...grpc.CallOption) (*transactionpb.RecordResponse, error) {
	if m.recordFn != nil {
		return m.recordFn(ctx, in)
	}
	return &transactionpb.RecordResponse{Success: true}, nil
}

func (m mockTransactionClient) GetHistory(ctx context.Context, in *transactionpb.GetHistoryRequest, _ ...grpc.CallOption) (*transactionpb.GetHistoryResponse, error) {
	if m.getHistoryFn != nil {
		return m.getHistoryFn(ctx, in)
	}
	return &transactionpb.GetHistoryResponse{}, nil
}

func testTokenManager(t *testing.T) *security.JWTManager {
	t.Helper()

	manager, err := security.NewJWTManager(strings.Repeat("s", 32), "peer-ledger-gateway", time.Hour, func() time.Time {
		return time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("NewJWTManager() error: %v", err)
	}
	return manager
}

func authContextRequest(t *testing.T, body string, subject string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(context.WithValue(req.Context(), userClaimsContextKey, &security.JWTClaims{
		Subject: subject,
		Name:    "Lucas",
		Email:   "lucas@mail.com",
	}))
}

func TestRegister_Success(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			registerFn: func(context.Context, *userpb.RegisterRequest) (*userpb.RegisterResponse, error) {
				return &userpb.RegisterResponse{
					UserId: "user-010",
					Name:   "Lucas",
					Email:  "lucas@mail.com",
				}, nil
			},
		},
		tokenManager: testTokenManager(t),
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(`{"name":"Lucas","email":"lucas@mail.com","password":"Password123!"}`))
	req.Header.Set("Content-Type", "application/json")

	app.Register(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"token\"") {
		t.Fatalf("expected token in response")
	}
}

func TestLogin_Success(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			loginFn: func(context.Context, *userpb.LoginRequest) (*userpb.LoginResponse, error) {
				return &userpb.LoginResponse{
					UserId: "user-001",
					Name:   "Lucas",
					Email:  "lucas@mail.com",
				}, nil
			},
		},
		tokenManager: testTokenManager(t),
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"lucas@mail.com","password":"Password123!"}`))
	req.Header.Set("Content-Type", "application/json")

	app.Login(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"token\"") {
		t.Fatalf("expected token in response")
	}
}

func TestCreateTransfer_Unauthenticated(t *testing.T) {
	app := Config{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewBufferString(`{"receiver_id":"user-002","amount":1000.01,"idempotency_key":"k-0"}`))
	req.Header.Set("Content-Type", "application/json")

	app.CreateTransfer(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
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
		walletClient:      mockWalletClient{},
		transactionClient: mockTransactionClient{},
	}

	rr := httptest.NewRecorder()
	req := authContextRequest(t, `{"receiver_id":"user-002","amount":1000.01,"idempotency_key":"k-1"}`, "user-001")

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
		transactionClient: mockTransactionClient{},
	}

	rr := httptest.NewRecorder()
	req := authContextRequest(t, `{"receiver_id":"user-002","amount":1000.01,"idempotency_key":"k-2"}`, "user-001")

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
		fraudClient:       mockFraudClient{},
		walletClient:      mockWalletClient{},
		transactionClient: mockTransactionClient{},
	}

	rr := httptest.NewRecorder()
	req := authContextRequest(t, `{"receiver_id":"user-002","amount":1000.01,"idempotency_key":"k-3"}`, "user-001")

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
			transferFn: func(_ context.Context, in *walletpb.TransferRequest) (*walletpb.TransferResponse, error) {
				if in.GetSenderId() != "user-001" {
					t.Fatalf("expected sender from auth context, got %s", in.GetSenderId())
				}
				return &walletpb.TransferResponse{
					TransactionId: "tx-200",
					SenderBalance: 98999.99,
				}, nil
			},
		},
		transactionClient: mockTransactionClient{
			recordFn: func(context.Context, *transactionpb.RecordRequest) (*transactionpb.RecordResponse, error) {
				return &transactionpb.RecordResponse{Success: true}, nil
			},
		},
	}

	rr := httptest.NewRecorder()
	req := authContextRequest(t, `{"receiver_id":"user-002","amount":1000.01,"idempotency_key":"k-4"}`, "user-001")

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
	if payload.Data["transaction_id"] != "tx-200" {
		t.Fatalf("expected transaction_id tx-200, got %v", payload.Data["transaction_id"])
	}
}

func TestCreateTransfer_TransactionRecordFailsAfterWalletSuccess(t *testing.T) {
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
					TransactionId: "tx-wallet-ok",
					SenderBalance: 900,
				}, nil
			},
		},
		transactionClient: mockTransactionClient{
			recordFn: func(context.Context, *transactionpb.RecordRequest) (*transactionpb.RecordResponse, error) {
				return nil, status.Error(codes.Unavailable, "transaction down")
			},
		},
	}

	rr := httptest.NewRecorder()
	req := authContextRequest(t, `{"receiver_id":"user-002","amount":10.00,"idempotency_key":"k-record-fail"}`, "user-001")

	app.CreateTransfer(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

func TestHealth(t *testing.T) {
	app := Config{}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	app.Health(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestGetHistory_Success(t *testing.T) {
	app := Config{
		transactionClient: mockTransactionClient{
			getHistoryFn: func(context.Context, *transactionpb.GetHistoryRequest) (*transactionpb.GetHistoryResponse, error) {
				return &transactionpb.GetHistoryResponse{
					Records: []*transactionpb.TransactionRecord{{
						TransactionId: "tx-1",
						SenderId:      "user-001",
						ReceiverId:    "user-002",
						Amount:        10,
						Status:        "completed",
						CreatedAt:     "2026-04-01T00:00:00Z",
					}},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/history/user-001", nil)
	req = req.WithContext(context.WithValue(req.Context(), userClaimsContextKey, &security.JWTClaims{Subject: "user-001"}))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userID", "user-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	app.GetHistory(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestGetHistory_ForbiddenForDifferentUser(t *testing.T) {
	app := Config{}

	req := httptest.NewRequest(http.MethodGet, "/history/user-002", nil)
	req = req.WithContext(context.WithValue(req.Context(), userClaimsContextKey, &security.JWTClaims{Subject: "user-001"}))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userID", "user-002")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	app.GetHistory(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rr.Code)
	}
}

func TestGetUser_Success(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			getUserFn: func(context.Context, *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
				return &userpb.GetUserResponse{
					UserId: "user-001",
					Name:   "Lucas",
					Email:  "lucas@mail.com",
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/users/user-001", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userID", "user-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	app.GetUser(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestGetUser_GrpcError(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			getUserFn: func(context.Context, *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
				return nil, status.Error(codes.NotFound, "user not found")
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/users/user-404", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userID", "user-404")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	app.GetUser(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}
}

func TestUserExists_HandlerSuccess(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			userExistsFn: func(context.Context, *userpb.UserExistsRequest) (*userpb.UserExistsResponse, error) {
				return &userpb.UserExistsResponse{Exists: true}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/users/user-001/exists", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("userID", "user-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	app.UserExists(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestEnsureUserExists_NotFound(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			userExistsFn: func(context.Context, *userpb.UserExistsRequest) (*userpb.UserExistsResponse, error) {
				return &userpb.UserExistsResponse{Exists: false}, nil
			},
		},
	}

	statusCode, err := app.ensureUserExists("user-404")
	if err == nil {
		t.Fatalf("expected error")
	}
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", statusCode)
	}
}

func TestEvaluateFraud_Success(t *testing.T) {
	app := Config{
		fraudClient: mockFraudClient{
			evaluateFn: func(context.Context, *fraudpb.EvaluateRequest) (*fraudpb.EvaluateResponse, error) {
				return &fraudpb.EvaluateResponse{Allowed: true, RuleCode: "OK"}, nil
			},
		},
	}

	resp, statusCode, err := app.evaluateFraud(transferRequest{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		Amount:         10,
		IdempotencyKey: "k1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if statusCode != 0 || !resp.GetAllowed() {
		t.Fatalf("unexpected response: status=%d allowed=%v", statusCode, resp.GetAllowed())
	}
}

func TestExecuteWalletTransfer_Success(t *testing.T) {
	app := Config{
		walletClient: mockWalletClient{
			transferFn: func(context.Context, *walletpb.TransferRequest) (*walletpb.TransferResponse, error) {
				return &walletpb.TransferResponse{
					TransactionId: "tx-wallet-ok",
					SenderBalance: 100,
				}, nil
			},
		},
	}

	resp, statusCode, err := app.executeWalletTransfer(transferRequest{
		SenderID:       "user-001",
		ReceiverID:     "user-002",
		Amount:         10,
		IdempotencyKey: "k1",
	})
	if err != nil || statusCode != 0 {
		t.Fatalf("unexpected result: status=%d err=%v", statusCode, err)
	}
	if resp.GetTransactionId() != "tx-wallet-ok" {
		t.Fatalf("unexpected transaction id: %s", resp.GetTransactionId())
	}
}
