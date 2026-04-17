package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	transactionpb "github.com/peer-ledger/gen/transaction"
	userpb "github.com/peer-ledger/gen/user"
	walletpb "github.com/peer-ledger/gen/wallet"
)

const (
	dashboardTimezone   = "America/Argentina/Buenos_Aires"
	defaultPageSize     = 20
	maxPageSize         = 100
	maxBackendFetchSize = 5000
	defaultActivityKind = "all"
)

type paginationRequest struct {
	Page       int
	PageSize   int
	Offset     int
	FetchLimit int
}

type timeFilters struct {
	From       *time.Time
	To         *time.Time
	FromString *string
	ToString   *string
}

type activityEnvelope struct {
	item      ActivityItem
	createdAt time.Time
}

// GetMeProfile godoc
// @Summary Get authenticated profile
// @Description Returns the profile of the authenticated user.
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} MeProfileResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 504 {object} ErrorResponse
// @Router /me/profile [get]
func (app *Config) GetMeProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		_ = app.errorJSON(w, errors.New("authentication required"), http.StatusUnauthorized)
		return
	}

	userData, err := app.fetchAuthenticatedUser(r.Context(), claims.Subject)
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapGrpcToHTTPStatus(err))
		return
	}

	_ = app.writeJSON(w, http.StatusOK, MeProfileResponse{
		Error:   false,
		Message: "ok",
		Data:    userData,
	})
}

// GetMeDashboard godoc
// @Summary Get dashboard summary
// @Description Returns an aggregated summary for the authenticated user's dashboard home.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Success 200 {object} MeDashboardResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 504 {object} ErrorResponse
// @Router /me/dashboard [get]
func (app *Config) GetMeDashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		_ = app.errorJSON(w, errors.New("authentication required"), http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	userData, err := app.fetchAuthenticatedUser(ctx, claims.Subject)
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapGrpcToHTTPStatus(err))
		return
	}

	balanceResp, err := app.walletClient.GetBalance(ctx, &walletpb.GetBalanceRequest{UserId: claims.Subject})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapWalletGrpcErrorStatus(err))
		return
	}

	topUpSummaryResp, err := app.walletClient.GetTopUpSummary(ctx, &walletpb.GetTopUpSummaryRequest{
		UserId:   claims.Subject,
		Timezone: dashboardTimezone,
	})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapWalletGrpcErrorStatus(err))
		return
	}

	transferSummaryResp, err := app.transactionClient.GetTransferSummary(ctx, &transactionpb.GetTransferSummaryRequest{
		UserId:   claims.Subject,
		Timezone: dashboardTimezone,
	})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapTransactionGrpcErrorStatus(err))
		return
	}

	recentTransferResp, err := app.transactionClient.ListTransfers(ctx, &transactionpb.ListTransfersRequest{
		UserId:    claims.Subject,
		Direction: transactionpb.TransferDirection_TRANSFER_DIRECTION_ALL,
		Limit:     5,
	})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapTransactionGrpcErrorStatus(err))
		return
	}

	recentTopUpResp, err := app.walletClient.ListTopUps(ctx, &walletpb.ListTopUpsRequest{
		UserId: claims.Subject,
		Limit:  5,
	})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapWalletGrpcErrorStatus(err))
		return
	}

	recentTransfers := make([]ActivityItem, 0, len(recentTransferResp.GetRecords()))
	for _, record := range recentTransferResp.GetRecords() {
		recentTransfers = append(recentTransfers, app.mapTransferRecordToActivityItem(claims.Subject, record))
	}

	recentTopUps := make([]TopUpItem, 0, len(recentTopUpResp.GetRecords()))
	for _, record := range recentTopUpResp.GetRecords() {
		recentTopUps = append(recentTopUps, mapTopUpRecordToTopUpItem(record))
	}

	_ = app.writeJSON(w, http.StatusOK, MeDashboardResponse{
		Error:   false,
		Message: "ok",
		Data: MeDashboardData{
			Timezone: dashboardTimezone,
			User: UserDTO{
				UserID: userData.UserID,
				Name:   userData.Name,
				Email:  userData.Email,
			},
			Wallet: DashboardWalletSummary{
				Balance: balanceResp.GetBalance(),
			},
			Transfers: DashboardTransferSummary{
				SentTotal:          transferSummaryResp.GetSentTotal(),
				ReceivedTotal:      transferSummaryResp.GetReceivedTotal(),
				SentCountTotal:     transferSummaryResp.GetSentCountTotal(),
				ReceivedCountTotal: transferSummaryResp.GetReceivedCountTotal(),
			},
			Topups: DashboardTopUpSummary{
				CountTotal:  topUpSummaryResp.GetTopupCountTotal(),
				AmountTotal: topUpSummaryResp.GetTopupAmountTotal(),
				CountToday:  topUpSummaryResp.GetTopupCountToday(),
				AmountToday: topUpSummaryResp.GetTopupAmountToday(),
			},
			ActivityToday: DashboardActivityToday{
				TransferSentCount:     transferSummaryResp.GetSentCountToday(),
				TransferReceivedCount: transferSummaryResp.GetReceivedCountToday(),
				TopUpCount:            topUpSummaryResp.GetTopupCountToday(),
				TotalEvents: transferSummaryResp.GetSentCountToday() +
					transferSummaryResp.GetReceivedCountToday() +
					topUpSummaryResp.GetTopupCountToday(),
			},
			RecentTransfers: recentTransfers,
			RecentTopUps:    recentTopUps,
		},
	})
}

// GetMeWallet godoc
// @Summary Get authenticated wallet summary
// @Description Returns the current balance and topup summary for the authenticated user.
// @Tags wallet
// @Produce json
// @Security BearerAuth
// @Success 200 {object} MeWalletResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 504 {object} ErrorResponse
// @Router /me/wallet [get]
func (app *Config) GetMeWallet(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		_ = app.errorJSON(w, errors.New("authentication required"), http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	balanceResp, err := app.walletClient.GetBalance(ctx, &walletpb.GetBalanceRequest{UserId: claims.Subject})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapWalletGrpcErrorStatus(err))
		return
	}

	topUpSummaryResp, err := app.walletClient.GetTopUpSummary(ctx, &walletpb.GetTopUpSummaryRequest{
		UserId:   claims.Subject,
		Timezone: dashboardTimezone,
	})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapWalletGrpcErrorStatus(err))
		return
	}

	_ = app.writeJSON(w, http.StatusOK, MeWalletResponse{
		Error:   false,
		Message: "ok",
		Data: MeWalletData{
			Timezone: dashboardTimezone,
			UserID:   claims.Subject,
			Balance:  balanceResp.GetBalance(),
			Topups: DashboardTopUpSummary{
				CountTotal:  topUpSummaryResp.GetTopupCountTotal(),
				AmountTotal: topUpSummaryResp.GetTopupAmountTotal(),
				CountToday:  topUpSummaryResp.GetTopupCountToday(),
				AmountToday: topUpSummaryResp.GetTopupAmountToday(),
			},
		},
	})
}

// GetMeTopUps godoc
// @Summary Get authenticated topup history
// @Description Returns paginated topup history for the authenticated user.
// @Tags wallet
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param from query string false "RFC3339 lower bound"
// @Param to query string false "RFC3339 upper bound"
// @Success 200 {object} MeTopUpsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 504 {object} ErrorResponse
// @Router /me/topups [get]
func (app *Config) GetMeTopUps(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		_ = app.errorJSON(w, errors.New("authentication required"), http.StatusUnauthorized)
		return
	}

	pagination, err := parsePaginationRequest(r)
	if err != nil {
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	filters, err := parseTimeFilters(r)
	if err != nil {
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := app.walletClient.ListTopUps(ctx, &walletpb.ListTopUpsRequest{
		UserId:        claims.Subject,
		FromCreatedAt: formatFilterTime(filters.From),
		ToCreatedAt:   formatFilterTime(filters.To),
		Limit:         int32(pagination.FetchLimit),
	})
	if err != nil {
		_ = app.errorJSON(w, mapGrpcToHTTPError(err), mapWalletGrpcErrorStatus(err))
		return
	}

	items := make([]TopUpItem, 0, len(resp.GetRecords()))
	for _, record := range resp.GetRecords() {
		items = append(items, mapTopUpRecordToTopUpItem(record))
	}

	pagedItems, hasNext := slicePage(items, pagination)

	_ = app.writeJSON(w, http.StatusOK, MeTopUpsResponse{
		Error:   false,
		Message: "ok",
		Data: MeTopUpsData{
			Timezone: dashboardTimezone,
			Items:    pagedItems,
			Pagination: PaginationMeta{
				Page:     pagination.Page,
				PageSize: pagination.PageSize,
				HasNext:  hasNext,
			},
			Filters: TopUpFilters{
				From: filters.FromString,
				To:   filters.ToString,
			},
		},
	})
}

// GetMeActivity godoc
// @Summary Get authenticated activity feed
// @Description Returns paginated activity for the authenticated user, including transfers and topups.
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param kind query string false "all|topup|transfer|transfer_sent|transfer_received" default(all)
// @Param from query string false "RFC3339 lower bound"
// @Param to query string false "RFC3339 upper bound"
// @Success 200 {object} MeActivityResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 502 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Failure 504 {object} ErrorResponse
// @Router /me/activity [get]
func (app *Config) GetMeActivity(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		_ = app.errorJSON(w, errors.New("authentication required"), http.StatusUnauthorized)
		return
	}

	pagination, err := parsePaginationRequest(r)
	if err != nil {
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}
	filters, err := parseTimeFilters(r)
	if err != nil {
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}
	kind, err := parseActivityKind(r)
	if err != nil {
		_ = app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	items, err := app.fetchActivityItems(ctx, claims.Subject, kind, filters, pagination.FetchLimit)
	if err != nil {
		var statusCode int
		switch {
		case errors.Is(err, errWalletActivitySource):
			statusCode = http.StatusServiceUnavailable
		case errors.Is(err, errTransactionActivitySource):
			statusCode = http.StatusServiceUnavailable
		default:
			statusCode = http.StatusBadGateway
		}
		_ = app.errorJSON(w, unwrapActivityError(err), statusCode)
		return
	}

	pagedItems, hasNext := slicePage(items, pagination)

	_ = app.writeJSON(w, http.StatusOK, MeActivityResponse{
		Error:   false,
		Message: "ok",
		Data: MeActivityData{
			Timezone: dashboardTimezone,
			Items:    pagedItems,
			Pagination: PaginationMeta{
				Page:     pagination.Page,
				PageSize: pagination.PageSize,
				HasNext:  hasNext,
			},
			Filters: ActivityFilters{
				Kind: kind,
				From: filters.FromString,
				To:   filters.ToString,
			},
		},
	})
}

var (
	errWalletActivitySource      = errors.New("wallet activity source failed")
	errTransactionActivitySource = errors.New("transaction activity source failed")
)

func (app *Config) fetchAuthenticatedUser(ctx context.Context, userID string) (GetUserData, error) {
	resp, err := app.userClient.GetUser(ctx, &userpb.GetUserRequest{Id: userID})
	if err != nil {
		return GetUserData{}, err
	}

	return GetUserData{
		UserID: resp.GetUserId(),
		Name:   resp.GetName(),
		Email:  resp.GetEmail(),
	}, nil
}

func parsePaginationRequest(r *http.Request) (paginationRequest, error) {
	page := 1
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return paginationRequest{}, errors.New("page must be a positive integer")
		}
		page = parsed
	}

	pageSize := defaultPageSize
	if raw := strings.TrimSpace(r.URL.Query().Get("page_size")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return paginationRequest{}, errors.New("page_size must be a positive integer")
		}
		if parsed > maxPageSize {
			return paginationRequest{}, errors.New("page_size must be less than or equal to 100")
		}
		pageSize = parsed
	}

	fetchLimit64 := int64(page)*int64(pageSize) + 1
	if fetchLimit64 > maxBackendFetchSize {
		return paginationRequest{}, errors.New("requested page window is too large")
	}

	return paginationRequest{
		Page:       page,
		PageSize:   pageSize,
		Offset:     (page - 1) * pageSize,
		FetchLimit: int(fetchLimit64),
	}, nil
}

func parseTimeFilters(r *http.Request) (timeFilters, error) {
	from, fromString, err := parseOptionalRFC3339(r.URL.Query().Get("from"))
	if err != nil {
		return timeFilters{}, errors.New("from must be RFC3339")
	}
	to, toString, err := parseOptionalRFC3339(r.URL.Query().Get("to"))
	if err != nil {
		return timeFilters{}, errors.New("to must be RFC3339")
	}
	if from != nil && to != nil && from.After(*to) {
		return timeFilters{}, errors.New("from must be before or equal to to")
	}

	return timeFilters{
		From:       from,
		To:         to,
		FromString: fromString,
		ToString:   toString,
	}, nil
}

func parseOptionalRFC3339(value string) (*time.Time, *string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, nil, err
	}

	utc := parsed.UTC()
	formatted := utc.Format(time.RFC3339)
	return &utc, &formatted, nil
}

func formatFilterTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func parseActivityKind(r *http.Request) (string, error) {
	kind := strings.TrimSpace(r.URL.Query().Get("kind"))
	if kind == "" {
		return defaultActivityKind, nil
	}

	switch kind {
	case "all", "topup", "transfer", "transfer_sent", "transfer_received":
		return kind, nil
	default:
		return "", errors.New("kind must be one of all, topup, transfer, transfer_sent, transfer_received")
	}
}

func (app *Config) fetchActivityItems(ctx context.Context, userID, kind string, filters timeFilters, fetchLimit int) ([]ActivityItem, error) {
	switch kind {
	case "topup":
		topUps, err := app.fetchTopUpActivity(ctx, userID, filters, fetchLimit)
		if err != nil {
			return nil, err
		}
		return envelopesToItems(topUps), nil
	case "transfer":
		transfers, err := app.fetchTransferActivity(ctx, userID, transactionpb.TransferDirection_TRANSFER_DIRECTION_ALL, filters, fetchLimit)
		if err != nil {
			return nil, err
		}
		return envelopesToItems(transfers), nil
	case "transfer_sent":
		transfers, err := app.fetchTransferActivity(ctx, userID, transactionpb.TransferDirection_TRANSFER_DIRECTION_SENT, filters, fetchLimit)
		if err != nil {
			return nil, err
		}
		return envelopesToItems(transfers), nil
	case "transfer_received":
		transfers, err := app.fetchTransferActivity(ctx, userID, transactionpb.TransferDirection_TRANSFER_DIRECTION_RECEIVED, filters, fetchLimit)
		if err != nil {
			return nil, err
		}
		return envelopesToItems(transfers), nil
	default:
		topUps, err := app.fetchTopUpActivity(ctx, userID, filters, fetchLimit)
		if err != nil {
			return nil, err
		}
		transfers, err := app.fetchTransferActivity(ctx, userID, transactionpb.TransferDirection_TRANSFER_DIRECTION_ALL, filters, fetchLimit)
		if err != nil {
			return nil, err
		}
		combined := append(topUps, transfers...)
		sort.SliceStable(combined, func(i, j int) bool {
			if combined[i].createdAt.Equal(combined[j].createdAt) {
				return combined[i].item.ID > combined[j].item.ID
			}
			return combined[i].createdAt.After(combined[j].createdAt)
		})
		return envelopesToItems(combined), nil
	}
}

func (app *Config) fetchTopUpActivity(ctx context.Context, userID string, filters timeFilters, fetchLimit int) ([]activityEnvelope, error) {
	resp, err := app.walletClient.ListTopUps(ctx, &walletpb.ListTopUpsRequest{
		UserId:        userID,
		FromCreatedAt: formatFilterTime(filters.From),
		ToCreatedAt:   formatFilterTime(filters.To),
		Limit:         int32(fetchLimit),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errWalletActivitySource, mapGrpcToHTTPError(err))
	}

	items := make([]activityEnvelope, 0, len(resp.GetRecords()))
	for _, record := range resp.GetRecords() {
		item, createdAt := mapTopUpRecordToActivityItem(record)
		items = append(items, activityEnvelope{item: item, createdAt: createdAt})
	}
	return items, nil
}

func (app *Config) fetchTransferActivity(ctx context.Context, userID string, direction transactionpb.TransferDirection, filters timeFilters, fetchLimit int) ([]activityEnvelope, error) {
	resp, err := app.transactionClient.ListTransfers(ctx, &transactionpb.ListTransfersRequest{
		UserId:        userID,
		Direction:     direction,
		FromCreatedAt: formatFilterTime(filters.From),
		ToCreatedAt:   formatFilterTime(filters.To),
		Limit:         int32(fetchLimit),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errTransactionActivitySource, mapGrpcToHTTPError(err))
	}

	items := make([]activityEnvelope, 0, len(resp.GetRecords()))
	for _, record := range resp.GetRecords() {
		item := app.mapTransferRecordToActivityItem(userID, record)
		items = append(items, activityEnvelope{
			item:      item,
			createdAt: parseActivityTimestamp(record.GetCreatedAt()),
		})
	}
	return items, nil
}

func unwrapActivityError(err error) error {
	switch {
	case errors.Is(err, errWalletActivitySource):
		return errors.Unwrap(err)
	case errors.Is(err, errTransactionActivitySource):
		return errors.Unwrap(err)
	default:
		return err
	}
}

func envelopesToItems(envelopes []activityEnvelope) []ActivityItem {
	items := make([]ActivityItem, 0, len(envelopes))
	for _, envelope := range envelopes {
		items = append(items, envelope.item)
	}
	return items
}

func slicePage[T any](items []T, pagination paginationRequest) ([]T, bool) {
	if pagination.Offset >= len(items) {
		return []T{}, false
	}

	end := pagination.Offset + pagination.PageSize
	hasNext := len(items) > end
	if end > len(items) {
		end = len(items)
	}
	return items[pagination.Offset:end], hasNext
}

func (app *Config) mapTransferRecordToActivityItem(userID string, record *transactionpb.TransactionRecord) ActivityItem {
	kind := "transfer_received"
	counterpartyUserID := record.GetSenderId()
	if record.GetSenderId() == userID {
		kind = "transfer_sent"
		counterpartyUserID = record.GetReceiverId()
	}

	status := strings.TrimSpace(record.GetStatus())
	if status == "" {
		status = "completed"
	}

	return ActivityItem{
		ID:                 record.GetTransactionId(),
		Kind:               kind,
		Status:             status,
		Amount:             record.GetAmount(),
		CreatedAt:          record.GetCreatedAt(),
		CounterpartyUserID: counterpartyUserID,
	}
}

func mapTopUpRecordToTopUpItem(record *walletpb.TopUpRecord) TopUpItem {
	return TopUpItem{
		ID:           record.GetTopupId(),
		Kind:         "topup",
		Status:       "completed",
		Amount:       record.GetAmount(),
		BalanceAfter: record.GetBalanceAfter(),
		CreatedAt:    record.GetCreatedAt(),
	}
}

func mapTopUpRecordToActivityItem(record *walletpb.TopUpRecord) (ActivityItem, time.Time) {
	balanceAfter := record.GetBalanceAfter()
	return ActivityItem{
		ID:           record.GetTopupId(),
		Kind:         "topup",
		Status:       "completed",
		Amount:       record.GetAmount(),
		CreatedAt:    record.GetCreatedAt(),
		BalanceAfter: &balanceAfter,
	}, parseActivityTimestamp(record.GetCreatedAt())
}

func parseActivityTimestamp(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC()
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}
