// Package config loads the application configuration from environment variables.
package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv   string
	HTTPAddr string
	// DocsEnabled serves Swagger UI and Scalar.
	DocsEnabled     bool
	ShutdownTimeout time.Duration
	// RequestTimeout bounds one HTTP request, including its database queries.
	RequestTimeout time.Duration
	DatabaseURL    string
	// DataDir holds the five source files that ship with the service.
	DataDir string
	// DataOverrideDir replaces DataDir when it holds source files.
	DataOverrideDir string
	// DatabaseWait bounds how long startup waits for PostgreSQL.
	DatabaseWait time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:          getenv("APP_ENV", "production"),
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		DocsEnabled:     getbool("DOCS_ENABLED", true),
		ShutdownTimeout: getduration("SHUTDOWN_TIMEOUT", 15*time.Second),
		RequestTimeout:  getduration("REQUEST_TIMEOUT", 60*time.Second),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		DataDir:         getenv("DATA_DIR", "./data"),
		DataOverrideDir: getenv("DATA_OVERRIDE_DIR", "../data"),
		DatabaseWait:    getduration("DATABASE_WAIT", 3*time.Minute),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
	}
	return cfg, nil
}

func (c Config) IsProduction() bool { return c.AppEnv == "production" }

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getbool(key string, fallback bool) bool {
	if b, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		return b
	}
	return fallback
}

func getduration(key string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(os.Getenv(key)); err == nil && d > 0 {
		return d
	}
	return fallback
}
