// Package config loads 12-factor configuration from environment variables.
package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all service configuration.
type Config struct {
	// Server
	GRPCAddr string `envconfig:"GRPC_ADDR" default:":9090"`
	HTTPAddr string `envconfig:"HTTP_ADDR" default:":8080"`

	// Postgres
	DatabaseURL string `envconfig:"DATABASE_URL" default:"postgres://vault:vault@localhost:5432/vault?sslmode=disable"`

	// Redis
	RedisAddr     string        `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	RedisPassword string        `envconfig:"REDIS_PASSWORD" default:""`
	CacheTTL      time.Duration `envconfig:"CACHE_TTL" default:"5m"`

	// Rate limiting (per IP, per window)
	RateLimitMax    int           `envconfig:"RATE_LIMIT_MAX" default:"100"`
	RateLimitWindow time.Duration `envconfig:"RATE_LIMIT_WINDOW" default:"1m"`

	// Short code
	CodeLength int `envconfig:"CODE_LENGTH" default:"7"`

	// Graceful shutdown
	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" default:"10s"`

	// Observability
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
}

// Load reads config from environment, returning an error if required vars are absent.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &cfg, nil
}
