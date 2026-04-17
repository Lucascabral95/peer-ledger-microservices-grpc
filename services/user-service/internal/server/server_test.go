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
	getByIDFn    func(ctx context.Context, id string) (*repository.User, error)
	existsFn     func(ctx context.Context, id string) (bool, error)
	getByEmailFn func(ctx context.Context, email string) (*repository.User, error)
	createFn     func(ctx context.Context, input repository.CreateUserInput) (*repository.User, error)
	deleteFn     func(ctx context.Context, id string) (bool, error)
}

func (m mockUserRepo) GetByID(ctx context.Context, id string) (*repository.User, error) {
	return m.getByIDFn(ctx, id)
}

func (m mockUserRepo) Exists(ctx context.Context, id string) (bool, error) {
	return m.existsFn(ctx, id)
}

func (m mockUserRepo) GetByEmail(ctx context.Context, email string) (*repository.User, error) {
	return m.getByEmailFn(ctx, email)
}

func (m mockUserRepo) Create(ctx context.Context, input repository.CreateUserInput) (*repository.User, error) {
	return m.createFn(ctx, input)
}

func (m mockUserRepo) Delete(ctx context.Context, id string) (bool, error) {
	return m.deleteFn(ctx, id)
}

type mockPasswordHasher struct {
	hashFn    func(password string) (string, error)
	compareFn func(encodedHash, password string) (bool, error)
}

func (m mockPasswordHasher) Hash(password string) (string, error) {
	return m.hashFn(password)
}

func (m mockPasswordHasher) Compare(encodedHash, password string) (bool, error) {
	return m.compareFn(encodedHash, password)
}

func newTestServer(t *testing.T, repo mockUserRepo, hasher mockPasswordHasher) *UserGRPCServer {
	t.Helper()

	srv, err := NewUserGRPCServerWithDeps(repo, hasher, 8, func() (string, error) {
		return "user-new", nil
	})
	if err != nil {
		t.Fatalf("unexpected constructor error: %v", err)
	}
	return srv
}

func TestNewUserGRPCServer_NilRepo(t *testing.T) {
	_, err := NewUserGRPCServerWithDeps(nil, mockPasswordHasher{}, 8, nil)
	if err == nil {
		t.Fatalf("expected error for nil repo")
	}
}

func TestNewUserGRPCServer_NilHasher(t *testing.T) {
	_, err := NewUserGRPCServerWithDeps(mockUserRepo{}, nil, 8, nil)
	if err == nil {
		t.Fatalf("expected error for nil hasher")
	}
}

func TestGetUser_InvalidArgument(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{}, mockPasswordHasher{})
	_, err := srv.GetUser(context.Background(), &userpb.GetUserRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestGetUser_NotFound(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) {
			return nil, repository.ErrUserNotFound
		},
		existsFn:     func(context.Context, string) (bool, error) { return false, nil },
		getByEmailFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
		createFn:     func(context.Context, repository.CreateUserInput) (*repository.User, error) { return nil, nil },
		deleteFn:     func(context.Context, string) (bool, error) { return false, nil },
	}, mockPasswordHasher{})

	_, err := srv.GetUser(context.Background(), &userpb.GetUserRequest{Id: "user-404"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (%v)", status.Code(err), err)
	}
}

func TestGetUser_Success(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) {
			return &repository.User{ID: "user-001", Name: "Lucas", Email: "lucas@mail.com"}, nil
		},
		existsFn:     func(context.Context, string) (bool, error) { return true, nil },
		getByEmailFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
		createFn:     func(context.Context, repository.CreateUserInput) (*repository.User, error) { return nil, nil },
		deleteFn:     func(context.Context, string) (bool, error) { return false, nil },
	}, mockPasswordHasher{})

	resp, err := srv.GetUser(context.Background(), &userpb.GetUserRequest{Id: "user-001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetUserId() != "user-001" {
		t.Fatalf("expected user-001, got %s", resp.GetUserId())
	}
}

func TestUserExists_InvalidArgument(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{}, mockPasswordHasher{})
	_, err := srv.UserExists(context.Background(), &userpb.UserExistsRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
	}
}

func TestUserExists_Success(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{
		getByIDFn:    func(context.Context, string) (*repository.User, error) { return nil, nil },
		existsFn:     func(context.Context, string) (bool, error) { return true, nil },
		getByEmailFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
		createFn:     func(context.Context, repository.CreateUserInput) (*repository.User, error) { return nil, nil },
		deleteFn:     func(context.Context, string) (bool, error) { return false, nil },
	}, mockPasswordHasher{})

	resp, err := srv.UserExists(context.Background(), &userpb.UserExistsRequest{UserId: "user-001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetExists() {
		t.Fatalf("expected exists=true")
	}
}

func TestRegister_AlreadyExists(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
		existsFn:  func(context.Context, string) (bool, error) { return false, nil },
		getByEmailFn: func(context.Context, string) (*repository.User, error) {
			return nil, nil
		},
		createFn: func(context.Context, repository.CreateUserInput) (*repository.User, error) {
			return nil, repository.ErrEmailAlreadyExists
		},
		deleteFn: func(context.Context, string) (bool, error) { return false, nil },
	}, mockPasswordHasher{
		hashFn:    func(string) (string, error) { return "hash", nil },
		compareFn: func(string, string) (bool, error) { return false, nil },
	})

	_, err := srv.Register(context.Background(), &userpb.RegisterRequest{
		Name:     "Lucas",
		Email:    "lucas@mail.com",
		Password: "Password123!",
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v (%v)", status.Code(err), err)
	}
}

func TestRegister_InvalidPasswordPolicy(t *testing.T) {
	cases := []struct {
		name     string
		password string
	}{
		{name: "too short", password: "Aa1!a"},
		{name: "missing uppercase", password: "password1!"},
		{name: "missing lowercase", password: "PASSWORD1!"},
		{name: "missing number", password: "Password!"},
		{name: "missing punctuation", password: "Password1"},
	}

	for _, tc := range cases {
		srv := newTestServer(t, mockUserRepo{
			getByIDFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
			existsFn:  func(context.Context, string) (bool, error) { return false, nil },
			getByEmailFn: func(context.Context, string) (*repository.User, error) {
				return nil, nil
			},
			createFn: func(context.Context, repository.CreateUserInput) (*repository.User, error) {
				t.Fatalf("create should not be called for invalid password policy")
				return nil, nil
			},
			deleteFn: func(context.Context, string) (bool, error) { return false, nil },
		}, mockPasswordHasher{
			hashFn: func(string) (string, error) {
				t.Fatalf("hash should not be called for invalid password policy")
				return "", nil
			},
			compareFn: func(string, string) (bool, error) { return false, nil },
		})

		_, err := srv.Register(context.Background(), &userpb.RegisterRequest{
			Name:     "Lucas",
			Email:    "lucas@mail.com",
			Password: tc.password,
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("%s: expected InvalidArgument, got %v (%v)", tc.name, status.Code(err), err)
		}
		if got := status.Convert(err).Message(); got != "password must be at least 8 characters and include uppercase, lowercase, number, and punctuation" {
			t.Fatalf("%s: unexpected message %q", tc.name, got)
		}
	}
}

func TestRegister_Success(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
		existsFn:  func(context.Context, string) (bool, error) { return false, nil },
		getByEmailFn: func(context.Context, string) (*repository.User, error) {
			return nil, nil
		},
		createFn: func(_ context.Context, input repository.CreateUserInput) (*repository.User, error) {
			if input.PasswordHash != "hash" {
				t.Fatalf("expected hashed password, got %s", input.PasswordHash)
			}
			return &repository.User{ID: input.ID, Name: input.Name, Email: input.Email}, nil
		},
		deleteFn: func(context.Context, string) (bool, error) { return false, nil },
	}, mockPasswordHasher{
		hashFn:    func(string) (string, error) { return "hash", nil },
		compareFn: func(string, string) (bool, error) { return false, nil },
	})

	resp, err := srv.Register(context.Background(), &userpb.RegisterRequest{
		Name:     "Lucas",
		Email:    "Lucas@Mail.com",
		Password: "Password123!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetUserId() != "user-new" {
		t.Fatalf("expected user-new, got %s", resp.GetUserId())
	}
	if resp.GetEmail() != "lucas@mail.com" {
		t.Fatalf("expected normalized email, got %s", resp.GetEmail())
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
		existsFn:  func(context.Context, string) (bool, error) { return false, nil },
		getByEmailFn: func(context.Context, string) (*repository.User, error) {
			return nil, repository.ErrUserNotFound
		},
		createFn: func(context.Context, repository.CreateUserInput) (*repository.User, error) { return nil, nil },
		deleteFn: func(context.Context, string) (bool, error) { return false, nil },
	}, mockPasswordHasher{
		hashFn:    func(string) (string, error) { return "", nil },
		compareFn: func(string, string) (bool, error) { return false, nil },
	})

	_, err := srv.Login(context.Background(), &userpb.LoginRequest{
		Email:    "lucas@mail.com",
		Password: "Password123!",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v (%v)", status.Code(err), err)
	}
}

func TestLogin_CompareError(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
		existsFn:  func(context.Context, string) (bool, error) { return false, nil },
		getByEmailFn: func(context.Context, string) (*repository.User, error) {
			return &repository.User{ID: "user-001", Name: "Lucas", Email: "lucas@mail.com", PasswordHash: "hash"}, nil
		},
		createFn: func(context.Context, repository.CreateUserInput) (*repository.User, error) { return nil, nil },
		deleteFn: func(context.Context, string) (bool, error) { return false, nil },
	}, mockPasswordHasher{
		hashFn: func(string) (string, error) { return "", nil },
		compareFn: func(string, string) (bool, error) {
			return false, errors.New("compare failed")
		},
	})

	_, err := srv.Login(context.Background(), &userpb.LoginRequest{
		Email:    "lucas@mail.com",
		Password: "Password123!",
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal, got %v (%v)", status.Code(err), err)
	}
}

func TestLogin_Success(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{
		getByIDFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
		existsFn:  func(context.Context, string) (bool, error) { return false, nil },
		getByEmailFn: func(context.Context, string) (*repository.User, error) {
			return &repository.User{ID: "user-001", Name: "Lucas", Email: "lucas@mail.com", PasswordHash: "hash"}, nil
		},
		createFn: func(context.Context, repository.CreateUserInput) (*repository.User, error) { return nil, nil },
		deleteFn: func(context.Context, string) (bool, error) { return false, nil },
	}, mockPasswordHasher{
		hashFn: func(string) (string, error) { return "", nil },
		compareFn: func(string, string) (bool, error) {
			return true, nil
		},
	})

	resp, err := srv.Login(context.Background(), &userpb.LoginRequest{
		Email:    "lucas@mail.com",
		Password: "Password123!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetUserId() != "user-001" {
		t.Fatalf("expected user-001, got %s", resp.GetUserId())
	}
}

func TestDeleteUser_Success(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{
		getByIDFn:    func(context.Context, string) (*repository.User, error) { return nil, nil },
		existsFn:     func(context.Context, string) (bool, error) { return false, nil },
		getByEmailFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
		createFn:     func(context.Context, repository.CreateUserInput) (*repository.User, error) { return nil, nil },
		deleteFn: func(context.Context, string) (bool, error) {
			return true, nil
		},
	}, mockPasswordHasher{})

	resp, err := srv.DeleteUser(context.Background(), &userpb.DeleteUserRequest{UserId: "user-001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetDeleted() {
		t.Fatalf("expected deleted=true")
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	srv := newTestServer(t, mockUserRepo{
		getByIDFn:    func(context.Context, string) (*repository.User, error) { return nil, nil },
		existsFn:     func(context.Context, string) (bool, error) { return false, nil },
		getByEmailFn: func(context.Context, string) (*repository.User, error) { return nil, nil },
		createFn:     func(context.Context, repository.CreateUserInput) (*repository.User, error) { return nil, nil },
		deleteFn: func(context.Context, string) (bool, error) {
			return false, repository.ErrUserNotFound
		},
	}, mockPasswordHasher{})

	_, err := srv.DeleteUser(context.Background(), &userpb.DeleteUserRequest{UserId: "user-missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (%v)", status.Code(err), err)
	}
}
