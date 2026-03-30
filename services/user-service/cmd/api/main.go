package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	"os"
	"time"

	_ "github.com/lib/pq"
	userpb "github.com/peer-ledger/gen/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	userpb.UnimplementedUserServiceServer
	db *sql.DB
}

func main() {
	grpcPort := getEnv("GRPC_PORT", "50051")
	dsn := getEnv("USER_DB_DSN", "postgres://admin:secret@postgres:5432/users_db?sslmode=disable")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("db open error: %v", err)
	}
	defer db.Close()

	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		log.Fatalf("db ping error: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}

	grpcServer := grpc.NewServer()
	userpb.RegisterUserServiceServer(grpcServer, &server{db: db})

	log.Printf("user-service gRPC listening on :%s", grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve error: %v", err)
	}
}

func (s *server) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	const query = `
		SELECT id, name, email
		FROM users
		WHERE id = $1
	`

	row := s.db.QueryRowContext(ctx, query, req.GetId())

	var id, name, email string
	if err := row.Scan(&id, &name, &email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "db read error")
	}

	return &userpb.GetUserResponse{
		UserId: id,
		Name:   name,
		Email:  email,
	}, nil
}

func (s *server) UserExists(ctx context.Context, req *userpb.UserExistsRequest) (*userpb.UserExistsResponse, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM users
			WHERE id = $1
		)
	`

	var exists bool
	if err := s.db.QueryRowContext(ctx, query, req.GetUserId()).Scan(&exists); err != nil {
		return nil, status.Error(codes.Internal, "db read error")
	}

	return &userpb.UserExistsResponse{
		Exists: exists,
	}, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
