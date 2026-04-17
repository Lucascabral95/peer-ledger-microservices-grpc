package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

var ErrUserNotFound = errors.New("user not found")
var ErrEmailAlreadyExists = errors.New("email already exists")

type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
}

type CreateUserInput struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
}

type UserReader interface {
	GetByID(ctx context.Context, id string) (*User, error)
	Exists(ctx context.Context, id string) (bool, error)
}

type UserStore interface {
	UserReader
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, input CreateUserInput) (*User, error)
	Delete(ctx context.Context, id string) (bool, error)
}

type RowScanner interface {
	Scan(dest ...any) error
}

type QueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type UserRepository struct {
	query QueryRower
}

type SQLQueryRower struct {
	DB *sql.DB
}

func (q SQLQueryRower) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	return q.DB.QueryRowContext(ctx, query, args...)
}

func (q SQLQueryRower) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return q.DB.ExecContext(ctx, query, args...)
}

func NewUserRepository(query QueryRower) *UserRepository {
	return &UserRepository{query: query}
}

func NewUserRepositoryFromSQLDB(db *sql.DB) *UserRepository {
	return &UserRepository{
		query: SQLQueryRower{DB: db},
	}
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrUserNotFound
	}

	const query = `
		SELECT id, name, email
		FROM users
		WHERE id = $1
	`

	row := r.query.QueryRowContext(ctx, query, id)
	user := &User{}
	if err := row.Scan(&user.ID, &user.Name, &user.Email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, ErrUserNotFound
	}

	const query = `
		SELECT id, name, email, password_hash
		FROM users
		WHERE email = $1
	`

	row := r.query.QueryRowContext(ctx, query, email)
	user := &User{}
	if err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) Exists(ctx context.Context, id string) (bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return false, nil
	}

	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM users
			WHERE id = $1
		)
	`

	var exists bool
	if err := r.query.QueryRowContext(ctx, query, id).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func (r *UserRepository) Create(ctx context.Context, input CreateUserInput) (*User, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.Name = strings.TrimSpace(input.Name)
	input.Email = normalizeEmail(input.Email)
	input.PasswordHash = strings.TrimSpace(input.PasswordHash)

	if input.ID == "" || input.Name == "" || input.Email == "" || input.PasswordHash == "" {
		return nil, fmt.Errorf("invalid create user input")
	}

	const query = `
		INSERT INTO users (id, name, email, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, email, password_hash
	`

	row := r.query.QueryRowContext(ctx, query, input.ID, input.Name, input.Email, input.PasswordHash)
	user := &User{}
	if err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, ErrEmailAlreadyExists
		}
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) (bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return false, ErrUserNotFound
	}

	const query = `
		DELETE FROM users
		WHERE id = $1
	`

	result, err := r.query.ExecContext(ctx, query, id)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected == 0 {
		return false, ErrUserNotFound
	}

	return true, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
