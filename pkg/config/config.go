// Package config provides configuration structures.
package config

// ServerConfig holds configuration for the API server.
type ServerConfig struct {
	Port int `mapstructure:"port"`
}
