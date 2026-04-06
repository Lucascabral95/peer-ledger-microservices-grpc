package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	fraudpb "github.com/peer-ledger/gen/fraud"
	transactionpb "github.com/peer-ledger/gen/transaction"
	userpb "github.com/peer-ledger/gen/user"
	walletpb "github.com/peer-ledger/gen/wallet"
	"github.com/peer-ledger/internal/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (app *Config) Health(w http.ResponseWriter, r *http.Request) {
	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "ok",
	})
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type topUpRequest struct {
	Amount float64 `json:"amount"`
}

type transferRequest struct {
	SenderID       string  `json:"-"`
	ReceiverID     string  `json:"receiver_id"`
	Amount         float64 `json:"amount"`
	IdempotencyKey string  `json:"idempotency_key"`
}

func (app *Config) Register(w http.ResponseWriter, r *http.Request) {
	var payload registerRequest
	if err := app.readJSON(w, r, &payload); err != nil {
		app.logEvent(r.Context(), "warn", "register payload validation failed", map[string]any{
			"route":  "/auth/register",
			"status": http.StatusBadRequest,
			"error":  err.Error(),
		})
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}
	if err := validateRegisterPayload(payload); err != nil {
		app.logEvent(r.Context(), "warn", "register payload validation failed", map[string]any{
			"route":  "/auth/register",
			"status": http.StatusBadRequest,
			"error":  err.Error(),
		})
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := app.userClient.Register(ctx, &userpb.RegisterRequest{
		Name:     strings.TrimSpace(payload.Name),
		Email:    strings.TrimSpace(payload.Email),
		Password: payload.Password,
	})
	if err != nil {
		app.logEvent(r.Context(), "error", "register failed in user-service", map[string]any{
			"route":  "/auth/register",
			"status": mapGrpcToHTTPStatus(err),
			"error":  mapGrpcToHTTPError(err).Error(),
			"email":  strings.TrimSpace(payload.Email),
		})
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapGrpcToHTTPStatus(err))
		return
	}

	if statusCode, walletErr := app.createWalletForUser(resp.GetUserId()); walletErr != nil {
		app.logEvent(r.Context(), "error", "wallet provisioning failed", map[string]any{
			"route":   "/auth/register",
			"status":  statusCode,
			"user_id": resp.GetUserId(),
			"error":   walletErr.Error(),
		})
		_ = app.writeJSON(w, statusCode, jsonResponse{
			Error:   true,
			Message: "user created but wallet provisioning failed",
			Data: map[string]any{
				"user_id": resp.GetUserId(),
				"stage":   "wallet_provisioning",
			},
		})
		return
	}

	token, err := app.issueJWT(resp.GetUserId(), resp.GetName(), resp.GetEmail())
	if err != nil {
		app.logEvent(r.Context(), "error", "jwt issue failed on register", map[string]any{
			"route":   "/auth/register",
			"status":  http.StatusInternalServerError,
			"user_id": resp.GetUserId(),
			"error":   err.Error(),
		})
		_ = app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.logEvent(r.Context(), "info", "user registered successfully", map[string]any{
		"route":   "/auth/register",
		"status":  http.StatusCreated,
		"user_id": resp.GetUserId(),
		"email":   resp.GetEmail(),
	})

	_ = app.writeJSON(w, http.StatusCreated, jsonResponse{
		Error:   false,
		Message: "user registered successfully",
		Data: map[string]any{
			"token": token,
			"user": map[string]any{
				"user_id": resp.GetUserId(),
				"name":    resp.GetName(),
				"email":   resp.GetEmail(),
			},
		},
	})
}

func (app *Config) Login(w http.ResponseWriter, r *http.Request) {
	var payload loginRequest
	if err := app.readJSON(w, r, &payload); err != nil {
		app.logEvent(r.Context(), "warn", "login payload validation failed", map[string]any{
			"route":  "/auth/login",
			"status": http.StatusBadRequest,
			"error":  err.Error(),
		})
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}
	if err := validateLoginPayload(payload); err != nil {
		app.logEvent(r.Context(), "warn", "login payload validation failed", map[string]any{
			"route":  "/auth/login",
			"status": http.StatusBadRequest,
			"error":  err.Error(),
		})
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := app.userClient.Login(ctx, &userpb.LoginRequest{
		Email:    strings.TrimSpace(payload.Email),
		Password: payload.Password,
	})
	if err != nil {
		app.logEvent(r.Context(), "warn", "login failed", map[string]any{
			"route":  "/auth/login",
			"status": mapGrpcToHTTPStatus(err),
			"error":  mapGrpcToHTTPError(err).Error(),
			"email":  strings.TrimSpace(payload.Email),
		})
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapGrpcToHTTPStatus(err))
		return
	}

	token, err := app.issueJWT(resp.GetUserId(), resp.GetName(), resp.GetEmail())
	if err != nil {
		app.logEvent(r.Context(), "error", "jwt issue failed on login", map[string]any{
			"route":   "/auth/login",
			"status":  http.StatusInternalServerError,
			"user_id": resp.GetUserId(),
			"error":   err.Error(),
		})
		_ = app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	app.logEvent(r.Context(), "info", "login successful", map[string]any{
		"route":   "/auth/login",
		"status":  http.StatusOK,
		"user_id": resp.GetUserId(),
		"email":   resp.GetEmail(),
	})

	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "login successful",
		Data: map[string]any{
			"token": token,
			"user": map[string]any{
				"user_id": resp.GetUserId(),
				"name":    resp.GetName(),
				"email":   resp.GetEmail(),
			},
		},
	})
}

func (app *Config) TopUp(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		app.logEvent(r.Context(), "warn", "topup unauthorized", map[string]any{
			"route":  "/topups",
			"status": http.StatusUnauthorized,
		})
		_ = app.errorJSON(w, errors.New("authentication required"), http.StatusUnauthorized)
		return
	}

	var payload topUpRequest
	if err := app.readJSON(w, r, &payload); err != nil {
		app.logEvent(r.Context(), "warn", "topup payload validation failed", map[string]any{
			"route":   "/topups",
			"status":  http.StatusBadRequest,
			"user_id": claims.Subject,
			"error":   err.Error(),
		})
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}
	if payload.Amount <= 0 {
		app.logEvent(r.Context(), "warn", "topup payload validation failed", map[string]any{
			"route":   "/topups",
			"status":  http.StatusBadRequest,
			"user_id": claims.Subject,
			"error":   "amount must be greater than zero",
		})
		_ = app.errorJSON(w, errors.New("amount must be greater than zero"), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := app.walletClient.TopUp(ctx, &walletpb.TopUpRequest{
		UserId: claims.Subject,
		Amount: payload.Amount,
	})
	if err != nil {
		app.logEvent(r.Context(), "error", "topup failed in wallet-service", map[string]any{
			"route":   "/topups",
			"status":  mapWalletGrpcErrorStatus(err),
			"user_id": claims.Subject,
			"amount":  payload.Amount,
			"error":   mapGrpcToHTTPError(err).Error(),
		})
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapWalletGrpcErrorStatus(err))
		return
	}

	app.logEvent(r.Context(), "info", "topup completed", map[string]any{
		"route":   "/topups",
		"status":  http.StatusOK,
		"user_id": claims.Subject,
		"amount":  payload.Amount,
		"balance": resp.GetBalance(),
	})

	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "wallet topped up successfully",
		Data: map[string]any{
			"user_id": claims.Subject,
			"balance": resp.GetBalance(),
			"amount":  payload.Amount,
		},
	})
}

func (app *Config) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		app.logEvent(r.Context(), "warn", "transfer unauthorized", map[string]any{
			"route":  "/transfers",
			"status": http.StatusUnauthorized,
		})
		_ = app.errorJSON(w, errors.New("authentication required"), http.StatusUnauthorized)
		return
	}

	var payload transferRequest
	if err := app.readJSON(w, r, &payload); err != nil {
		app.logEvent(r.Context(), "warn", "transfer payload validation failed", map[string]any{
			"route":   "/transfers",
			"status":  http.StatusBadRequest,
			"user_id": claims.Subject,
			"error":   err.Error(),
		})
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	payload.SenderID = claims.Subject
	if err := validateTransferPayload(payload); err != nil {
		app.logEvent(r.Context(), "warn", "transfer payload validation failed", map[string]any{
			"route":   "/transfers",
			"status":  http.StatusBadRequest,
			"user_id": claims.Subject,
			"error":   err.Error(),
		})
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	if statusCode, err := app.ensureUserExists(payload.SenderID); err != nil {
		app.logEvent(r.Context(), "warn", "transfer sender validation failed", map[string]any{
			"route":   "/transfers",
			"status":  statusCode,
			"user_id": payload.SenderID,
			"error":   err.Error(),
		})
		_ = app.errorJSON(w, err, statusCode)
		return
	}

	if statusCode, err := app.ensureUserExists(payload.ReceiverID); err != nil {
		app.logEvent(r.Context(), "warn", "transfer receiver validation failed", map[string]any{
			"route":       "/transfers",
			"status":      statusCode,
			"user_id":     payload.SenderID,
			"receiver_id": payload.ReceiverID,
			"error":       err.Error(),
		})
		_ = app.errorJSON(w, err, statusCode)
		return
	}

	fraudResp, fraudStatusCode, err := app.evaluateFraud(payload)
	if err != nil {
		app.logEvent(r.Context(), "error", "fraud evaluation failed", map[string]any{
			"route":       "/transfers",
			"status":      fraudStatusCode,
			"user_id":     payload.SenderID,
			"receiver_id": payload.ReceiverID,
			"amount":      payload.Amount,
			"error":       err.Error(),
		})
		_ = app.errorJSON(w, err, fraudStatusCode)
		return
	}

	if !fraudResp.Allowed {
		app.logEvent(r.Context(), "warn", "transfer blocked by fraud service", map[string]any{
			"route":       "/transfers",
			"status":      http.StatusForbidden,
			"user_id":     payload.SenderID,
			"receiver_id": payload.ReceiverID,
			"amount":      payload.Amount,
			"rule_code":   fraudResp.RuleCode,
			"reason":      fraudResp.Reason,
		})
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
		app.logEvent(r.Context(), "error", "wallet transfer failed", map[string]any{
			"route":       "/transfers",
			"status":      walletStatusCode,
			"user_id":     payload.SenderID,
			"receiver_id": payload.ReceiverID,
			"amount":      payload.Amount,
			"error":       err.Error(),
		})
		_ = app.errorJSON(w, err, walletStatusCode)
		return
	}

	if statusCode, err := app.recordTransaction(payload, walletResp); err != nil {
		app.logEvent(r.Context(), "error", "failed to record audit transaction", map[string]any{
			"route":          "/transfers",
			"status":         statusCode,
			"user_id":        payload.SenderID,
			"receiver_id":    payload.ReceiverID,
			"amount":         payload.Amount,
			"transaction_id": walletResp.GetTransactionId(),
			"error":          err.Error(),
		})
		_ = app.writeJSON(w, statusCode, jsonResponse{
			Error:   true,
			Message: "transfer executed in wallet-service but failed to record audit transaction",
			Data: map[string]any{
				"transaction_id":  walletResp.GetTransactionId(),
				"sender_balance":  walletResp.GetSenderBalance(),
				"stage":           "transaction_recording",
				"retryable":       true,
				"idempotency_key": payload.IdempotencyKey,
			},
		})
		return
	}

	app.logEvent(r.Context(), "info", "transfer completed", map[string]any{
		"route":          "/transfers",
		"status":         http.StatusOK,
		"user_id":        payload.SenderID,
		"receiver_id":    payload.ReceiverID,
		"amount":         payload.Amount,
		"transaction_id": walletResp.GetTransactionId(),
		"sender_balance": walletResp.GetSenderBalance(),
	})

	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "transfer executed and recorded successfully",
		Data: map[string]any{
			"transaction_id": walletResp.GetTransactionId(),
			"sender_balance": walletResp.GetSenderBalance(),
			"sender_id":      payload.SenderID,
			"receiver_id":    payload.ReceiverID,
			"amount":         payload.Amount,
		},
	})
}

func (app *Config) GetHistory(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		_ = app.errorJSON(w, errors.New("authentication required"), http.StatusUnauthorized)
		return
	}

	userID := strings.TrimSpace(chi.URLParam(r, "userID"))
	if userID == "" {
		_ = app.errorJSON(w, errors.New("userID is required"), http.StatusBadRequest)
		return
	}
	if claims.Subject != userID {
		_ = app.errorJSON(w, errors.New("forbidden"), http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := app.transactionClient.GetHistory(ctx, &transactionpb.GetHistoryRequest{
		UserId: userID,
	})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapTransactionGrpcErrorStatus(err))
		return
	}

	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "ok",
		Data: map[string]any{
			"user_id": userID,
			"records": resp.GetRecords(),
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

	resp, err := app.userClient.GetUser(ctx, &userpb.GetUserRequest{Id: userID})
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

	resp, err := app.userClient.UserExists(ctx, &userpb.UserExistsRequest{UserId: userID})
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

func validateRegisterPayload(p registerRequest) error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(p.Email) == "" {
		return errors.New("email is required")
	}
	if strings.TrimSpace(p.Password) == "" {
		return errors.New("password is required")
	}
	return nil
}

func validateLoginPayload(p loginRequest) error {
	if strings.TrimSpace(p.Email) == "" {
		return errors.New("email is required")
	}
	if strings.TrimSpace(p.Password) == "" {
		return errors.New("password is required")
	}
	return nil
}

func validateTransferPayload(p transferRequest) error {
	if strings.TrimSpace(p.SenderID) == "" {
		return errors.New("authenticated sender is required")
	}
	if strings.TrimSpace(p.ReceiverID) == "" {
		return errors.New("receiver_id is required")
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

func (app *Config) issueJWT(userID, name, email string) (string, error) {
	if app.tokenManager == nil {
		return "", errors.New("jwt manager is not configured")
	}

	return app.tokenManager.Generate(security.JWTUser{
		Subject: userID,
		Name:    name,
		Email:   email,
	})
}

func (app *Config) createWalletForUser(userID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := app.walletClient.CreateWallet(ctx, &walletpb.CreateWalletRequest{
		UserId: userID,
	})
	if err != nil {
		return mapWalletGrpcErrorStatus(err), err
	}

	return 0, nil
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

func (app *Config) recordTransaction(payload transferRequest, walletResp *walletpb.TransferResponse) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := app.transactionClient.Record(ctx, &transactionpb.RecordRequest{
		SenderId:       payload.SenderID,
		ReceiverId:     payload.ReceiverID,
		Amount:         payload.Amount,
		IdempotencyKey: payload.IdempotencyKey,
		TransactionId:  walletResp.GetTransactionId(),
	})
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

func (app *Config) ensureUserExists(userID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := app.userClient.UserExists(ctx, &userpb.UserExistsRequest{UserId: userID})
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
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.Unauthenticated:
		return http.StatusUnauthorized
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
	case codes.AlreadyExists:
		return http.StatusConflict
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

func mapTransactionGrpcErrorStatus(err error) int {
	st, ok := status.FromError(err)
	if !ok {
		return http.StatusBadGateway
	}

	switch st.Code() {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unavailable, codes.DeadlineExceeded:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadGateway
	}
}
