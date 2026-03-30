package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID    string
	Name  string
	Email string
}

type UserReader interface {
	GetByID(ctx context.Context, id string) (*User, error)
	Exists(ctx context.Context, id string) (bool, error)
}

type RowScanner interface {
	Scan(dest ...any) error
}

type QueryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) RowScanner
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
