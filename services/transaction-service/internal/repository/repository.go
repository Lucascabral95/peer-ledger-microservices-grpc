package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

var (
	ErrInvalidRecordInput         = errors.New("invalid record input")
	ErrIdempotencyPayloadMismatch = errors.New("idempotency key reused with different payload")
	ErrTransactionIDConflict      = errors.New("transaction id conflict")
	errIdempotencyKeyConflict     = errors.New("idempotency key conflict")
)

type RecordInput struct {
	TransactionID             string
	SenderID                  string
	ReceiverID                string
	AmountCents               int64
	IdempotencyKey            string
	Status                    string
	SenderBalanceAfterCents   int64
	ReceiverBalanceAfterCents int64
}

type HistoryRecord struct {
	TransactionID             string
	SenderID                  string
	ReceiverID                string
	AmountCents               int64
	Status                    string
	CreatedAt                 time.Time
	SenderBalanceAfterCents   int64
	ReceiverBalanceAfterCents int64
}

type TransferDirection int

const (
	TransferDirectionAll TransferDirection = iota
	TransferDirectionSent
	TransferDirectionReceived
)

type TransferSummary struct {
	UserID                   string
	SentTotalCents           int64
	ReceivedTotalCents       int64
	SentCountTotal           int64
	ReceivedCountTotal       int64
	SentCountToday           int64
	ReceivedCountToday       int64
	SentAmountTodayCents     int64
	ReceivedAmountTodayCents int64
}

type TransferDirection int

const (
	TransferDirectionAll TransferDirection = iota
	TransferDirectionSent
	TransferDirectionReceived
)

type TransferSummary struct {
	UserID                   string
	SentTotalCents           int64
	ReceivedTotalCents       int64
	SentCountTotal           int64
	ReceivedCountTotal       int64
	SentCountToday           int64
	ReceivedCountToday       int64
	SentAmountTodayCents     int64
	ReceivedAmountTodayCents int64
}

type TransactionStore interface {
	Record(ctx context.Context, input RecordInput) error
	GetHistory(ctx context.Context, userID string) ([]HistoryRecord, error)
	GetTransferSummary(ctx context.Context, userID, timezone string) (TransferSummary, error)
	ListTransfers(ctx context.Context, userID string, direction TransferDirection, from, to *time.Time, limit int) ([]HistoryRecord, bool, error)
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

type TransactionRepository struct {
	db TxStarter
}

type sqlTxStarter struct {
	db *sql.DB
}

type sqlTx struct {
	tx *sql.Tx
}

type storedRecord struct {
	TransactionID             string
	SenderID                  string
	ReceiverID                string
	AmountCents               int64
	IdempotencyKey            string
	Status                    string
	CreatedAt                 time.Time
	SenderBalanceAfterCents   int64
	ReceiverBalanceAfterCents int64
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

func NewTransactionRepository(starter TxStarter) (*TransactionRepository, error) {
	if starter == nil {
		return nil, errors.New("transaction tx starter cannot be nil")
	}
	return &TransactionRepository{db: starter}, nil  
}

func NewTransactionRepositoryFromSQLDB(db *sql.DB) (*TransactionRepository, error) {
	if db == nil {
		return nil, errors.New("transaction db cannot be nil")
	}
	return NewTransactionRepository(sqlTxStarter{db: db})
}

func (r *TransactionRepository) Record(ctx context.Context, input RecordInput) error {
	normalized, err := normalizeRecordInput(input)
	if err != nil {
		return err
	}

	cached, found, err := r.getByIdempotencyKey(ctx, normalized.IdempotencyKey)
	if err != nil {
		return err
	}
	if found {
		if !sameStoredRecordAndInput(cached, normalized) {
			return ErrIdempotencyPayloadMismatch
		}
		return nil
	}

	err = r.insertRecordTx(ctx, normalized)
	if err == nil {
		return nil
	}

	if errors.Is(err, errIdempotencyKeyConflict) {
		cachedAfterConflict, foundAfterConflict, getErr := r.getByIdempotencyKey(ctx, normalized.IdempotencyKey)
		if getErr != nil {
			return getErr
		}
		if !foundAfterConflict {
			return err
		}
		if !sameStoredRecordAndInput(cachedAfterConflict, normalized) {
			return ErrIdempotencyPayloadMismatch
		}
		return nil
	}

	if errors.Is(err, ErrTransactionIDConflict) {
		stored, foundByID, getErr := r.getByTransactionID(ctx, normalized.TransactionID)
		if getErr != nil {
			return getErr
		}
		if !foundByID {
			return err
		}
		if sameStoredRecordAndInput(stored, normalized) {
			return nil
		}
		return ErrTransactionIDConflict
	}

	return err
}

func (r *TransactionRepository) GetHistory(ctx context.Context, userID string) ([]HistoryRecord, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidRecordInput)
	}

	const query = `
		SELECT
			id,
			sender_id,
			receiver_id,
			amount::text,
			status,
			created_at,
			COALESCE(sender_balance_after, 0)::text,
			COALESCE(receiver_balance_after, 0)::text
		FROM transactions
		WHERE sender_id = $1 OR receiver_id = $1
		ORDER BY created_at DESC, id DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]HistoryRecord, 0)
	for rows.Next() {
		var (
			transactionID            string
			senderID                 string
			receiverID               string
			amountText               string
			statusValue              string
			createdAt                time.Time
			senderBalanceAfterText   string
			receiverBalanceAfterText string
		)

		if err := rows.Scan(
			&transactionID,
			&senderID,
			&receiverID,
			&amountText,
			&statusValue,
			&createdAt,
			&senderBalanceAfterText,
			&receiverBalanceAfterText,
		); err != nil {
			return nil, err
		}

		amountCents, err := decimalStringToCents(amountText)
		if err != nil {
			return nil, err
		}
		senderBalanceAfterCents, err := decimalStringToCents(senderBalanceAfterText)
		if err != nil {
			return nil, err
		}
		receiverBalanceAfterCents, err := decimalStringToCents(receiverBalanceAfterText)
		if err != nil {
			return nil, err
		}

		records = append(records, HistoryRecord{
			TransactionID:             transactionID,
			SenderID:                  senderID,
			ReceiverID:                receiverID,
			AmountCents:               amountCents,
			Status:                    statusValue,
			CreatedAt:                 createdAt.UTC(),
			SenderBalanceAfterCents:   senderBalanceAfterCents,
			ReceiverBalanceAfterCents: receiverBalanceAfterCents,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

func (r *TransactionRepository) GetTransferSummary(ctx context.Context, userID, timezone string) (TransferSummary, error) {
	userID = strings.TrimSpace(userID)
	timezone = strings.TrimSpace(timezone)
	if userID == "" {
		return TransferSummary{}, fmt.Errorf("%w: user_id is required", ErrInvalidRecordInput)
	}
	if timezone == "" {
		return TransferSummary{}, fmt.Errorf("%w: timezone is required", ErrInvalidRecordInput)
	}

	const query = `
		SELECT
			COALESCE(SUM(CASE WHEN sender_id = $1 THEN amount ELSE 0 END), 0)::text,
			COALESCE(SUM(CASE WHEN receiver_id = $1 THEN amount ELSE 0 END), 0)::text,
			COALESCE(SUM(CASE WHEN sender_id = $1 THEN 1 ELSE 0 END), 0)::bigint,
			COALESCE(SUM(CASE WHEN receiver_id = $1 THEN 1 ELSE 0 END), 0)::bigint,
			COALESCE(SUM(CASE WHEN sender_id = $1 AND (created_at AT TIME ZONE $2)::date = (CURRENT_TIMESTAMP AT TIME ZONE $2)::date THEN 1 ELSE 0 END), 0)::bigint,
			COALESCE(SUM(CASE WHEN receiver_id = $1 AND (created_at AT TIME ZONE $2)::date = (CURRENT_TIMESTAMP AT TIME ZONE $2)::date THEN 1 ELSE 0 END), 0)::bigint,
			COALESCE(SUM(CASE WHEN sender_id = $1 AND (created_at AT TIME ZONE $2)::date = (CURRENT_TIMESTAMP AT TIME ZONE $2)::date THEN amount ELSE 0 END), 0)::text,
			COALESCE(SUM(CASE WHEN receiver_id = $1 AND (created_at AT TIME ZONE $2)::date = (CURRENT_TIMESTAMP AT TIME ZONE $2)::date THEN amount ELSE 0 END), 0)::text
		FROM transactions
		WHERE sender_id = $1 OR receiver_id = $1
	`

	var (
		sentTotalText           string
		receivedTotalText       string
		sentCountTotal          int64
		receivedCountTotal      int64
		sentCountToday          int64
		receivedCountToday      int64
		sentAmountTodayText     string
		receivedAmountTodayText string
	)

	if err := r.db.QueryRowContext(ctx, query, userID, timezone).Scan(
		&sentTotalText,
		&receivedTotalText,
		&sentCountTotal,
		&receivedCountTotal,
		&sentCountToday,
		&receivedCountToday,
		&sentAmountTodayText,
		&receivedAmountTodayText,
	); err != nil {
		return TransferSummary{}, err
	}

	sentTotalCents, err := decimalStringToCents(sentTotalText)
	if err != nil {
		return TransferSummary{}, err
	}
	receivedTotalCents, err := decimalStringToCents(receivedTotalText)
	if err != nil {
		return TransferSummary{}, err
	}
	sentAmountTodayCents, err := decimalStringToCents(sentAmountTodayText)
	if err != nil {
		return TransferSummary{}, err
	}
	receivedAmountTodayCents, err := decimalStringToCents(receivedAmountTodayText)
	if err != nil {
		return TransferSummary{}, err
	}

	return TransferSummary{
		UserID:                   userID,
		SentTotalCents:           sentTotalCents,
		ReceivedTotalCents:       receivedTotalCents,
		SentCountTotal:           sentCountTotal,
		ReceivedCountTotal:       receivedCountTotal,
		SentCountToday:           sentCountToday,
		ReceivedCountToday:       receivedCountToday,
		SentAmountTodayCents:     sentAmountTodayCents,
		ReceivedAmountTodayCents: receivedAmountTodayCents,
	}, nil
}

func (r *TransactionRepository) ListTransfers(ctx context.Context, userID string, direction TransferDirection, from, to *time.Time, limit int) ([]HistoryRecord, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, false, fmt.Errorf("%w: user_id is required", ErrInvalidRecordInput)
	}
	if limit <= 0 {
		return nil, false, fmt.Errorf("%w: limit must be greater than zero", ErrInvalidRecordInput)
	}

	args := []any{userID}
	conditions := []string{}

	switch direction {
	case TransferDirectionSent:
		conditions = append(conditions, "sender_id = $1")
	case TransferDirectionReceived:
		conditions = append(conditions, "receiver_id = $1")
	default:
		conditions = append(conditions, "(sender_id = $1 OR receiver_id = $1)")
	}

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
		SELECT
			id,
			sender_id,
			receiver_id,
			amount::text,
			status,
			created_at,
			COALESCE(sender_balance_after, 0)::text,
			COALESCE(receiver_balance_after, 0)::text
		FROM transactions
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d
	`, strings.Join(conditions, " AND "), len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	records := make([]HistoryRecord, 0, limit+1)
	for rows.Next() {
		var (
			transactionID            string
			senderID                 string
			receiverID               string
			amountText               string
			statusValue              string
			createdAt                time.Time
			senderBalanceAfterText   string
			receiverBalanceAfterText string
		)

		if err := rows.Scan(
			&transactionID,
			&senderID,
			&receiverID,
			&amountText,
			&statusValue,
			&createdAt,
			&senderBalanceAfterText,
			&receiverBalanceAfterText,
		); err != nil {
			return nil, false, err
		}

		amountCents, err := decimalStringToCents(amountText)
		if err != nil {
			return nil, false, err
		}
		senderBalanceAfterCents, err := decimalStringToCents(senderBalanceAfterText)
		if err != nil {
			return nil, false, err
		}
		receiverBalanceAfterCents, err := decimalStringToCents(receiverBalanceAfterText)
		if err != nil {
			return nil, false, err
		}

		records = append(records, HistoryRecord{
			TransactionID:             transactionID,
			SenderID:                  senderID,
			ReceiverID:                receiverID,
			AmountCents:               amountCents,
			Status:                    statusValue,
			CreatedAt:                 createdAt.UTC(),
			SenderBalanceAfterCents:   senderBalanceAfterCents,
			ReceiverBalanceAfterCents: receiverBalanceAfterCents,
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

func (r *TransactionRepository) insertRecordTx(ctx context.Context, input RecordInput) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	const query = `
		INSERT INTO transactions (
			id,
			sender_id,
			receiver_id,
			amount,
			idempotency_key,
			status,
			sender_balance_after,
			receiver_balance_after
		) VALUES ($1, $2, $3, $4::numeric(12,2), $5, $6, $7::numeric(12,2), $8::numeric(12,2))
	`

	_, err = tx.ExecContext(
		ctx,
		query,
		input.TransactionID,
		input.SenderID,
		input.ReceiverID,
		centsToDecimalString(input.AmountCents),
		input.IdempotencyKey,
		input.Status,
		centsToDecimalString(input.SenderBalanceAfterCents),
		centsToDecimalString(input.ReceiverBalanceAfterCents),
	)
	if err != nil {
		switch {
		case isUniqueViolationOnConstraint(err, "transactions_idempotency_key_key"):
			return errIdempotencyKeyConflict
		case isUniqueViolationOnConstraint(err, "transactions_pkey"):
			return ErrTransactionIDConflict
		case isUniqueViolation(err):
			return ErrTransactionIDConflict
		default:
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (r *TransactionRepository) getByIdempotencyKey(ctx context.Context, key string) (storedRecord, bool, error) {
	const query = `
		SELECT
			id,
			sender_id,
			receiver_id,
			amount::text,
			idempotency_key,
			status,
			created_at,
			COALESCE(sender_balance_after, 0)::text,
			COALESCE(receiver_balance_after, 0)::text
		FROM transactions
		WHERE idempotency_key = $1
	`

	return scanStoredRecord(
		r.db.QueryRowContext(ctx, query, key),
	)
}

func (r *TransactionRepository) getByTransactionID(ctx context.Context, transactionID string) (storedRecord, bool, error) {
	const query = `
		SELECT
			id,
			sender_id,
			receiver_id,
			amount::text,
			idempotency_key,
			status,
			created_at,
			COALESCE(sender_balance_after, 0)::text,
			COALESCE(receiver_balance_after, 0)::text
		FROM transactions
		WHERE id = $1
	`

	return scanStoredRecord(
		r.db.QueryRowContext(ctx, query, transactionID),
	)
}

func scanStoredRecord(row RowScanner) (storedRecord, bool, error) {
	var (
		record                   storedRecord
		amountText               string
		senderBalanceAfterText   string
		receiverBalanceAfterText string
	)

	err := row.Scan(
		&record.TransactionID,
		&record.SenderID,
		&record.ReceiverID,
		&amountText,
		&record.IdempotencyKey,
		&record.Status,
		&record.CreatedAt,
		&senderBalanceAfterText,
		&receiverBalanceAfterText,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedRecord{}, false, nil
		}
		return storedRecord{}, false, err
	}

	amountCents, err := decimalStringToCents(amountText)
	if err != nil {
		return storedRecord{}, false, err
	}
	record.AmountCents = amountCents
	senderBalanceAfterCents, err := decimalStringToCents(senderBalanceAfterText)
	if err != nil {
		return storedRecord{}, false, err
	}
	receiverBalanceAfterCents, err := decimalStringToCents(receiverBalanceAfterText)
	if err != nil {
		return storedRecord{}, false, err
	}
	record.SenderBalanceAfterCents = senderBalanceAfterCents
	record.ReceiverBalanceAfterCents = receiverBalanceAfterCents
	record.CreatedAt = record.CreatedAt.UTC()

	return record, true, nil
}

func normalizeRecordInput(input RecordInput) (RecordInput, error) {
	input.TransactionID = strings.TrimSpace(input.TransactionID)
	input.SenderID = strings.TrimSpace(input.SenderID)
	input.ReceiverID = strings.TrimSpace(input.ReceiverID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = "completed"
	}

	if input.TransactionID == "" {
		return RecordInput{}, fmt.Errorf("%w: transaction_id is required", ErrInvalidRecordInput)
	}
	if input.SenderID == "" || input.ReceiverID == "" {
		return RecordInput{}, fmt.Errorf("%w: sender_id and receiver_id are required", ErrInvalidRecordInput)
	}
	if input.SenderID == input.ReceiverID {
		return RecordInput{}, fmt.Errorf("%w: sender_id and receiver_id must be different", ErrInvalidRecordInput)
	}
	if input.IdempotencyKey == "" {
		return RecordInput{}, fmt.Errorf("%w: idempotency_key is required", ErrInvalidRecordInput)
	}
	if input.AmountCents <= 0 {
		return RecordInput{}, fmt.Errorf("%w: amount must be greater than zero", ErrInvalidRecordInput)
	}
	if input.SenderBalanceAfterCents < 0 || input.ReceiverBalanceAfterCents < 0 {
		return RecordInput{}, fmt.Errorf("%w: balance_after cannot be negative", ErrInvalidRecordInput)
	}

	return input, nil
}

func sameStoredRecordAndInput(record storedRecord, input RecordInput) bool {
	return record.TransactionID == input.TransactionID &&
		record.SenderID == input.SenderID &&
		record.ReceiverID == input.ReceiverID &&
		record.AmountCents == input.AmountCents &&
		record.IdempotencyKey == input.IdempotencyKey &&
		record.Status == input.Status &&
		record.SenderBalanceAfterCents == input.SenderBalanceAfterCents &&
		record.ReceiverBalanceAfterCents == input.ReceiverBalanceAfterCents
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return string(pqErr.Code) == "23505"
}

func isUniqueViolationOnConstraint(err error, constraint string) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return string(pqErr.Code) == "23505" && strings.EqualFold(pqErr.Constraint, constraint)
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
