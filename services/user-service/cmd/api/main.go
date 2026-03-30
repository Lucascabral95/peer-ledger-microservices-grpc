package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os/signal"
	"syscall"
	"time"

	userpb "github.com/peer-ledger/gen/user"
	"github.com/peer-ledger/services/user-service/internal/config"
	"github.com/peer-ledger/services/user-service/internal/db"
	"github.com/peer-ledger/services/user-service/internal/repository"
	userserver "github.com/peer-ledger/services/user-service/internal/server"
	"google.golang.org/grpc"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbConn, err := db.ConnectWithRetry(ctx, cfg)
	if err != nil {
		return fmt.Errorf("db connection failed: %w", err)
	}
	defer dbConn.Close()

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.GRPCPort, err)
	}
	defer lis.Close()

	userRepo := repository.NewUserRepositoryFromSQLDB(dbConn)
	userService, err := userserver.NewUserGRPCServer(userRepo)
	if err != nil {
		return fmt.Errorf("create user grpc server: %w", err)
	}

	grpcServer := grpc.NewServer()
	userpb.RegisterUserServiceServer(grpcServer, userService)

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- grpcServer.Serve(lis)
	}()

	log.Printf("user-service gRPC listening on :%s", cfg.GRPCPort)

	select {
	case err := <-serveErrCh:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		log.Printf("shutdown signal received, closing user-service")
		if err := gracefulStopGRPC(grpcServer, cfg.GracefulShutdownTimeout); err != nil {
			log.Printf("graceful stop timeout, forcing stop")
			grpcServer.Stop()
		}
		return nil
	}
}

func gracefulStopGRPC(grpcServer *grpc.Server, timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("graceful stop timed out after %s", timeout)
	}
}
