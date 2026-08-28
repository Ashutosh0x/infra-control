// Package main provides the entry point for async workers.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting infra-control worker process")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received signal, shutting down worker...", zap.String("signal", sig.String()))
		cancel()
	}()

	// TODO: Load config, connect to NATS, subscribe to work queues
	fmt.Println("Worker initialized")

	<-ctx.Done()
	logger.Info("Worker stopped")
}
