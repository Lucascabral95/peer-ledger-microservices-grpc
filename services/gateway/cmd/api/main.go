package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	fraudpb "github.com/peer-ledger/gen/fraud"
	transactionpb "github.com/peer-ledger/gen/transaction"
	userpb "github.com/peer-ledger/gen/user"
	walletpb "github.com/peer-ledger/gen/wallet"
	"github.com/peer-ledger/internal/security"
	_ "github.com/peer-ledger/services/gateway/docs"
	gatewayconfig "github.com/peer-ledger/services/gateway/internal/config"
	gatewaymiddleware "github.com/peer-ledger/services/gateway/internal/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	userClient        userpb.UserServiceClient
	fraudClient       fraudpb.FraudServiceClient
	walletClient      walletpb.WalletServiceClient
	transactionClient transactionpb.TransactionServiceClient
	rateLimiter       *gatewaymiddleware.RateLimiter
	tokenManager      *security.JWTManager
	httpMetrics       *gatewaymiddleware.HTTPMetrics
	metricsHandler    http.Handler
	metricsPath       string
}

// @title Peer Ledger Gateway API
// @version 1.0
// @description Public HTTP API for Peer Ledger. The gateway is the only external entrypoint and orchestrates the internal gRPC services.
// @description Authenticated routes require a bearer JWT issued by the gateway after register or login.
// @contact.name Lucas Cabral
// @contact.url https://github.com/Lucascabral95/peer-ledger-microservices-grpc
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer JWT issued by the gateway. Format: Bearer {token}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := gatewayconfig.Load()
	if err != nil {
		return fmt.Errorf("load gateway config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	userConn, err := dialGRPCWithRetry(cfg.UserServiceGRPCAddr, cfg.GRPCDialTimeout, cfg.GRPCMaxAttempts)
	if err != nil {
		return fmt.Errorf("failed to connect user-service grpc at %s: %w", cfg.UserServiceGRPCAddr, err)
	}
	defer userConn.Close()

	fraudConn, err := dialGRPCWithRetry(cfg.FraudServiceGRPCAddr, cfg.GRPCDialTimeout, cfg.GRPCMaxAttempts)
	if err != nil {
		return fmt.Errorf("failed to connect fraud-service grpc at %s: %w", cfg.FraudServiceGRPCAddr, err)
	}
	defer fraudConn.Close()

	walletConn, err := dialGRPCWithRetry(cfg.WalletServiceGRPCAddr, cfg.GRPCDialTimeout, cfg.GRPCMaxAttempts)
	if err != nil {
		return fmt.Errorf("failed to connect wallet-service grpc at %s: %w", cfg.WalletServiceGRPCAddr, err)
	}
	defer walletConn.Close()

	transactionConn, err := dialGRPCWithRetry(cfg.TransactionServiceAddr, cfg.GRPCDialTimeout, cfg.GRPCMaxAttempts)
	if err != nil {
		return fmt.Errorf("failed to connect transaction-service grpc at %s: %w", cfg.TransactionServiceAddr, err)
	}
	defer transactionConn.Close()

	app := &Config{
		userClient:        userpb.NewUserServiceClient(userConn),
		fraudClient:       fraudpb.NewFraudServiceClient(fraudConn),
		walletClient:      walletpb.NewWalletServiceClient(walletConn),
		transactionClient: transactionpb.NewTransactionServiceClient(transactionConn),
		metricsPath:       cfg.MetricsPath,
	}

	app.tokenManager, err = security.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL, nil)
	if err != nil {
		return fmt.Errorf("create jwt manager: %w", err)
	}
	if cfg.RateLimitEnabled {
		app.rateLimiter = gatewaymiddleware.NewRateLimiter(
			gatewaymiddleware.Policy{
				Name:     "default",
				Requests: cfg.RateLimitDefaultRequests,
				Window:   cfg.RateLimitDefaultWindow,
			},
			map[string]gatewaymiddleware.Policy{
				"/transfers": {
					Name:     "transfers",
					Requests: cfg.RateLimitTransfersRequests,
					Window:   cfg.RateLimitTransfersWindow,
				},
			},
			cfg.RateLimitExemptPaths,
			cfg.RateLimitCleanup,
			cfg.RateLimitTrustProxy,
			nil,
		)
	}
	if cfg.MetricsEnabled {
		registry := prometheus.NewRegistry()
		app.httpMetrics = gatewaymiddleware.NewHTTPMetrics(registry)
		app.metricsHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler: app.routes(),
	}

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- srv.ListenAndServe()
	}()

	log.Printf("gateway HTTP listening on :%s", cfg.HTTPPort)

	select {
	case err := <-serverErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http serve: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
		return nil
	}
}

func dialGRPC(address string, timeout time.Duration) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return grpc.DialContext(
		ctx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
}

func dialGRPCWithRetry(address string, timeout time.Duration, maxAttempts int) (*grpc.ClientConn, error) {
	var lastErr error

	backoff := 500 * time.Millisecond
	maxBackoff := 5 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := dialGRPC(address, timeout)
		if err == nil {
			if attempt > 1 {
				log.Printf("gRPC connection established to %s on attempt %d", address, attempt)
			}
			return conn, nil
		}

		lastErr = err
		log.Printf("gRPC dial attempt %d/%d failed for %s: %v", attempt, maxAttempts, address, err)

		if attempt == maxAttempts {
			break
		}

		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	return nil, errors.New("exhausted grpc dial retries: " + lastErr.Error())
}
