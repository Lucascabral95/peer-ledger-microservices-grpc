package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

var (
	ErrWalletNotFound             = errors.New("wallet not found")
	ErrInsufficientFunds          = errors.New("insufficient funds")
	ErrIdempotencyPayloadMismatch = errors.New("idempotency key reused with different payload")
	ErrInvalidTransferInput       = errors.New("invalid transfer input")
	ErrInvalidWalletInput         = errors.New("invalid wallet input")
	errIdempotencyKeyConflict     = errors.New("idempotency key conflict")
)

type TransferInput struct {
	SenderID       string
	ReceiverID     string
	AmountCents    int64
	IdempotencyKey string
}

type TransferResult struct {
	TransactionID        string
	SenderBalanceCents   int64
	ReceiverBalanceCents int64
}

type TopUpSummary struct {
	UserID                string
	TopUpCountTotal       int64
	TopUpAmountTotalCents int64
	TopUpCountToday       int64
	TopUpAmountTodayCents int64
}

type TopUpRecord struct {
	TopUpID           string
	UserID            string
	AmountCents       int64
	BalanceAfterCents int64
	CreatedAt         time.Time
}

type WalletStore interface {
	GetBalance(ctx context.Context, userID string) (int64, error)
	CreateWallet(ctx context.Context, userID string, initialBalanceCents int64) (int64, error)
	TopUp(ctx context.Context, userID string, amountCents int64) (int64, error)
	GetTopUpSummary(ctx context.Context, userID, timezone string) (TopUpSummary, error)
	ListTopUps(ctx context.Context, userID string, from, to *time.Time, limit int) ([]TopUpRecord, bool, error)
	Transfer(ctx context.Context, input TransferInput) (TransferResult, error)
}

type RowScanner interface {
	Scan(dest ...any) error
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type Tx interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Commit() error
	Rollback() error
}

type TxStarter interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
}

type WalletRepository struct {
	db TxStarter
}

type sqlTxStarter struct {
	db *sql.DB
}

type sqlTx struct {
	tx *sql.Tx
}

func (s sqlTxStarter) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s sqlTxStarter) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

func (s sqlTxStarter) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return sqlTx{tx: tx}, nil
}

func (t sqlTx) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t sqlTx) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t sqlTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t sqlTx) Commit() error {
	return t.tx.Commit()
}

func (t sqlTx) Rollback() error {
	return t.tx.Rollback()
}

func NewWalletRepository(starter TxStarter) (*WalletRepository, error) {
	if starter == nil {
		return nil, errors.New("wallet tx starter cannot be nil")
	}
	return &WalletRepository{db: starter}, nil
}

func NewWalletRepositoryFromSQLDB(db *sql.DB) (*WalletRepository, error) {
	if db == nil {
		return nil, errors.New("wallet db cannot be nil")
	}
	return NewWalletRepository(sqlTxStarter{db: db})
}

func (r *WalletRepository) GetBalance(ctx context.Context, userID string) (int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, ErrWalletNotFound
	}

	const query = `
		SELECT balance::text
		FROM wallets
		WHERE user_id = $1
	`

	var balanceText string
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&balanceText)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrWalletNotFound
		}
		return 0, err
	}

	balanceCents, err := decimalStringToCents(balanceText)
	if err != nil {
		return 0, err
	}

	return balanceCents, nil
}

func (r *WalletRepository) CreateWallet(ctx context.Context, userID string, initialBalanceCents int64) (int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, fmt.Errorf("%w: user_id is required", ErrInvalidWalletInput)
	}
	if initialBalanceCents < 0 {
		return 0, fmt.Errorf("%w: initial balance cannot be negative", ErrInvalidWalletInput)
	}

	const insertQuery = `
		INSERT INTO wallets (user_id, balance)
		VALUES ($1, $2::numeric(12,2))
		ON CONFLICT (user_id) DO NOTHING
	`

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, insertQuery, userID, centsToDecimalString(initialBalanceCents)); err != nil {
		return 0, err
	}

	balanceCents, err := getLockedBalance(ctx, tx, userID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true

	return balanceCents, nil
}

func (r *WalletRepository) TopUp(ctx context.Context, userID string, amountCents int64) (int64, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, fmt.Errorf("%w: user_id is required", ErrInvalidWalletInput)
	}
	if amountCents <= 0 {
		return 0, fmt.Errorf("%w: amount must be greater than zero", ErrInvalidWalletInput)
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	currentBalance, err := getLockedBalance(ctx, tx, userID)
	if err != nil {
		return 0, err
	}

	newBalance := currentBalance + amountCents
	if err := updateWalletBalance(ctx, tx, userID, newBalance); err != nil {
		return 0, err
	}
	if err := insertTopUpRecord(ctx, tx, userID, amountCents, newBalance); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true

	return newBalance, nil
}

func (r *WalletRepository) GetTopUpSummary(ctx context.Context, userID, timezone string) (TopUpSummary, error) {
	userID = strings.TrimSpace(userID)
	timezone = strings.TrimSpace(timezone)
	if userID == "" {
		return TopUpSummary{}, fmt.Errorf("%w: user_id is required", ErrInvalidWalletInput)
	}
	if timezone == "" {
		return TopUpSummary{}, fmt.Errorf("%w: timezone is required", ErrInvalidWalletInput)
	}

	const query = `
		SELECT
			COALESCE(COUNT(*), 0)::bigint,
			COALESCE(SUM(amount), 0)::text,
			COALESCE(COUNT(*) FILTER (
				WHERE (created_at AT TIME ZONE $2)::date = (CURRENT_TIMESTAMP AT TIME ZONE $2)::date
			), 0)::bigint,
			COALESCE(SUM(amount) FILTER (
				WHERE (created_at AT TIME ZONE $2)::date = (CURRENT_TIMESTAMP AT TIME ZONE $2)::date
			), 0)::text
		FROM wallet_topups
		WHERE user_id = $1
	`

	var (
		countTotal      int64
		amountTotalText string
		countToday      int64
		amountTodayText string
	)

	if err := r.db.QueryRowContext(ctx, query, userID, timezone).Scan(
		&countTotal,
		&amountTotalText,
		&countToday,
		&amountTodayText,
	); err != nil {
		return TopUpSummary{}, err
	}

	amountTotalCents, err := decimalStringToCents(amountTotalText)
	if err != nil {
		return TopUpSummary{}, err
	}
	amountTodayCents, err := decimalStringToCents(amountTodayText)
	if err != nil {
		return TopUpSummary{}, err
	}

	return TopUpSummary{
		UserID:                userID,
		TopUpCountTotal:       countTotal,
		TopUpAmountTotalCents: amountTotalCents,
		TopUpCountToday:       countToday,
		TopUpAmountTodayCents: amountTodayCents,
	}, nil
}

func (r *WalletRepository) ListTopUps(ctx context.Context, userID string, from, to *time.Time, limit int) ([]TopUpRecord, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, false, fmt.Errorf("%w: user_id is required", ErrInvalidWalletInput)
	}
	if limit <= 0 {
		return nil, false, fmt.Errorf("%w: limit must be greater than zero", ErrInvalidWalletInput)
	}

	args := []any{userID}
	conditions := []string{"user_id = $1"}

	if from != nil {
		args = append(args, from.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)))
	}
	if to != nil {
		args = append(args, to.UTC())
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", len(args)))
	}

	args = append(args, limit+1)
	query := fmt.Sprintf(`
		SELECT id, user_id, amount::text, balance_after::text, created_at
		FROM wallet_topups
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d
	`, strings.Join(conditions, " AND "), len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	records := make([]TopUpRecord, 0, limit+1)
	for rows.Next() {
		var (
			topUpID          string
			rowUserID        string
			amountText       string
			balanceAfterText string
			createdAt        time.Time
		)

		if err := rows.Scan(&topUpID, &rowUserID, &amountText, &balanceAfterText, &createdAt); err != nil {
			return nil, false, err
		}

		amountCents, err := decimalStringToCents(amountText)
		if err != nil {
			return nil, false, err
		}
		balanceAfterCents, err := decimalStringToCents(balanceAfterText)
		if err != nil {
			return nil, false, err
		}

		records = append(records, TopUpRecord{
			TopUpID:           topUpID,
			UserID:            rowUserID,
			AmountCents:       amountCents,
			BalanceAfterCents: balanceAfterCents,
			CreatedAt:         createdAt.UTC(),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}

	return records, hasMore, nil
}

func (r *WalletRepository) Transfer(ctx context.Context, input TransferInput) (TransferResult, error) {
	if err := validateTransferInput(input); err != nil {
		return TransferResult{}, err
	}

	cached, found, err := r.getIdempotencyResult(ctx, input.IdempotencyKey)
	if err != nil {
		return TransferResult{}, err
	}
	if found {
		if !isSamePayload(cached, input) {
			return TransferResult{}, ErrIdempotencyPayloadMismatch
		}
		return cached.toResult(), nil
	}

	result, err := r.executeTransferTx(ctx, input)
	if err == nil {
		return result, nil
	}

	if errors.Is(err, errIdempotencyKeyConflict) {
		cachedAfterConflict, foundAfterConflict, getErr := r.getIdempotencyResult(ctx, input.IdempotencyKey)
		if getErr != nil {
			return TransferResult{}, getErr
		}
		if !foundAfterConflict {
			return TransferResult{}, err
		}
		if !isSamePayload(cachedAfterConflict, input) {
			return TransferResult{}, ErrIdempotencyPayloadMismatch
		}
		return cachedAfterConflict.toResult(), nil
	}

	return TransferResult{}, err
}

func getLockedBalance(ctx context.Context, tx Tx, userID string) (int64, error) {
	const query = `
		SELECT balance::text
		FROM wallets
		WHERE user_id = $1
		FOR UPDATE
	`

	var balanceText string
	if err := tx.QueryRowContext(ctx, query, userID).Scan(&balanceText); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrWalletNotFound
		}
		return 0, err
	}

	return decimalStringToCents(balanceText)
}

func (r *WalletRepository) executeTransferTx(ctx context.Context, input TransferInput) (TransferResult, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return TransferResult{}, err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	lockedBalances, err := lockWalletBalances(ctx, tx, input.SenderID, input.ReceiverID)
	if err != nil {
		return TransferResult{}, err
	}

	senderBalance, okSender := lockedBalances[input.SenderID]
	receiverBalance, okReceiver := lockedBalances[input.ReceiverID]
	if !okSender || !okReceiver {
		return TransferResult{}, ErrWalletNotFound
	}

	if senderBalance < input.AmountCents {
		return TransferResult{}, ErrInsufficientFunds
	}

	newSenderBalance := senderBalance - input.AmountCents
	newReceiverBalance := receiverBalance + input.AmountCents

	if err := updateWalletBalance(ctx, tx, input.SenderID, newSenderBalance); err != nil {
		return TransferResult{}, err
	}
	if err := updateWalletBalance(ctx, tx, input.ReceiverID, newReceiverBalance); err != nil {
		return TransferResult{}, err
	}

	transactionID, err := generateTransactionID(ctx, tx)
	if err != nil {
		return TransferResult{}, err
	}

	record := idempotencyRecord{
		SenderID:             input.SenderID,
		ReceiverID:           input.ReceiverID,
		AmountCents:          input.AmountCents,
		TransactionID:        transactionID,
		SenderBalanceCents:   newSenderBalance,
		ReceiverBalanceCents: newReceiverBalance,
	}

	if err := insertIdempotencyResult(ctx, tx, input.IdempotencyKey, record); err != nil {
		if isUniqueViolation(err) {
			return TransferResult{}, errIdempotencyKeyConflict
		}
		return TransferResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return TransferResult{}, err
	}
	committed = true

	return record.toResult(), nil
}

func lockWalletBalances(ctx context.Context, tx Tx, senderID, receiverID string) (map[string]int64, error) {
	const query = `
		SELECT user_id, balance::text
		FROM wallets
		WHERE user_id IN ($1, $2)
		ORDER BY user_id
		FOR UPDATE
	`

	rows, err := tx.QueryContext(ctx, query, senderID, receiverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	balances := make(map[string]int64, 2)
	for rows.Next() {
		var userID, balanceText string
		if scanErr := rows.Scan(&userID, &balanceText); scanErr != nil {
			return nil, scanErr
		}

		balanceCents, convErr := decimalStringToCents(balanceText)
		if convErr != nil {
			return nil, convErr
		}
		balances[userID] = balanceCents
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return balances, nil
}

func updateWalletBalance(ctx context.Context, tx Tx, userID string, newBalanceCents int64) error {
	const query = `
		UPDATE wallets
		SET balance = $1::numeric(12,2)
		WHERE user_id = $2
	`

	newBalanceDecimal := centsToDecimalString(newBalanceCents)
	result, err := tx.ExecContext(ctx, query, newBalanceDecimal, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrWalletNotFound
	}

	return nil
}

func insertTopUpRecord(ctx context.Context, tx Tx, userID string, amountCents, balanceAfterCents int64) error {
	const query = `
		INSERT INTO wallet_topups (user_id, amount, balance_after)
		VALUES ($1, $2::numeric(12,2), $3::numeric(12,2))
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		userID,
		centsToDecimalString(amountCents),
		centsToDecimalString(balanceAfterCents),
	)
	return err
}

func generateTransactionID(ctx context.Context, tx Tx) (string, error) {
	const query = `SELECT gen_random_uuid()::text`

	var transactionID string
	if err := tx.QueryRowContext(ctx, query).Scan(&transactionID); err != nil {
		return "", err
	}
	return transactionID, nil
}

func insertIdempotencyResult(ctx context.Context, tx Tx, idempotencyKey string, record idempotencyRecord) error {
	const query = `
		INSERT INTO idempotency_keys (key, result)
		VALUES ($1, $2)
	`

	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, query, idempotencyKey, string(payload))
	return err
}

func validateTransferInput(input TransferInput) error {
	if strings.TrimSpace(input.SenderID) == "" || strings.TrimSpace(input.ReceiverID) == "" {
		return fmt.Errorf("%w: sender_id and receiver_id are required", ErrInvalidTransferInput)
	}
	if input.SenderID == input.ReceiverID {
		return fmt.Errorf("%w: sender_id and receiver_id must be different", ErrInvalidTransferInput)
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return fmt.Errorf("%w: idempotency_key is required", ErrInvalidTransferInput)
	}
	if input.AmountCents <= 0 {
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalidTransferInput)
	}
	return nil
}

type idempotencyRecord struct {
	SenderID             string `json:"sender_id"`
	ReceiverID           string `json:"receiver_id"`
	AmountCents          int64  `json:"amount_cents"`
	TransactionID        string `json:"transaction_id"`
	SenderBalanceCents   int64  `json:"sender_balance_cents"`
	ReceiverBalanceCents int64  `json:"receiver_balance_cents"`
}

func (r *WalletRepository) getIdempotencyResult(ctx context.Context, key string) (idempotencyRecord, bool, error) {
	const query = `
		SELECT result
		FROM idempotency_keys
		WHERE key = $1
	`

	var payload string
	err := r.db.QueryRowContext(ctx, query, key).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idempotencyRecord{}, false, nil
		}
		return idempotencyRecord{}, false, err
	}

	var record idempotencyRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return idempotencyRecord{}, false, err
	}

	return record, true, nil
}

func isSamePayload(record idempotencyRecord, input TransferInput) bool {
	return record.SenderID == input.SenderID &&
		record.ReceiverID == input.ReceiverID &&
		record.AmountCents == input.AmountCents
}

func (r idempotencyRecord) toResult() TransferResult {
	return TransferResult{
		TransactionID:        r.TransactionID,
		SenderBalanceCents:   r.SenderBalanceCents,
		ReceiverBalanceCents: r.ReceiverBalanceCents,
	}
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return string(pqErr.Code) == "23505"
}

func decimalStringToCents(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty decimal value")
	}

	sign := int64(1)
	if strings.HasPrefix(value, "-") {
		sign = -1
		value = strings.TrimPrefix(value, "-")
	}

	parts := strings.SplitN(value, ".", 2)

	wholePart := int64(0)
	for _, ch := range parts[0] {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid decimal value: %s", value)
		}
		wholePart = wholePart*10 + int64(ch-'0')
	}

	fractionPart := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		switch {
		case len(fraction) == 0:
			fractionPart = 0
		case len(fraction) == 1:
			if fraction[0] < '0' || fraction[0] > '9' {
				return 0, fmt.Errorf("invalid decimal fraction: %s", value)
			}
			fractionPart = int64(fraction[0]-'0') * 10
		default:
			if fraction[0] < '0' || fraction[0] > '9' || fraction[1] < '0' || fraction[1] > '9' {
				return 0, fmt.Errorf("invalid decimal fraction: %s", value)
			}
			fractionPart = int64(fraction[0]-'0')*10 + int64(fraction[1]-'0')
		}
	}

	return sign * (wholePart*100 + fractionPart), nil
}

func centsToDecimalString(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}

	whole := cents / 100
	fraction := cents % 100

	return fmt.Sprintf("%s%d.%02d", sign, whole, fraction)
}
