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

// Health godoc
// @Summary Service health
// @Description Lightweight liveness endpoint for the public gateway.
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
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

type refreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
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

// Register godoc
// @Summary Register a user
// @Description Creates a user in user-service, provisions the wallet, and returns a bearer JWT.
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body RegisterRequestDoc true "Register payload"
// @Success 201 {object} RegisterResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} WalletProvisioningFailureResponse
// @Failure 504 {object} ErrorResponse
// @Router /auth/register [post]
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

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
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

	if walletStatus, walletErr := app.createWalletForUser(r.Context(), resp.GetUserId()); walletErr != nil {
		if rollbackErr := app.deleteUserAfterWalletFailure(r.Context(), resp.GetUserId()); rollbackErr != nil {
			app.logEvent(r.Context(), "error", "user rollback failed after wallet provisioning error", map[string]any{
				"route":   "/auth/register",
				"status":  http.StatusBadGateway,
				"user_id": resp.GetUserId(),
				"error":   rollbackErr.Error(),
			})
		}
		app.logEvent(r.Context(), "error", "wallet provisioning failed", map[string]any{
			"route":   "/auth/register",
			"status":  walletStatus,
			"user_id": resp.GetUserId(),
			"error":   walletErr.Error(),
		})
		_ = app.writeJSON(w, walletStatus, jsonResponse{
			Error:   true,
			Message: "user registration failed because wallet provisioning did not complete",
			Data: map[string]any{
				"user_id":     resp.GetUserId(),
				"stage":       "wallet_provisioning",
				"rolled_back": true,
			},
		})
		return
	}

	authPayload, err := app.issueTokenPair(resp.GetUserId(), resp.GetName(), resp.GetEmail())
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
		Data:    authPayload,
	})
}

// Login godoc
// @Summary Login a user
// @Description Validates credentials through user-service and returns a bearer JWT.
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body LoginRequestDoc true "Login payload"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 504 {object} ErrorResponse
// @Router /auth/login [post]
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

	authPayload, err := app.issueTokenPair(resp.GetUserId(), resp.GetName(), resp.GetEmail())
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
		Data:    authPayload,
	})
}

// RefreshToken godoc
// @Summary Refresh an access token
// @Description Exchanges a valid refresh token for a new access token and a rotated refresh token.
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body RefreshTokenRequestDoc true "Refresh token payload"
// @Success 200 {object} RefreshTokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 504 {object} ErrorResponse
// @Router /auth/refresh [post]
func (app *Config) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var payload refreshTokenRequest
	if err := app.readJSON(w, r, &payload); err != nil {
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.RefreshToken) == "" {
		_ = app.errorJSON(w, errors.New("refresh_token is required"), http.StatusBadRequest)
		return
	}
	if app.refreshManager == nil {
		_ = app.errorJSON(w, errors.New("refresh token manager is not configured"), http.StatusInternalServerError)
		return
	}

	claims, err := app.refreshManager.Parse(strings.TrimSpace(payload.RefreshToken))
	if err != nil {
		_ = app.errorJSON(w, errors.New("invalid or expired refresh token"), http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	userResp, err := app.userClient.GetUser(ctx, &userpb.GetUserRequest{Id: claims.Subject})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapGrpcToHTTPStatus(err))
		return
	}

	authPayload, err := app.issueTokenPair(userResp.GetUserId(), userResp.GetName(), userResp.GetEmail())
	if err != nil {
		_ = app.errorJSON(w, err, http.StatusInternalServerError)
		return
	}

	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "token refreshed successfully",
		Data:    authPayload,
	})
}

// TopUp godoc
// @Summary Top up wallet balance
// @Description Credits funds to the authenticated user's wallet.
// @Tags wallet
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body TopUpRequestDoc true "Top-up payload"
// @Success 200 {object} TopUpResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /topups [post]
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

// CreateTransfer godoc
// @Summary Execute a transfer
// @Description Runs user validation, fraud evaluation, wallet transfer, and transaction recording for the authenticated sender.
// @Tags transfers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body TransferRequestDoc true "Transfer payload"
// @Success 200 {object} TransferResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} FraudBlockedResponse
// @Failure 404 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} TransferAuditFailureResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 504 {object} ErrorResponse
// @Router /transfers [post]
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
				"transaction_id":   walletResp.GetTransactionId(),
				"sender_balance":   walletResp.GetSenderBalance(),
				"receiver_balance": walletResp.GetReceiverBalance(),
				"stage":            "transaction_recording",
				"retryable":        true,
				"idempotency_key":  payload.IdempotencyKey,
			},
		})
		return
	}

	app.logEvent(r.Context(), "info", "transfer completed", map[string]any{
		"route":            "/transfers",
		"status":           http.StatusOK,
		"user_id":          payload.SenderID,
		"receiver_id":      payload.ReceiverID,
		"amount":           payload.Amount,
		"transaction_id":   walletResp.GetTransactionId(),
		"sender_balance":   walletResp.GetSenderBalance(),
		"receiver_balance": walletResp.GetReceiverBalance(),
	})

	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "transfer executed and recorded successfully",
		Data: map[string]any{
			"transaction_id":   walletResp.GetTransactionId(),
			"sender_balance":   walletResp.GetSenderBalance(),
			"receiver_balance": walletResp.GetReceiverBalance(),
			"sender_id":        payload.SenderID,
			"receiver_id":      payload.ReceiverID,
			"amount":           payload.Amount,
		},
	})
}

// GetHistory godoc
// @Summary Get transaction history
// @Description Returns the transaction history for the authenticated user. The JWT subject must match the path user ID.
// @Tags transfers
// @Produce json
// @Security BearerAuth
// @Param userID path string true "User ID"
// @Success 200 {object} GetHistoryResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /history/{userID} [get]
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

	records := make([]TransactionRecordDTO, 0, len(resp.GetRecords()))
	for _, record := range resp.GetRecords() {
		records = append(records, app.mapTransactionRecordToHistoryDTO(userID, record))
	}
	app.enrichHistoryRecordsWithCounterparties(ctx, records)

	_ = app.writeJSON(w, http.StatusOK, jsonResponse{
		Error:   false,
		Message: "ok",
		Data: map[string]any{
			"user_id": userID,
			"records": records,
		},
	})
}

// GetUser godoc
// @Summary Get user by ID
// @Description Reads user profile data from user-service.
// @Tags users
// @Produce json
// @Param userID path string true "User ID"
// @Success 200 {object} GetUserResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 504 {object} ErrorResponse
// @Router /users/{userID} [get]
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

// UserExists godoc
// @Summary Check whether a user exists
// @Description Queries user-service for existence of a user ID.
// @Tags users
// @Produce json
// @Param userID path string true "User ID"
// @Success 200 {object} UserExistsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 504 {object} ErrorResponse
// @Router /users/{userID}/exists [get]
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

func (app *Config) issueRefreshToken(userID, name, email string) (string, error) {
	if app.refreshManager == nil {
		return "", errors.New("refresh jwt manager is not configured")
	}

	return app.refreshManager.Generate(security.JWTUser{
		Subject: userID,
		Name:    name,
		Email:   email,
	})
}

func (app *Config) issueTokenPair(userID, name, email string) (map[string]any, error) {
	accessToken, err := app.issueJWT(userID, name, email)
	if err != nil {
		return nil, err
	}

	refreshToken, err := app.issueRefreshToken(userID, name, email)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"token":         accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int64(app.accessTokenTTL / time.Second),
		"user": map[string]any{
			"user_id": userID,
			"name":    name,
			"email":   email,
		},
	}, nil
}

func (app *Config) createWalletForUser(parent context.Context, userID string) (int, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	if _, err := app.walletClient.CreateWallet(ctx, &walletpb.CreateWalletRequest{
		UserId: userID,
	}); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return http.StatusGatewayTimeout, status.Error(codes.DeadlineExceeded, "wallet provisioning timed out")
		}
		return mapWalletGrpcErrorStatus(err), err
	}

	return 0, nil
}

func (app *Config) deleteUserAfterWalletFailure(parent context.Context, userID string) error {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	_, err := app.userClient.DeleteUser(ctx, &userpb.DeleteUserRequest{
		UserId: userID,
	})
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return status.Error(codes.DeadlineExceeded, "rollback delete user timed out")
		}
		return err
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "rollback delete user timed out")
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

func (app *Config) recordTransaction(payload transferRequest, walletResp *walletpb.TransferResponse) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := app.transactionClient.Record(ctx, &transactionpb.RecordRequest{
		SenderId:             payload.SenderID,
		ReceiverId:           payload.ReceiverID,
		Amount:               payload.Amount,
		IdempotencyKey:       payload.IdempotencyKey,
		TransactionId:        walletResp.GetTransactionId(),
		SenderBalanceAfter:   walletResp.GetSenderBalance(),
		ReceiverBalanceAfter: walletResp.GetReceiverBalance(),
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
