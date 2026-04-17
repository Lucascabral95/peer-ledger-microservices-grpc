package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/lib/pq"
)

type mockRow struct {
	scanFn func(dest ...any) error
}

func (m mockRow) Scan(dest ...any) error {
	return m.scanFn(dest...)
}

type mockQueryRower struct {
	queryRowFn func(ctx context.Context, query string, args ...any) RowScanner
	execFn     func(ctx context.Context, query string, args ...any) (sql.Result, error)
	calls      int
}

func (m *mockQueryRower) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	m.calls++
	return m.queryRowFn(ctx, query, args...)
}

func (m *mockQueryRower) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	m.calls++
	return m.execFn(ctx, query, args...)
}

type mockSQLResult struct {
	rowsAffected int64
}

func (m mockSQLResult) LastInsertId() (int64, error) {
	return 0, errors.New("not implemented")
}

func (m mockSQLResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

func TestGetByID_EmptyID(t *testing.T) {
	mock := &mockQueryRower{
		queryRowFn: func(context.Context, string, ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error { return nil }}
		},
	}

	repo := NewUserRepository(mock)
	_, err := repo.GetByID(context.Background(), "   ")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if mock.calls != 0 {
		t.Fatalf("expected no DB call, got %d", mock.calls)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	mock := &mockQueryRower{
		queryRowFn: func(context.Context, string, ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error { return sql.ErrNoRows }}
		},
	}

	repo := NewUserRepository(mock)
	_, err := repo.GetByID(context.Background(), "user-404")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetByID_Success(t *testing.T) {
	mock := &mockQueryRower{
		queryRowFn: func(context.Context, string, ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "user-001"
				*(dest[1].(*string)) = "Lucas"
				*(dest[2].(*string)) = "lucas@mail.com"
				return nil
			}}
		},
	}

	repo := NewUserRepository(mock)
	user, err := repo.GetByID(context.Background(), "user-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "user-001" || user.Name != "Lucas" || user.Email != "lucas@mail.com" {
		t.Fatalf("unexpected user payload: %+v", user)
	}
}

func TestGetByEmail_Success(t *testing.T) {
	mock := &mockQueryRower{
		queryRowFn: func(context.Context, string, ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "user-001"
				*(dest[1].(*string)) = "Lucas"
				*(dest[2].(*string)) = "lucas@mail.com"
				*(dest[3].(*string)) = "hash"
				return nil
			}}
		},
	}

	repo := NewUserRepository(mock)
	user, err := repo.GetByEmail(context.Background(), "Lucas@Mail.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "lucas@mail.com" {
		t.Fatalf("expected normalized email, got %s", user.Email)
	}
	if user.PasswordHash != "hash" {
		t.Fatalf("expected password hash")
	}
}

func TestGetByEmail_NotFound(t *testing.T) {
	mock := &mockQueryRower{
		queryRowFn: func(context.Context, string, ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error { return sql.ErrNoRows }}
		},
	}

	repo := NewUserRepository(mock)
	_, err := repo.GetByEmail(context.Background(), "missing@mail.com")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestCreate_InvalidInput(t *testing.T) {
	repo := NewUserRepository(&mockQueryRower{})
	_, err := repo.Create(context.Background(), CreateUserInput{})
	if err == nil {
		t.Fatalf("expected invalid input error")
	}
}

func TestCreate_EmailAlreadyExists(t *testing.T) {
	mock := &mockQueryRower{
		queryRowFn: func(context.Context, string, ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				return &pq.Error{Code: "23505"}
			}}
		},
	}

	repo := NewUserRepository(mock)
	_, err := repo.Create(context.Background(), CreateUserInput{
		ID:           "user-001",
		Name:         "Lucas",
		Email:        "lucas@mail.com",
		PasswordHash: "hash",
	})
	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestCreate_Success(t *testing.T) {
	mock := &mockQueryRower{
		queryRowFn: func(context.Context, string, ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "user-010"
				*(dest[1].(*string)) = "Lucas"
				*(dest[2].(*string)) = "lucas@mail.com"
				*(dest[3].(*string)) = "hash"
				return nil
			}}
		},
	}

	repo := NewUserRepository(mock)
	user, err := repo.Create(context.Background(), CreateUserInput{
		ID:           "user-010",
		Name:         "Lucas",
		Email:        "Lucas@Mail.com",
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "user-010" || user.Email != "lucas@mail.com" {
		t.Fatalf("unexpected user payload: %+v", user)
	}
}

func TestExists_EmptyID(t *testing.T) {
	mock := &mockQueryRower{
		queryRowFn: func(context.Context, string, ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error { return nil }}
		},
	}

	repo := NewUserRepository(mock)
	exists, err := repo.Exists(context.Background(), "   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Fatalf("expected false for empty id")
	}
	if mock.calls != 0 {
		t.Fatalf("expected no DB call, got %d", mock.calls)
	}
}

func TestExists_QueryError(t *testing.T) {
	mock := &mockQueryRower{
		queryRowFn: func(context.Context, string, ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error { return errors.New("db read error") }}
		},
	}

	repo := NewUserRepository(mock)
	_, err := repo.Exists(context.Background(), "user-001")
	if err == nil {
		t.Fatalf("expected query error")
	}
}

func TestExists_Success(t *testing.T) {
	mock := &mockQueryRower{
		queryRowFn: func(context.Context, string, ...any) RowScanner {
			return mockRow{scanFn: func(dest ...any) error {
				*(dest[0].(*bool)) = true
				return nil
			}}
		},
	}

	repo := NewUserRepository(mock)
	exists, err := repo.Exists(context.Background(), "user-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatalf("expected true")
	}
}

func TestDelete_EmptyID(t *testing.T) {
	repo := NewUserRepository(&mockQueryRower{})
	deleted, err := repo.Delete(context.Background(), "   ")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if deleted {
		t.Fatalf("expected deleted=false")
	}
}

func TestDelete_NotFound(t *testing.T) {
	mock := &mockQueryRower{
		execFn: func(context.Context, string, ...any) (sql.Result, error) {
			return mockSQLResult{rowsAffected: 0}, nil
		},
	}

	repo := NewUserRepository(mock)
	deleted, err := repo.Delete(context.Background(), "user-missing")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if deleted {
		t.Fatalf("expected deleted=false")
	}
}

func TestDelete_Success(t *testing.T) {
	mock := &mockQueryRower{
		execFn: func(context.Context, string, ...any) (sql.Result, error) {
			return mockSQLResult{rowsAffected: 1}, nil
		},
	}

	repo := NewUserRepository(mock)
	deleted, err := repo.Delete(context.Background(), "user-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Fatalf("expected deleted=true")
	}
}
