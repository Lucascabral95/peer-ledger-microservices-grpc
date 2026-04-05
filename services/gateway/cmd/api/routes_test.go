package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	fraudpb "github.com/peer-ledger/gen/fraud"
	transactionpb "github.com/peer-ledger/gen/transaction"
	userpb "github.com/peer-ledger/gen/user"
	walletpb "github.com/peer-ledger/gen/wallet"
	gatewaymiddleware "github.com/peer-ledger/services/gateway/internal/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

type routeUserClient struct{}

func (routeUserClient) GetUser(ctx context.Context, in *userpb.GetUserRequest, _ ...grpc.CallOption) (*userpb.GetUserResponse, error) {
	return &userpb.GetUserResponse{UserId: in.GetId(), Name: "Lucas", Email: "lucas@mail.com"}, nil
}

func (routeUserClient) UserExists(context.Context, *userpb.UserExistsRequest, ...grpc.CallOption) (*userpb.UserExistsResponse, error) {
	return &userpb.UserExistsResponse{Exists: true}, nil
}

func (routeUserClient) Register(context.Context, *userpb.RegisterRequest, ...grpc.CallOption) (*userpb.RegisterResponse, error) {
	return &userpb.RegisterResponse{UserId: "user-new", Name: "Lucas", Email: "lucas@mail.com"}, nil
}

func (routeUserClient) Login(context.Context, *userpb.LoginRequest, ...grpc.CallOption) (*userpb.LoginResponse, error) {
	return &userpb.LoginResponse{UserId: "user-001", Name: "Lucas", Email: "lucas@mail.com"}, nil
}

type routeFraudClient struct{}

func (routeFraudClient) EvaluateTransfer(context.Context, *fraudpb.EvaluateRequest, ...grpc.CallOption) (*fraudpb.EvaluateResponse, error) {
	return &fraudpb.EvaluateResponse{Allowed: true, RuleCode: "OK"}, nil
}

type routeWalletClient struct{}

func (routeWalletClient) GetBalance(context.Context, *walletpb.GetBalanceRequest, ...grpc.CallOption) (*walletpb.GetBalanceResponse, error) {
	return &walletpb.GetBalanceResponse{}, nil
}

func (routeWalletClient) CreateWallet(context.Context, *walletpb.CreateWalletRequest, ...grpc.CallOption) (*walletpb.CreateWalletResponse, error) {
	return &walletpb.CreateWalletResponse{UserId: "user-new", Balance: 0}, nil
}

func (routeWalletClient) TopUp(context.Context, *walletpb.TopUpRequest, ...grpc.CallOption) (*walletpb.TopUpResponse, error) {
	return &walletpb.TopUpResponse{UserId: "user-001", Balance: 100}, nil
}

func (routeWalletClient) Transfer(context.Context, *walletpb.TransferRequest, ...grpc.CallOption) (*walletpb.TransferResponse, error) {
	return &walletpb.TransferResponse{TransactionId: "tx-route", SenderBalance: 1}, nil
}

type routeTransactionClient struct{}

func (routeTransactionClient) Record(context.Context, *transactionpb.RecordRequest, ...grpc.CallOption) (*transactionpb.RecordResponse, error) {
	return &transactionpb.RecordResponse{Success: true}, nil
}

func (routeTransactionClient) GetHistory(context.Context, *transactionpb.GetHistoryRequest, ...grpc.CallOption) (*transactionpb.GetHistoryResponse, error) {
	return &transactionpb.GetHistoryResponse{
		Records: []*transactionpb.TransactionRecord{{TransactionId: "tx-history"}},
	}, nil
}

func newRouteApp(t *testing.T) Config {
	t.Helper()
	registry := prometheus.NewRegistry()
	return Config{
		userClient:        routeUserClient{},
		fraudClient:       routeFraudClient{},
		walletClient:      routeWalletClient{},
		transactionClient: routeTransactionClient{},
		tokenManager:      testTokenManager(t),
		httpMetrics:       gatewaymiddleware.NewHTTPMetrics(registry),
		metricsHandler:    promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true}),
		metricsPath:       "/metrics",
	}
}

func TestRoutes_Health(t *testing.T) {
	app := newRouteApp(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestRoutes_Register(t *testing.T) {
	app := newRouteApp(t)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	rr := httptest.NewRecorder()

	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestRoutes_Transfers_Unauthorized(t *testing.T) {
	app := newRouteApp(t)
	req := httptest.NewRequest(http.MethodPost, "/transfers", nil)
	rr := httptest.NewRecorder()

	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestRoutes_GetHistory_Unauthorized(t *testing.T) {
	app := newRouteApp(t)
	req := httptest.NewRequest(http.MethodGet, "/history/user-001", nil)
	rr := httptest.NewRecorder()

	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rr.Code)
	}
}

func TestRoutes_GetUser(t *testing.T) {
	app := newRouteApp(t)
	req := httptest.NewRequest(http.MethodGet, "/users/user-001", nil)
	rr := httptest.NewRecorder()

	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestRoutes_Metrics(t *testing.T) {
	app := newRouteApp(t)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	app.routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}
