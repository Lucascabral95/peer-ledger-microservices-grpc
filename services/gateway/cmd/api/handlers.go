package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	userpb "github.com/peer-ledger/gen/user"
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

	if err := app.ensureUserExists(payload.SenderID); err != nil {
		_ = app.errorJSON(w, err, http.StatusNotFound)
		return
	}

	if err := app.ensureUserExists(payload.ReceiverID); err != nil {
		_ = app.errorJSON(w, err, http.StatusNotFound)
		return
	}

	_ = app.writeJSON(w, http.StatusAccepted, jsonResponse{
		Error:   false,
		Message: "users validated via gRPC; next step is fraud/wallet/transaction orchestration",
		Data: map[string]any{
			"sender_id":       payload.SenderID,
			"receiver_id":     payload.ReceiverID,
			"amount":          payload.Amount,
			"idempotency_key": payload.IdempotencyKey,
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

func (app *Config) ensureUserExists(userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := app.userClient.UserExists(ctx, &userpb.UserExistsRequest{
		UserId: userID,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return errors.New("user not found: " + userID)
		}
		return errors.New("user-service unavailable")
	}

	if !resp.Exists {
		return errors.New("user not found: " + userID)
	}

	return nil
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
