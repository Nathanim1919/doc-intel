package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration loaded from environment variables.
// No defaults for secrets — missing secrets cause a startup failure, not a silent bad state.
type Config struct {
	// Server
	Port int

	// Postgres
	PostgresDSN string

	// Redis
	RedisURL string

	// MinIO / S3
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool
}

// Load reads all configuration from environment. Returns an error if any
// required variable is missing. This is called once at startup — crash fast.
func Load() (*Config, error) {
	cfg := &Config{}
	var missing []string

	cfg.PostgresDSN = required("POSTGRES_DSN", &missing)
	cfg.RedisURL = required("REDIS_URL", &missing)
	cfg.MinioEndpoint = required("MINIO_ENDPOINT", &missing)
	cfg.MinioAccessKey = required("MINIO_ROOT_USER", &missing)
	cfg.MinioSecretKey = required("MINIO_ROOT_PASSWORD", &missing)
	cfg.MinioBucket = required("MINIO_BUCKET", &missing)

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %v", missing)
	}

	// Optional with defaults
	cfg.Port = envInt("API_PORT", 8080)
	cfg.MinioUseSSL = envBool("MINIO_USE_SSL", false)

	return cfg, nil
}

func required(key string, missing *[]string) string {
	v := os.Getenv(key)
	if v == "" {
		*missing = append(*missing, key)
	}
	return v
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
