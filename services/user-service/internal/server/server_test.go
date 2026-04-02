package server

import (
	"context"
	"errors"
	"testing"

	userpb "github.com/peer-ledger/gen/user"
	"github.com/peer-ledger/services/user-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockUserRepo struct {
	getByIDFn func(ctx context.Context, id string) (*repository.User, error)
	existsFn  func(ctx context.Context, id string) (bool, error)
}

func (m mockUserRepo) GetByID(ctx context.Context, id string) (*repository.User, error) {
	return m.getByIDFn(ctx, id)
}

func (m mockUserRepo) Exists(ctx context.Context, id string) (bool, error) {
	return m.existsFn(ctx, id)
}

func TestNewUserGRPCServer_NilRepo(t *testing.T) {
	_, err := NewUserGRPCServer(nil)
	if err == nil {
		t.Fatalf("expected error for nil repo")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	srv, err := NewUserGRPCServer(mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) {
			return nil, repository.ErrUserNotFound
		},
		existsFn: func(context.Context, string) (bool, error) {
			return false, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	_, err = srv.GetUser(context.Background(), &userpb.GetUserRequest{Id: "user-404"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (%v)", status.Code(err), err)
	}
}

func TestGetUser_InternalError(t *testing.T) {
	srv, err := NewUserGRPCServer(mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) {
			return nil, errors.New("db down")
		},
		existsFn: func(context.Context, string) (bool, error) {
			return false, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	_, err = srv.GetUser(context.Background(), &userpb.GetUserRequest{Id: "user-001"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v (%v)", status.Code(err), err)
	}
}

func TestGetUser_Success(t *testing.T) {
	srv, err := NewUserGRPCServer(mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) {
			return &repository.User{
				ID:    "user-001",
				Name:  "Lucas",
				Email: "lucas@mail.com",
			}, nil
		},
		existsFn: func(context.Context, string) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	resp, err := srv.GetUser(context.Background(), &userpb.GetUserRequest{Id: "user-001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetUserId() != "user-001" {
		t.Fatalf("expected user-001, got %s", resp.GetUserId())
	}
}

func TestUserExists_InternalError(t *testing.T) {
	srv, err := NewUserGRPCServer(mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) {
			return nil, nil
		},
		existsFn: func(context.Context, string) (bool, error) {
			return false, errors.New("db down")
		},
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	_, err = srv.UserExists(context.Background(), &userpb.UserExistsRequest{UserId: "user-001"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v (%v)", status.Code(err), err)
	}
}

func TestUserExists_Success(t *testing.T) {
	srv, err := NewUserGRPCServer(mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) {
			return nil, nil
		},
		existsFn: func(context.Context, string) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}

	resp, err := srv.UserExists(context.Background(), &userpb.UserExistsRequest{UserId: "user-001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetExists() {
		t.Fatalf("expected exists=true")
	}
}
