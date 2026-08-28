package telemetry

import (
	"fmt"
	"os"

	"github.com/ashutosh0x/infra-control/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger initializes a zap logger based on TelemetryConfig.
func InitLogger(cfg config.TelemetryConfig) (*zap.Logger, error) {
	var logConfig zap.Config

	// Determine development vs production presets
	// A simple heuristic based on level
	if cfg.Logging.Level == "debug" {
		logConfig = zap.NewDevelopmentConfig()
	} else {
		logConfig = zap.NewProductionConfig()
	}

	// Override log level
	level, err := zapcore.ParseLevel(cfg.Logging.Level)
	if err == nil {
		logConfig.Level = zap.NewAtomicLevelAt(level)
	}

	// Override format (json or console)
	if cfg.Logging.Format == "json" {
		logConfig.Encoding = "json"
	} else if cfg.Logging.Format == "console" {
		logConfig.Encoding = "console"
	}

	// Set output paths
	if len(cfg.Logging.OutputPaths) > 0 {
		logConfig.OutputPaths = cfg.Logging.OutputPaths
	} else {
		logConfig.OutputPaths = []string{"stdout"}
	}

	// Build the logger
	logger, err := logConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to build logger: %w", err)
	}

	// Add common fields
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	logger = logger.With(
		zap.String("service", "infra-control"),
		zap.String("version", "v0.1.0"), // Ideally injected at build time
		zap.String("environment", env),
	)

	return logger, nil
}
