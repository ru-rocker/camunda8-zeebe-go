package config

import (
	"os"
	"strconv"
	"time"
)

// Config encapsulates runtime configuration for the Zeebe worker
type Config struct {
	ZeebeAddress           string
	ZeebeInsecure          bool
	ZeebeClientID          string
	ZeebeClientSecret      string
	ZeebeAudience          string
	ZeebeAuthServerURL     string
	WorkerName             string
	WorkerTimeout          time.Duration
	WorkerMaxJobsActive    int
	WorkerConcurrency      int
	CBFailureThreshold     int
	CBRecoveryTimeout      time.Duration
	BackoffInitialInterval time.Duration
	BackoffMaxInterval     time.Duration
	BackoffMaxRetries      int
}

// LoadConfig reads configuration from environment variables with fallback defaults
func LoadConfig() *Config {
	return &Config{
		ZeebeAddress:           getEnv("ZEEBE_ADDRESS", "localhost:26500"),
		ZeebeInsecure:          getEnvAsBool("ZEEBE_INSECURE_CONNECTION", true),
		ZeebeClientID:          getEnv("ZEEBE_CLIENT_ID", ""),
		ZeebeClientSecret:      getEnv("ZEEBE_CLIENT_SECRET", ""),
		ZeebeAudience:          getEnv("ZEEBE_TOKEN_AUDIENCE", ""),
		ZeebeAuthServerURL:     getEnv("ZEEBE_AUTHORIZATION_SERVER_URL", ""),
		WorkerName:             getEnv("WORKER_NAME", "order-fulfillment-worker"),
		WorkerTimeout:          getEnvAsDuration("WORKER_TIMEOUT", 30*time.Second),
		WorkerMaxJobsActive:    getEnvAsInt("WORKER_MAX_JOBS_ACTIVE", 32),
		WorkerConcurrency:      getEnvAsInt("WORKER_CONCURRENCY", 4),
		CBFailureThreshold:     getEnvAsInt("CIRCUIT_BREAKER_FAILURE_THRESHOLD", 3),
		CBRecoveryTimeout:      getEnvAsDuration("CIRCUIT_BREAKER_RECOVERY_TIMEOUT", 10*time.Second),
		BackoffInitialInterval: getEnvAsDuration("BACKOFF_INITIAL_INTERVAL", 500*time.Millisecond),
		BackoffMaxInterval:     getEnvAsDuration("BACKOFF_MAX_INTERVAL", 30*time.Second),
		BackoffMaxRetries:      getEnvAsInt("BACKOFF_MAX_RETRIES", 3),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		i, err := strconv.Atoi(val)
		if err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		d, err := time.ParseDuration(val)
		if err == nil {
			return d
		}
	}
	return defaultVal
}
