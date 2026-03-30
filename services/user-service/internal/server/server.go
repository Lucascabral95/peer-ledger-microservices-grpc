package server

import (
	"context"
	"errors"

	userpb "github.com/peer-ledger/gen/user"
	"github.com/peer-ledger/services/user-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserGRPCServer struct {
	userpb.UnimplementedUserServiceServer
	repo repository.UserReader
}

func NewUserGRPCServer(repo repository.UserReader) (*UserGRPCServer, error) {
	if repo == nil {
		return nil, errors.New("user repository cannot be nil")
	}
	return &UserGRPCServer{repo: repo}, nil
}

func (s *UserGRPCServer) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
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
	exists, err := s.repo.Exists(ctx, req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.Internal, "db read error")
	}

	return &userpb.UserExistsResponse{
		Exists: exists,
	}, nil
}
