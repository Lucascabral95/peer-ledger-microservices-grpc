package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	fraudpb "github.com/peer-ledger/gen/fraud"
	userpb "github.com/peer-ledger/gen/user"
	walletpb "github.com/peer-ledger/gen/wallet"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	userClient   userpb.UserServiceClient
	fraudClient  fraudpb.FraudServiceClient
	walletClient walletpb.WalletServiceClient
}

func main() {
	webPort := getEnv("PORT", "8080")
	userSvcAddr := getEnv("USER_SERVICE_GRPC_ADDR", "user-service:50051")
	fraudSvcAddr := getEnv("FRAUD_SERVICE_GRPC_ADDR", "fraud-service:50052")
	walletSvcAddr := getEnv("WALLET_SERVICE_GRPC_ADDR", "wallet-service:50053")

	userConn, err := dialGRPCWithRetry(userSvcAddr, 3*time.Second, 10)
	if err != nil {
		log.Fatalf("failed to connect user-service grpc at %s: %v", userSvcAddr, err)
	}
	defer userConn.Close()

	fraudConn, err := dialGRPCWithRetry(fraudSvcAddr, 3*time.Second, 10)
	if err != nil {
		log.Fatalf("failed to connect fraud-service grpc at %s: %v", fraudSvcAddr, err)
	}
	defer fraudConn.Close()

	walletConn, err := dialGRPCWithRetry(walletSvcAddr, 3*time.Second, 10)
	if err != nil {
		log.Fatalf("failed to connect wallet-service grpc at %s: %v", walletSvcAddr, err)
	}
	defer walletConn.Close()

	app := Config{
		userClient:   userpb.NewUserServiceClient(userConn),
		fraudClient:  fraudpb.NewFraudServiceClient(fraudConn),
		walletClient: walletpb.NewWalletServiceClient(walletConn),
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", webPort),
		Handler: app.routes(),
	}

	err = srv.ListenAndServe()
	if err != nil {
		log.Panic(err)
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
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
