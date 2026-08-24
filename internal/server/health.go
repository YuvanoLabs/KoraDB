package server

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const healthCheckMethod = "/grpc.health.v1.Health/Check"

// RegisterHealthService registers the standard gRPC health protocol and marks
// the process serving. Call it only after the database has opened successfully.
// The protocol is intentionally unauthenticated so container and service
// orchestrators can perform readiness checks without database credentials.
func RegisterHealthService(grpcServer *grpc.Server) {
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)
}

func isPublicHealthMethod(fullMethod string) bool {
	return fullMethod == healthCheckMethod
}
