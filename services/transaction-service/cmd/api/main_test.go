package main

import (
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestGracefulStopGRPC(t *testing.T) {
	server := grpc.NewServer()
	if err := gracefulStopGRPC(server, time.Second); err != nil {
		t.Fatalf("gracefulStopGRPC() error: %v", err)
	}
}
