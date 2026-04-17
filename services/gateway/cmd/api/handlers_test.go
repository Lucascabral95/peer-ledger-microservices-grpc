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
	deleteUserFn func(ctx context.Context, in *userpb.DeleteUserRequest) (*userpb.DeleteUserResponse, error)
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

func (m mockUserClient) DeleteUser(ctx context.Context, in *userpb.DeleteUserRequest, _ ...grpc.CallOption) (*userpb.DeleteUserResponse, error) {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, in)
	}
	return &userpb.DeleteUserResponse{Deleted: true}, nil
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
	getBalanceFn      func(ctx context.Context, in *walletpb.GetBalanceRequest) (*walletpb.GetBalanceResponse, error)
	createFn          func(ctx context.Context, in *walletpb.CreateWalletRequest) (*walletpb.CreateWalletResponse, error)
	topUpFn           func(ctx context.Context, in *walletpb.TopUpRequest) (*walletpb.TopUpResponse, error)
	getTopUpSummaryFn func(ctx context.Context, in *walletpb.GetTopUpSummaryRequest) (*walletpb.GetTopUpSummaryResponse, error)
	listTopUpsFn      func(ctx context.Context, in *walletpb.ListTopUpsRequest) (*walletpb.ListTopUpsResponse, error)
	transferFn        func(ctx context.Context, in *walletpb.TransferRequest) (*walletpb.TransferResponse, error)
}

func (m mockWalletClient) GetBalance(ctx context.Context, in *walletpb.GetBalanceRequest, _ ...grpc.CallOption) (*walletpb.GetBalanceResponse, error) {
	if m.getBalanceFn != nil {
		return m.getBalanceFn(ctx, in)
	}
	return &walletpb.GetBalanceResponse{UserId: in.GetUserId(), Balance: 0}, nil
}

func (m mockWalletClient) CreateWallet(ctx context.Context, in *walletpb.CreateWalletRequest, _ ...grpc.CallOption) (*walletpb.CreateWalletResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, in)
	}
	return &walletpb.CreateWalletResponse{UserId: in.GetUserId(), Balance: 0}, nil
}

func (m mockWalletClient) TopUp(ctx context.Context, in *walletpb.TopUpRequest, _ ...grpc.CallOption) (*walletpb.TopUpResponse, error) {
	if m.topUpFn != nil {
		return m.topUpFn(ctx, in)
	}
	return &walletpb.TopUpResponse{UserId: in.GetUserId(), Balance: in.GetAmount()}, nil
}

func (m mockWalletClient) GetTopUpSummary(ctx context.Context, in *walletpb.GetTopUpSummaryRequest, _ ...grpc.CallOption) (*walletpb.GetTopUpSummaryResponse, error) {
	if m.getTopUpSummaryFn != nil {
		return m.getTopUpSummaryFn(ctx, in)
	}
	return &walletpb.GetTopUpSummaryResponse{UserId: in.GetUserId()}, nil
}

func (m mockWalletClient) ListTopUps(ctx context.Context, in *walletpb.ListTopUpsRequest, _ ...grpc.CallOption) (*walletpb.ListTopUpsResponse, error) {
	if m.listTopUpsFn != nil {
		return m.listTopUpsFn(ctx, in)
	}
	return &walletpb.ListTopUpsResponse{}, nil
}

func (m mockWalletClient) Transfer(ctx context.Context, in *walletpb.TransferRequest, _ ...grpc.CallOption) (*walletpb.TransferResponse, error) {
	if m.transferFn != nil {
		return m.transferFn(ctx, in)
	}
	return &walletpb.TransferResponse{TransactionId: "tx-default", SenderBalance: 0}, nil
}

type mockTransactionClient struct {
	recordFn             func(ctx context.Context, in *transactionpb.RecordRequest) (*transactionpb.RecordResponse, error)
	getHistoryFn         func(ctx context.Context, in *transactionpb.GetHistoryRequest) (*transactionpb.GetHistoryResponse, error)
	getTransferSummaryFn func(ctx context.Context, in *transactionpb.GetTransferSummaryRequest) (*transactionpb.GetTransferSummaryResponse, error)
	listTransfersFn      func(ctx context.Context, in *transactionpb.ListTransfersRequest) (*transactionpb.ListTransfersResponse, error)
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

func (m mockTransactionClient) GetTransferSummary(ctx context.Context, in *transactionpb.GetTransferSummaryRequest, _ ...grpc.CallOption) (*transactionpb.GetTransferSummaryResponse, error) {
	if m.getTransferSummaryFn != nil {
		return m.getTransferSummaryFn(ctx, in)
	}
	return &transactionpb.GetTransferSummaryResponse{UserId: in.GetUserId()}, nil
}

func (m mockTransactionClient) ListTransfers(ctx context.Context, in *transactionpb.ListTransfersRequest, _ ...grpc.CallOption) (*transactionpb.ListTransfersResponse, error) {
	if m.listTransfersFn != nil {
		return m.listTransfersFn(ctx, in)
	}
	return &transactionpb.ListTransfersResponse{}, nil
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

func testRefreshTokenManager(t *testing.T) *security.JWTManager {
	t.Helper()

	manager, err := security.NewTypedJWTManager(strings.Repeat("s", 32), "peer-ledger-gateway", 7*24*time.Hour, "refresh", func() time.Time {
		return time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("NewTypedJWTManager() error: %v", err)
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
		walletClient: mockWalletClient{
			createFn: func(context.Context, *walletpb.CreateWalletRequest) (*walletpb.CreateWalletResponse, error) {
				return &walletpb.CreateWalletResponse{UserId: "user-010", Balance: 0}, nil
			},
		},
		tokenManager:   testTokenManager(t),
		refreshManager: testRefreshTokenManager(t),
		accessTokenTTL: time.Hour,
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
	if !strings.Contains(rr.Body.String(), "\"refresh_token\"") {
		t.Fatalf("expected refresh_token in response")
	}
}

func TestRegister_WalletProvisioningFailure_RollsBackUser(t *testing.T) {
	rolledBack := false

	app := Config{
		userClient: mockUserClient{
			registerFn: func(context.Context, *userpb.RegisterRequest) (*userpb.RegisterResponse, error) {
				return &userpb.RegisterResponse{
					UserId: "user-rollback",
					Name:   "Lucas",
					Email:  "lucas@mail.com",
				}, nil
			},
			deleteUserFn: func(_ context.Context, in *userpb.DeleteUserRequest) (*userpb.DeleteUserResponse, error) {
				if in.GetUserId() != "user-rollback" {
					t.Fatalf("unexpected rollback user id: %s", in.GetUserId())
				}
				rolledBack = true
				return &userpb.DeleteUserResponse{Deleted: true}, nil
			},
		},
		walletClient: mockWalletClient{
			createFn: func(context.Context, *walletpb.CreateWalletRequest) (*walletpb.CreateWalletResponse, error) {
				return nil, status.Error(codes.Unavailable, "wallet-service unavailable")
			},
		},
		tokenManager:   testTokenManager(t),
		refreshManager: testRefreshTokenManager(t),
		accessTokenTTL: time.Hour,
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(`{"name":"Lucas","email":"lucas@mail.com","password":"Password123!"}`))
	req.Header.Set("Content-Type", "application/json")

	app.Register(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
	if !rolledBack {
		t.Fatalf("expected user rollback to be executed")
	}
	if !strings.Contains(rr.Body.String(), "\"rolled_back\":true") {
		t.Fatalf("expected rolled_back=true in response body")
	}
}

func TestTopUp_Success(t *testing.T) {
	app := Config{
		walletClient: mockWalletClient{
			topUpFn: func(context.Context, *walletpb.TopUpRequest) (*walletpb.TopUpResponse, error) {
				return &walletpb.TopUpResponse{UserId: "user-001", Balance: 2500}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/topups", bytes.NewBufferString(`{"amount":500}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), userClaimsContextKey, &security.JWTClaims{Subject: "user-001"}))
	rr := httptest.NewRecorder()

	app.TopUp(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
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
		tokenManager:   testTokenManager(t),
		refreshManager: testRefreshTokenManager(t),
		accessTokenTTL: time.Hour,
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
	if !strings.Contains(rr.Body.String(), "\"refresh_token\"") {
		t.Fatalf("expected refresh_token in response")
	}
}

func TestRefreshToken_Success(t *testing.T) {
	refreshManager := testRefreshTokenManager(t)
	refreshToken, err := refreshManager.Generate(security.JWTUser{
		Subject: "user-001",
		Name:    "Lucas",
		Email:   "lucas@mail.com",
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

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
		tokenManager:   testTokenManager(t),
		refreshManager: refreshManager,
		accessTokenTTL: time.Hour,
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(`{"refresh_token":"`+refreshToken+`"}`))
	req.Header.Set("Content-Type", "application/json")

	app.RefreshToken(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"refresh_token\"") {
		t.Fatalf("expected refresh_token in response")
	}
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	app := Config{
		tokenManager:   testTokenManager(t),
		refreshManager: testRefreshTokenManager(t),
		accessTokenTTL: time.Hour,
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBufferString(`{"refresh_token":"invalid"}`))
	req.Header.Set("Content-Type", "application/json")

	app.RefreshToken(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
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

func TestGetMeProfile_Success(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/me/profile", nil)
	req = req.WithContext(context.WithValue(req.Context(), userClaimsContextKey, &security.JWTClaims{Subject: "user-001"}))
	rr := httptest.NewRecorder()

	app.GetMeProfile(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"user_id\":\"user-001\"") {
		t.Fatalf("expected profile payload, got %s", rr.Body.String())
	}
}

func TestGetMeWallet_Success(t *testing.T) {
	app := Config{
		walletClient: mockWalletClient{
			getBalanceFn: func(context.Context, *walletpb.GetBalanceRequest) (*walletpb.GetBalanceResponse, error) {
				return &walletpb.GetBalanceResponse{UserId: "user-001", Balance: 125000.5}, nil
			},
			getTopUpSummaryFn: func(context.Context, *walletpb.GetTopUpSummaryRequest) (*walletpb.GetTopUpSummaryResponse, error) {
				return &walletpb.GetTopUpSummaryResponse{
					UserId:           "user-001",
					TopupCountTotal:  5,
					TopupAmountTotal: 30000,
					TopupCountToday:  1,
					TopupAmountToday: 5000,
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/me/wallet", nil)
	req = req.WithContext(context.WithValue(req.Context(), userClaimsContextKey, &security.JWTClaims{Subject: "user-001"}))
	rr := httptest.NewRecorder()

	app.GetMeWallet(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"balance\":125000.5") {
		t.Fatalf("expected wallet balance payload, got %s", rr.Body.String())
	}
}

func TestGetMeDashboard_Success(t *testing.T) {
	app := Config{
		userClient: mockUserClient{
			getUserFn: func(context.Context, *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
				return &userpb.GetUserResponse{UserId: "user-001", Name: "Lucas", Email: "lucas@mail.com"}, nil
			},
		},
		walletClient: mockWalletClient{
			getBalanceFn: func(context.Context, *walletpb.GetBalanceRequest) (*walletpb.GetBalanceResponse, error) {
				return &walletpb.GetBalanceResponse{UserId: "user-001", Balance: 125000.5}, nil
			},
			getTopUpSummaryFn: func(context.Context, *walletpb.GetTopUpSummaryRequest) (*walletpb.GetTopUpSummaryResponse, error) {
				return &walletpb.GetTopUpSummaryResponse{
					UserId:           "user-001",
					TopupCountTotal:  5,
					TopupAmountTotal: 30000,
					TopupCountToday:  1,
					TopupAmountToday: 5000,
				}, nil
			},
			listTopUpsFn: func(context.Context, *walletpb.ListTopUpsRequest) (*walletpb.ListTopUpsResponse, error) {
				return &walletpb.ListTopUpsResponse{
					Records: []*walletpb.TopUpRecord{{
						TopupId:      "topup-001",
						UserId:       "user-001",
						Amount:       5000,
						BalanceAfter: 125000.5,
						CreatedAt:    "2026-04-15T11:00:00Z",
					}},
				}, nil
			},
		},
		transactionClient: mockTransactionClient{
			getTransferSummaryFn: func(context.Context, *transactionpb.GetTransferSummaryRequest) (*transactionpb.GetTransferSummaryResponse, error) {
				return &transactionpb.GetTransferSummaryResponse{
					UserId:              "user-001",
					SentTotal:           18000,
					ReceivedTotal:       24250,
					SentCountTotal:      12,
					ReceivedCountTotal:  16,
					SentCountToday:      1,
					ReceivedCountToday:  2,
					SentAmountToday:     1500,
					ReceivedAmountToday: 3000,
				}, nil
			},
			listTransfersFn: func(context.Context, *transactionpb.ListTransfersRequest) (*transactionpb.ListTransfersResponse, error) {
				return &transactionpb.ListTransfersResponse{
					Records: []*transactionpb.TransactionRecord{{
						TransactionId: "tx-123",
						SenderId:      "user-001",
						ReceiverId:    "user-002",
						Amount:        1500,
						Status:        "completed",
						CreatedAt:     "2026-04-15T13:10:00Z",
					}},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/me/dashboard", nil)
	req = req.WithContext(context.WithValue(req.Context(), userClaimsContextKey, &security.JWTClaims{Subject: "user-001"}))
	rr := httptest.NewRecorder()

	app.GetMeDashboard(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"total_events\":4") {
		t.Fatalf("expected aggregated dashboard payload, got %s", rr.Body.String())
	}
}

func TestGetMeTopUps_Success(t *testing.T) {
	app := Config{
		walletClient: mockWalletClient{
			listTopUpsFn: func(context.Context, *walletpb.ListTopUpsRequest) (*walletpb.ListTopUpsResponse, error) {
				return &walletpb.ListTopUpsResponse{
					Records: []*walletpb.TopUpRecord{
						{TopupId: "topup-003", UserId: "user-001", Amount: 3000, BalanceAfter: 130000, CreatedAt: "2026-04-15T13:00:00Z"},
						{TopupId: "topup-002", UserId: "user-001", Amount: 2000, BalanceAfter: 127000, CreatedAt: "2026-04-15T12:00:00Z"},
						{TopupId: "topup-001", UserId: "user-001", Amount: 1000, BalanceAfter: 125000, CreatedAt: "2026-04-15T11:00:00Z"},
					},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/me/topups?page=1&page_size=2", nil)
	req = req.WithContext(context.WithValue(req.Context(), userClaimsContextKey, &security.JWTClaims{Subject: "user-001"}))
	rr := httptest.NewRecorder()

	app.GetMeTopUps(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"has_next\":true") {
		t.Fatalf("expected paginated topups response, got %s", rr.Body.String())
	}
}

func TestGetMeActivity_All_Success(t *testing.T) {
	app := Config{
		walletClient: mockWalletClient{
			listTopUpsFn: func(context.Context, *walletpb.ListTopUpsRequest) (*walletpb.ListTopUpsResponse, error) {
				return &walletpb.ListTopUpsResponse{
					Records: []*walletpb.TopUpRecord{{
						TopupId:      "topup-001",
						UserId:       "user-001",
						Amount:       5000,
						BalanceAfter: 125000.5,
						CreatedAt:    "2026-04-15T11:00:00Z",
					}},
				}, nil
			},
		},
		transactionClient: mockTransactionClient{
			listTransfersFn: func(context.Context, *transactionpb.ListTransfersRequest) (*transactionpb.ListTransfersResponse, error) {
				return &transactionpb.ListTransfersResponse{
					Records: []*transactionpb.TransactionRecord{{
						TransactionId: "tx-123",
						SenderId:      "user-001",
						ReceiverId:    "user-002",
						Amount:        1500,
						Status:        "completed",
						CreatedAt:     "2026-04-15T13:10:00Z",
					}},
				}, nil
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/me/activity?kind=all&page=1&page_size=20", nil)
	req = req.WithContext(context.WithValue(req.Context(), userClaimsContextKey, &security.JWTClaims{Subject: "user-001"}))
	rr := httptest.NewRecorder()

	app.GetMeActivity(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "\"kind\":\"transfer_sent\"") || !strings.Contains(body, "\"kind\":\"topup\"") {
		t.Fatalf("expected combined activity response, got %s", body)
	}
}

func TestGetMeActivity_InvalidPage(t *testing.T) {
	app := Config{}
	req := httptest.NewRequest(http.MethodGet, "/me/activity?page=0", nil)
	req = req.WithContext(context.WithValue(req.Context(), userClaimsContextKey, &security.JWTClaims{Subject: "user-001"}))
	rr := httptest.NewRecorder()

	app.GetMeActivity(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}
