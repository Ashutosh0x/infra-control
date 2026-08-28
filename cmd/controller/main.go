// Package main provides the daemon entry point for the controller.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	// "github.com/ashutosh0x/infra-control/internal/api"
	// "github.com/ashutosh0x/infra-control/pkg/config"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting infra-control controller")

	// Setup context with cancellation on signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received signal, shutting down gracefully...", zap.String("signal", sig.String()))
		cancel()
	}()

	// TODO: Load config, initialize database, event bus, API server, controller loops
	fmt.Println("Controller initialized")

	<-ctx.Done()
	logger.Info("Controller stopped")
}
