package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

type mockRow struct {
	scanFn func(dest ...any) error
}

func (m mockRow) Scan(dest ...any) error {
	return m.scanFn(dest...)
}

type mockQueryRower struct {
	queryRowFn func(ctx context.Context, query string, args ...any) RowScanner
	calls      int
}

func (m *mockQueryRower) QueryRowContext(ctx context.Context, query string, args ...any) RowScanner {
	m.calls++
	return m.queryRowFn(ctx, query, args...)
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
			return mockRow{scanFn: func(dest ...any) error {
				return sql.ErrNoRows
			}}
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
			return mockRow{scanFn: func(dest ...any) error {
				return errors.New("db read error")
			}}
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
