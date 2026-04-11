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

	fraudpb "github.com/peer-ledger/gen/fraud"
	"github.com/peer-ledger/internal/grpchealth"
	"github.com/peer-ledger/services/fraud-service/internal/config"
	"github.com/peer-ledger/services/fraud-service/internal/repository"
	fraudserver "github.com/peer-ledger/services/fraud-service/internal/server"
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

	fraudRepo, err := repository.NewFraudRepository(cfg)
	if err != nil {
		return fmt.Errorf("create fraud repository: %w", err)
	}
	fraudRepo.StartJanitor(ctx)

	fraudService, err := fraudserver.NewFraudGRPCServer(fraudRepo, nil)
	if err != nil {
		return fmt.Errorf("create fraud grpc server: %w", err)
	}

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.GRPCPort, err)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer()
	fraudpb.RegisterFraudServiceServer(grpcServer, fraudService)
	healthServer := grpchealth.Register(grpcServer, fraudpb.FraudService_ServiceDesc.ServiceName)

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- grpcServer.Serve(lis)
	}()

	log.Printf("fraud-service gRPC listening on :%s", cfg.GRPCPort)

	select {
	case err := <-serveErrCh:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		log.Printf("shutdown signal received, closing fraud-service")
		healthServer.Shutdown()
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
