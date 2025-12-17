package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

// Config holds messaging-service configuration loaded from environment.
type Config struct {
	Env               string
	GRPCAddr          string
	ScyllaHosts       []string
	ScyllaKeyspace    string
	ScyllaUsername    string
	ScyllaPassword    string
	ScyllaConsistency gocql.Consistency
	ScyllaTimeout     time.Duration
	ReplicationFactor int
}

// Load parses environment variables into a Config struct.
func Load() (Config, error) {
	cfg := Config{
		Env:            getEnv("APP_ENV", "dev"),
		GRPCAddr:       getEnv("GRPC_ADDR", ":9000"),
		ScyllaHosts:    splitAndTrim(getEnv("SCYLLA_HOSTS", "localhost")),
		ScyllaKeyspace: strings.TrimSpace(getEnv("SCYLLA_KEYSPACE", "rentme_messaging")),
		ScyllaUsername: strings.TrimSpace(os.Getenv("SCYLLA_USERNAME")),
		ScyllaPassword: strings.TrimSpace(os.Getenv("SCYLLA_PASSWORD")),
		ReplicationFactor: parseIntWithDefault(
			strings.TrimSpace(os.Getenv("SCYLLA_REPLICATION_FACTOR")), 1),
	}
	if cfg.ScyllaKeyspace == "" {
		return Config{}, fmt.Errorf("SCYLLA_KEYSPACE is required")
	}
	if len(cfg.ScyllaHosts) == 0 {
		return Config{}, fmt.Errorf("SCYLLA_HOSTS is required")
	}

	timeout, err := parseDuration("SCYLLA_TIMEOUT", "5s")
	if err != nil {
		return Config{}, err
	}
	cfg.ScyllaTimeout = timeout

	consistency, err := parseConsistency(getEnv("SCYLLA_CONSISTENCY", "quorum"))
	if err != nil {
		return Config{}, err
	}
	cfg.ScyllaConsistency = consistency
	if cfg.ReplicationFactor < 1 {
		cfg.ReplicationFactor = 1
	}
	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitAndTrim(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func parseDuration(key, def string) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		raw = def
	}
	dur, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return dur, nil
}

func parseIntWithDefault(raw string, def int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v == 0 {
		return def
	}
	return v
}

func parseConsistency(raw string) (gocql.Consistency, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "quorum":
		return gocql.Quorum, nil
	case "one":
		return gocql.One, nil
	case "local_quorum", "localquorum", "localquorom":
		return gocql.LocalQuorum, nil
	case "all":
		return gocql.All, nil
	default:
		return gocql.Quorum, fmt.Errorf("unsupported SCYLLA_CONSISTENCY: %s", raw)
	}
}
