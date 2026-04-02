package main

import (
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestGracefulStopGRPC(t *testing.T) {
	srv := grpc.NewServer()

	if err := gracefulStopGRPC(srv, 100*time.Millisecond); err != nil {
		t.Fatalf("gracefulStopGRPC() unexpected error: %v", err)
	}
}
