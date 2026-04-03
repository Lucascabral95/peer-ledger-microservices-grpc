package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"

	userpb "github.com/peer-ledger/gen/user"
	"github.com/peer-ledger/services/user-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(encodedHash, password string) (bool, error)
}

type IDGenerator func() (string, error)

type UserGRPCServer struct {
	userpb.UnimplementedUserServiceServer
	repo              repository.UserStore
	hasher            PasswordHasher
	generateID        IDGenerator
	minPasswordLength int
}

func NewUserGRPCServer(repo repository.UserStore, hasher PasswordHasher, minPasswordLength int) (*UserGRPCServer, error) {
	return NewUserGRPCServerWithDeps(repo, hasher, minPasswordLength, newUserID)
}

func NewUserGRPCServerWithDeps(repo repository.UserStore, hasher PasswordHasher, minPasswordLength int, generateID IDGenerator) (*UserGRPCServer, error) {
	if repo == nil {
		return nil, errors.New("user repository cannot be nil")
	}
	if hasher == nil {
		return nil, errors.New("password hasher cannot be nil")
	}
	if minPasswordLength < 8 {
		return nil, errors.New("minimum password length must be >= 8")
	}
	if generateID == nil {
		generateID = newUserID
	}

	return &UserGRPCServer{
		repo:              repo,
		hasher:            hasher,
		generateID:        generateID,
		minPasswordLength: minPasswordLength,
	}, nil
}

func (s *UserGRPCServer) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	if req == nil || strings.TrimSpace(req.GetId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	user, err := s.repo.GetByID(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "db read error")
	}

	return &userpb.GetUserResponse{
		UserId: user.ID,
		Name:   user.Name,
		Email:  user.Email,
	}, nil
}

func (s *UserGRPCServer) UserExists(ctx context.Context, req *userpb.UserExistsRequest) (*userpb.UserExistsResponse, error) {
	if req == nil || strings.TrimSpace(req.GetUserId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	exists, err := s.repo.Exists(ctx, req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.Internal, "db read error")
	}

	return &userpb.UserExistsResponse{Exists: exists}, nil
}

func (s *UserGRPCServer) Register(ctx context.Context, req *userpb.RegisterRequest) (*userpb.RegisterResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	name := strings.TrimSpace(req.GetName())
	email := normalizeEmail(req.GetEmail())
	password := req.GetPassword()

	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if !isValidEmail(email) {
		return nil, status.Error(codes.InvalidArgument, "email is invalid")
	}
	if len(password) < s.minPasswordLength {
		return nil, status.Errorf(codes.InvalidArgument, "password must be at least %d characters", s.minPasswordLength)
	}

	passwordHash, err := s.hasher.Hash(password)
	if err != nil {
		return nil, status.Error(codes.Internal, "password hash error")
	}

	userID, err := s.generateID()
	if err != nil {
		return nil, status.Error(codes.Internal, "user id generation error")
	}

	user, err := s.repo.Create(ctx, repository.CreateUserInput{
		ID:           userID,
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if errors.Is(err, repository.ErrEmailAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, "email already exists")
		}
		return nil, status.Error(codes.Internal, "db write error")
	}

	return &userpb.RegisterResponse{
		UserId: user.ID,
		Name:   user.Name,
		Email:  user.Email,
	}, nil
}

func (s *UserGRPCServer) Login(ctx context.Context, req *userpb.LoginRequest) (*userpb.LoginResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	email := normalizeEmail(req.GetEmail())
	password := req.GetPassword()
	if !isValidEmail(email) {
		return nil, status.Error(codes.InvalidArgument, "email is invalid")
	}
	if strings.TrimSpace(password) == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return nil, status.Error(codes.Internal, "db read error")
	}

	match, err := s.hasher.Compare(user.PasswordHash, password)
	if err != nil {
		return nil, status.Error(codes.Internal, "password verification error")
	}
	if !match {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	return &userpb.LoginResponse{
		UserId: user.ID,
		Name:   user.Name,
		Email:  user.Email,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isValidEmail(email string) bool {
	if strings.TrimSpace(email) == "" {
		return false
	}

	parsed, err := mail.ParseAddress(email)
	return err == nil && strings.EqualFold(parsed.Address, email)
}

func newUserID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}

	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	hexValue := hex.EncodeToString(raw[:])
	return hexValue[0:8] + "-" + hexValue[8:12] + "-" + hexValue[12:16] + "-" + hexValue[16:20] + "-" + hexValue[20:32], nil
}
