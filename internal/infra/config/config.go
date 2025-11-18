package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config aggregates application configuration values loaded from environment variables.
type Config struct {
	Env                string
	HTTPAddr           string
	MongoURI           string
	MongoDB            string
	KafkaBrokers       []string
	KafkaTopicPrefix   string
	IdempotencyTTL     time.Duration
	OutboxPollInterval time.Duration
	RetryBackoff       []time.Duration
}

// Load parses configuration from the current environment.
func Load() (Config, error) {
	cfg := Config{
		Env:              getEnv("APP_ENV", "dev"),
		HTTPAddr:         getEnv("HTTP_ADDR", ":8080"),
		MongoURI:         os.Getenv("MONGO_URI"),
		MongoDB:          getEnv("MONGO_DB", "rentals"),
		KafkaTopicPrefix: getEnv("KAFKA_TOPIC_PREFIX", ""),
	}
	brokers := getEnv("KAFKA_BROKERS", "")
	if brokers != "" {
		cfg.KafkaBrokers = strings.Split(brokers, ",")
	}
	idempotencyTTL, err := parseDurationEnv("IDEMP_TTL", 168*time.Hour)
	if err != nil {
		return Config{}, err
	}
	cfg.IdempotencyTTL = idempotencyTTL

	poll, err := parseDurationEnv("OUTBOX_POLL_INTERVAL", 500*time.Millisecond)
	if err != nil {
		return Config{}, err
	}
	cfg.OutboxPollInterval = poll

	retryStr := getEnv("RETRY_BACKOFF", "1s,5s,30s")
	for _, raw := range strings.Split(retryStr, ",") {
		val := strings.TrimSpace(raw)
		if val == "" {
			continue
		}
		d, err := time.ParseDuration(val)
		if err != nil {
			return Config{}, fmt.Errorf("invalid RETRY_BACKOFF component %q: %w", raw, err)
		}
		cfg.RetryBackoff = append(cfg.RetryBackoff, d)
	}

	if cfg.MongoURI == "" {
		return Config{}, fmt.Errorf("MONGO_URI is required")
	}
	if len(cfg.KafkaBrokers) == 0 {
		return Config{}, fmt.Errorf("KAFKA_BROKERS is required")
	}
	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDurationEnv(key string, def time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s duration: %w", key, err)
	}
	return d, nil
}
