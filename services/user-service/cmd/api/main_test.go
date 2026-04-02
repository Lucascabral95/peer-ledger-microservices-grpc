package main

import (
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestGracefulStopGRPC(t *testing.T) {
	srv := grpc.NewServer()

	err := gracefulStopGRPC(srv, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("expected graceful stop without error, got %v", err)
	}
}
