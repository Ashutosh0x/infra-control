package config

import (
	"errors"
	"fmt"
)

// Validate checks all required fields, port ranges, and logic constraints in the Config.
func Validate(cfg *Config) error {
	var errs []error

	if cfg.Server.HTTPPort <= 0 || cfg.Server.HTTPPort > 65535 {
		errs = append(errs, fmt.Errorf("invalid server http_port: %d", cfg.Server.HTTPPort))
	}

	if cfg.Database.Port <= 0 || cfg.Database.Port > 65535 {
		errs = append(errs, fmt.Errorf("invalid database port: %d", cfg.Database.Port))
	}
	if cfg.Database.Host == "" {
		errs = append(errs, errors.New("database host is required"))
	}

	if cfg.Events.URL == "" {
		errs = append(errs, errors.New("events URL is required"))
	}

	if cfg.AI.Provider != "" {
		if cfg.AI.Provider != "openai" && cfg.AI.Provider != "gemini" && cfg.AI.Provider != "anthropic" {
			errs = append(errs, fmt.Errorf("unsupported AI provider: %s", cfg.AI.Provider))
		}
	}

	if len(errs) > 0 {
		var msg string
		for _, err := range errs {
			msg += err.Error() + "; "
		}
		return fmt.Errorf("config validation failed: %s", msg)
	}

	return nil
}
