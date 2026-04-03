package main

import (
	"errors"
	"testing"
	"time"

	gatewayconfig "github.com/peer-ledger/services/gateway/internal/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGatewayConfigLoad_Defaults(t *testing.T) {
	cfg, err := gatewayconfig.LoadFromLookup(func(string) string { return "" })
	if err != nil {
		t.Fatalf("LoadFromLookup() error: %v", err)
	}
	if cfg.HTTPPort != "8080" {
		t.Fatalf("expected 8080, got %q", cfg.HTTPPort)
	}
	if !cfg.RateLimitEnabled {
		t.Fatalf("expected rate limiter enabled by default")
	}
}

func TestValidateTransferPayload(t *testing.T) {
	cases := []struct {
		name    string
		payload transferRequest
		wantErr bool
	}{
		{
			name: "valid",
			payload: transferRequest{
				ReceiverID:     "user-002",
				SenderID:       "user-001",
				Amount:         10,
				IdempotencyKey: "k1",
			},
		},
		{
			name: "same users",
			payload: transferRequest{
				SenderID:       "user-001",
				ReceiverID:     "user-001",
				Amount:         10,
				IdempotencyKey: "k1",
			},
			wantErr: true,
		},
		{
			name: "missing key",
			payload: transferRequest{
				SenderID:   "user-001",
				ReceiverID: "user-002",
				Amount:     10,
			},
			wantErr: true,
		},
		{
			name: "missing receiver",
			payload: transferRequest{
				SenderID:       "user-001",
				Amount:         10,
				IdempotencyKey: "k1",
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		err := validateTransferPayload(tc.payload)
		if tc.wantErr && err == nil {
			t.Fatalf("%s: expected error", tc.name)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
	}
}

func TestMapGrpcToHTTPStatus(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{status.Error(codes.NotFound, "x"), 404},
		{status.Error(codes.InvalidArgument, "x"), 400},
		{status.Error(codes.AlreadyExists, "x"), 409},
		{status.Error(codes.Unauthenticated, "x"), 401},
		{status.Error(codes.DeadlineExceeded, "x"), 504},
		{status.Error(codes.Unavailable, "x"), 503},
		{errors.New("plain"), 502},
	}

	for _, tc := range cases {
		if got := mapGrpcToHTTPStatus(tc.err); got != tc.want {
			t.Fatalf("mapGrpcToHTTPStatus(%v): expected %d, got %d", tc.err, tc.want, got)
		}
	}
}

func TestMapGrpcToHTTPError(t *testing.T) {
	err := mapGrpcToHTTPError(status.Error(codes.NotFound, "user not found"))
	if err.Error() != "user not found" {
		t.Fatalf("expected grpc message, got %q", err.Error())
	}

	err = mapGrpcToHTTPError(errors.New("plain"))
	if err.Error() != "grpc request failed" {
		t.Fatalf("expected generic grpc message, got %q", err.Error())
	}
}

func TestMapFraudGrpcErrorStatus(t *testing.T) {
	if got := mapFraudGrpcErrorStatus(status.Error(codes.Unavailable, "x")); got != 503 {
		t.Fatalf("expected 503, got %d", got)
	}
	if got := mapFraudGrpcErrorStatus(status.Error(codes.Internal, "x")); got != 502 {
		t.Fatalf("expected 502, got %d", got)
	}
}

func TestMapWalletGrpcErrorStatus(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{status.Error(codes.InvalidArgument, "x"), 400},
		{status.Error(codes.FailedPrecondition, "x"), 409},
		{status.Error(codes.NotFound, "x"), 404},
		{status.Error(codes.Unavailable, "x"), 503},
		{errors.New("plain"), 502},
	}

	for _, tc := range cases {
		if got := mapWalletGrpcErrorStatus(tc.err); got != tc.want {
			t.Fatalf("mapWalletGrpcErrorStatus(%v): expected %d, got %d", tc.err, tc.want, got)
		}
	}
}

func TestMapTransactionGrpcErrorStatus(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{status.Error(codes.InvalidArgument, "x"), 400},
		{status.Error(codes.Unavailable, "x"), 503},
		{status.Error(codes.DeadlineExceeded, "x"), 503},
		{errors.New("plain"), 502},
	}

	for _, tc := range cases {
		if got := mapTransactionGrpcErrorStatus(tc.err); got != tc.want {
			t.Fatalf("mapTransactionGrpcErrorStatus(%v): expected %d, got %d", tc.err, tc.want, got)
		}
	}
}

func TestDialGRPCWithRetry_InvalidAddress(t *testing.T) {
	_, err := dialGRPCWithRetry("127.0.0.1:1", 20*time.Millisecond, 1)
	if err == nil {
		t.Fatalf("expected dial error")
	}
}
