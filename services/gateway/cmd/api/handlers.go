package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	fraudpb "github.com/peer-ledger/gen/fraud"
	userpb "github.com/peer-ledger/gen/user"
	walletpb "github.com/peer-ledger/gen/wallet"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (app *Config) Health(w http.ResponseWriter, r *http.Request) {
	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "ok",
	})
}

type transferRequest struct {
	SenderID       string  `json:"sender_id"`
	ReceiverID     string  `json:"receiver_id"`
	Amount         float64 `json:"amount"`
	IdempotencyKey string  `json:"idempotency_key"`
}

func (app *Config) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	var payload transferRequest
	if err := app.readJSON(w, r, &payload); err != nil {
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	if err := validateTransferPayload(payload); err != nil {
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	if statusCode, err := app.ensureUserExists(payload.SenderID); err != nil {
		_ = app.errorJSON(w, err, statusCode)
		return
	}

	if statusCode, err := app.ensureUserExists(payload.ReceiverID); err != nil {
		_ = app.errorJSON(w, err, statusCode)
		return
	}

	fraudResp, fraudStatusCode, err := app.evaluateFraud(payload)
	if err != nil {
		_ = app.errorJSON(w, err, fraudStatusCode)
		return
	}

	if !fraudResp.Allowed {
		_ = app.writeJSON(w, http.StatusForbidden, jsonResponse{
			Error:   true,
			Message: "transfer blocked by fraud service",
			Data: map[string]any{
				"reason":    fraudResp.Reason,
				"rule_code": fraudResp.RuleCode,
			},
		})
		return
	}

	walletResp, walletStatusCode, err := app.executeWalletTransfer(payload)
	if err != nil {
		_ = app.errorJSON(w, err, walletStatusCode)
		return
	}

	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "transfer executed successfully via wallet-service",
		Data: map[string]any{
			"transaction_id": walletResp.GetTransactionId(),
			"sender_balance": walletResp.GetSenderBalance(),
			"sender_id":      payload.SenderID,
			"receiver_id":    payload.ReceiverID,
			"amount":         payload.Amount,
		},
	})
}

func (app *Config) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "userID"))
	if userID == "" {
		_ = app.errorJSON(w, errors.New("userID is required"), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := app.userClient.GetUser(ctx, &userpb.GetUserRequest{
		Id: userID,
	})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapGrpcToHTTPStatus(err))
		return
	}

	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "ok",
		Data: map[string]any{
			"user_id": resp.UserId,
			"name":    resp.Name,
			"email":   resp.Email,
		},
	})
}

func (app *Config) UserExists(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "userID"))
	if userID == "" {
		_ = app.errorJSON(w, errors.New("userID is required"), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := app.userClient.UserExists(ctx, &userpb.UserExistsRequest{
		UserId: userID,
	})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapGrpcToHTTPStatus(err))
		return
	}

	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "ok",
		Data: map[string]any{
			"user_id": userID,
			"exists":  resp.Exists,
		},
	})
}

func validateTransferPayload(p transferRequest) error {
	if strings.TrimSpace(p.SenderID) == "" || strings.TrimSpace(p.ReceiverID) == "" {
		return errors.New("sender_id and receiver_id are required")
	}
	if p.SenderID == p.ReceiverID {
		return errors.New("sender_id and receiver_id must be different")
	}
	if p.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if strings.TrimSpace(p.IdempotencyKey) == "" {
		return errors.New("idempotency_key is required")
	}
	return nil
}

func (app *Config) evaluateFraud(payload transferRequest) (*fraudpb.EvaluateResponse, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := app.fraudClient.EvaluateTransfer(ctx, &fraudpb.EvaluateRequest{
		SenderId:       payload.SenderID,
		ReceiverId:     payload.ReceiverID,
		Amount:         payload.Amount,
		IdempotencyKey: payload.IdempotencyKey,
	})
	if err != nil {
		return nil, mapFraudGrpcErrorStatus(err), errors.New("fraud-service unavailable")
	}

	return resp, 0, nil
}

func (app *Config) executeWalletTransfer(payload transferRequest) (*walletpb.TransferResponse, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := app.walletClient.Transfer(ctx, &walletpb.TransferRequest{
		SenderId:       payload.SenderID,
		ReceiverId:     payload.ReceiverID,
		Amount:         payload.Amount,
		IdempotencyKey: payload.IdempotencyKey,
	})
	if err != nil {
		return nil, mapWalletGrpcErrorStatus(err), mapGrpcToHTTPError(err)
	}

	return resp, 0, nil
}

func (app *Config) ensureUserExists(userID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := app.userClient.UserExists(ctx, &userpb.UserExistsRequest{
		UserId: userID,
	})
	if err != nil {
		return mapGrpcToHTTPStatus(err), mapGrpcToHTTPError(err)
	}

	if !resp.Exists {
		return http.StatusNotFound, errors.New("user not found: " + userID)
	}

	return 0, nil
}

func mapGrpcToHTTPStatus(err error) int {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusBadGateway
	}

	switch st.Code() {
	case codes.NotFound:
		return http.StatusNotFound
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadGateway
	}
}

func mapGrpcToHTTPError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return errors.New("grpc request failed")
	}
	return errors.New(st.Message())
}

func mapFraudGrpcErrorStatus(err error) int {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusBadGateway
	}

	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadGateway
	}
}

func mapWalletGrpcErrorStatus(err error) int {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusBadGateway
	}

	switch st.Code() {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.FailedPrecondition:
		return http.StatusConflict
	case codes.NotFound:
		return http.StatusNotFound
	case codes.Unavailable, codes.DeadlineExceeded:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadGateway
	}
}
