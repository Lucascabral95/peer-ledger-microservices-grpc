package grpchealth

import (
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func Register(grpcServer *grpc.Server, serviceNames ...string) *health.Server {
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	for _, serviceName := range serviceNames {
		if trimmed := strings.TrimSpace(serviceName); trimmed != "" {
			healthServer.SetServingStatus(trimmed, healthpb.HealthCheckResponse_SERVING)
		}
	}

	healthpb.RegisterHealthServer(grpcServer, healthServer)
	return healthServer
}
