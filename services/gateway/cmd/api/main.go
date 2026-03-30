package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	userpb "github.com/peer-ledger/gen/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Config struct {
	userClient userpb.UserServiceClient
}

func main() {
	webPort := getEnv("PORT", "8080")
	userSvcAddr := getEnv("USER_SERVICE_GRPC_ADDR", "user-service:50051")

	userConn, err := grpc.Dial(userSvcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect user-service grpc at %s: %v", userSvcAddr, err)
	}
	defer userConn.Close()

	app := Config{
		userClient: userpb.NewUserServiceClient(userConn),
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
