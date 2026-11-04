package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment     string
	Port            string
	DatabaseURL     string
	RedisAddr       string
	JWTSecret       string
	JWTAccessTTL    time.Duration
	RefreshTokenTTL time.Duration

	ResendAPIKey                string
	EmailFrom                   string
	EmailWorkerEnabled          bool
	EmailWorkerBatchSize        int
	EmailWorkerPollInterval     time.Duration
	EmailWorkerRecoveryInterval time.Duration
	EmailWorkerStaleAfter       time.Duration
}

func Load() (Config, error) {
	accessTTL, err := time.ParseDuration(
		getEnv("JWT_ACCESS_TTL", "15m"),
	)
	if err != nil {
		return Config{}, errors.New("invalid JWT_ACCESS_TTL")
	}

	refreshTTL, err := time.ParseDuration(
		getEnv("REFRESH_TOKEN_TTL", "168h"),
	)
	if err != nil {
		return Config{}, errors.New("invalid REFRESH_TOKEN_TTL")
	}

	if accessTTL <= 0 || refreshTTL <= 0 {
		return Config{}, errors.New("token lifetimes must be positive")
	}

	jwtSecret := getEnv("JWT_SECRET", "")
	if len(jwtSecret) < 32 {
		return Config{}, errors.New(
			"JWT_SECRET must contain at least 32 characters",
		)
	}

	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	emailWorkerEnabled, err := getEnvBool("EMAIL_WORKER_ENABLED", true)
	if err != nil {
		return Config{}, err
	}

	emailWorkerBatchSize, err := getEnvInt("EMAIL_WORKER_BATCH_SIZE", 10)
	if err != nil {
		return Config{}, err
	}

	if emailWorkerBatchSize < 1 || emailWorkerBatchSize > 100 {
		return Config{}, errors.New(
			"EMAIL_WORKER_BATCH_SIZE must be between 1 and 100",
		)
	}

	emailWorkerPollInterval, err := time.ParseDuration(
		getEnv("EMAIL_WORKER_POLL_INTERVAL", "15s"),
	)
	if err != nil {
		return Config{}, errors.New("invalid EMAIL_WORKER_POLL_INTERVAL")
	}

	emailWorkerRecoveryInterval, err := time.ParseDuration(
		getEnv("EMAIL_WORKER_RECOVERY_INTERVAL", "1m"),
	)
	if err != nil {
		return Config{}, errors.New("invalid EMAIL_WORKER_RECOVERY_INTERVAL")
	}

	emailWorkerStaleAfter, err := time.ParseDuration(
		getEnv("EMAIL_WORKER_STALE_AFTER", "5m"),
	)
	if err != nil {
		return Config{}, errors.New("invalid EMAIL_WORKER_STALE_AFTER")
	}

	if emailWorkerPollInterval <= 0 ||
		emailWorkerRecoveryInterval <= 0 ||
		emailWorkerStaleAfter <= 0 {
		return Config{}, errors.New("email worker intervals must be positive")
	}

	return Config{
		Environment:     getEnv("APP_ENV", "development"),
		Port:            getEnv("APP_PORT", "8080"),
		DatabaseURL:     databaseURL,
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:       jwtSecret,
		JWTAccessTTL:    accessTTL,
		RefreshTokenTTL: refreshTTL,

		ResendAPIKey:                getEnv("RESEND_API_KEY", ""),
		EmailFrom:                   getEnv("EMAIL_FROM", ""),
		EmailWorkerEnabled:          emailWorkerEnabled,
		EmailWorkerBatchSize:        emailWorkerBatchSize,
		EmailWorkerPollInterval:     emailWorkerPollInterval,
		EmailWorkerRecoveryInterval: emailWorkerRecoveryInterval,
		EmailWorkerStaleAfter:       emailWorkerStaleAfter,
	}, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)

	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s", key)
	}

	return parsed, nil
}

func getEnvInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)

	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}

	return parsed, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}
