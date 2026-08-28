package api

import (
	"context"
	"net/http"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/ashutosh0x/infra-control/pkg/config"
)

// Server represents the API server hosting HTTP and gRPC endpoints.
type Server struct {
	httpServer *http.Server
	grpcServer *grpc.Server
	logger     *zap.Logger
	config     *config.ServerConfig
}

// NewServer creates a new API server instance.
func NewServer(cfg *config.ServerConfig, logger *zap.Logger) *Server {
	return &Server{
		logger: logger,
		config: cfg,
	}
}

// Start starts the HTTP and gRPC servers.
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting API server")
	// TODO: implement actual start logic
	return nil
}

// Stop gracefully shuts down the API server.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping API server")
	// TODO: implement graceful shutdown
	return nil
}

// RegisterHTTPHandlers registers all HTTP routes to the provided mux.
func (s *Server) RegisterHTTPHandlers(mux *http.ServeMux) {
	// Handlers are registered in routes/v1.go
}
