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
	PricingMode        string
	MLPricingURL       string
	S3Endpoint         string
	S3PublicEndpoint   string
	S3AccessKey        string
	S3SecretKey        string
	S3Bucket           string
	S3UseSSL           bool
	MessagingGRPCAddr  string
	MessagingGRPCDial  time.Duration
	MessagingGRPCTime  time.Duration
}

// Load parses configuration from the current environment.
func Load() (Config, error) {
	cfg := Config{
		Env:               getEnv("APP_ENV", "dev"),
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		MongoURI:          os.Getenv("MONGO_URI"),
		MongoDB:           getEnv("MONGO_DB", "rentals"),
		KafkaTopicPrefix:  getEnv("KAFKA_TOPIC_PREFIX", ""),
		PricingMode:       strings.ToLower(getEnv("PRICING_MODE", "memory")),
		MLPricingURL:      getEnv("ML_PRICING_URL", "http://localhost:8000/predict"),
		S3Endpoint:        getEnv("S3_ENDPOINT", "http://localhost:9000"),
		S3PublicEndpoint:  getEnv("S3_PUBLIC_ENDPOINT", ""),
		S3AccessKey:       getEnv("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:       getEnv("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:          getEnv("S3_BUCKET", "rentme-photos"),
		MessagingGRPCAddr: getEnv("MESSAGING_GRPC_ADDR", "localhost:9000"),
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

	dialTimeout, err := parseDurationEnv("MESSAGING_GRPC_DIAL_TIMEOUT", 3*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.MessagingGRPCDial = dialTimeout

	callTimeout, err := parseDurationEnv("MESSAGING_GRPC_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	cfg.MessagingGRPCTime = callTimeout

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
	useSSL, err := parseBoolEnv("S3_USE_SSL", false)
	if err != nil {
		return Config{}, err
	}
	cfg.S3UseSSL = useSSL
	if cfg.S3PublicEndpoint == "" {
		cfg.S3PublicEndpoint = cfg.S3Endpoint
	}

	if cfg.MongoURI == "" {
		return Config{}, fmt.Errorf("MONGO_URI is required")
	}
	if len(cfg.KafkaBrokers) == 0 {
		return Config{}, fmt.Errorf("KAFKA_BROKERS is required")
	}
	if cfg.PricingMode == "" {
		cfg.PricingMode = "memory"
	}
	if cfg.MLPricingURL == "" {
		cfg.MLPricingURL = "http://localhost:8000/predict"
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

func parseBoolEnv(key string, def bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return def, nil
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "t", "true", "yes", "y", "on":
		return true, nil
	case "0", "f", "false", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s boolean: %q", key, raw)
	}
}
