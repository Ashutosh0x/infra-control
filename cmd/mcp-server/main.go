// Package main provides the entry point for the MCP interface server.
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

	logger.Info("Starting MCP server")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received signal, shutting down MCP server...", zap.String("signal", sig.String()))
		cancel()
	}()

	// TODO: Load config, start MCP server
	fmt.Println("MCP Server initialized")

	<-ctx.Done()
	logger.Info("MCP Server stopped")
}
