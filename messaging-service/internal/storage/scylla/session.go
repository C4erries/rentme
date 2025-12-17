package scylla

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/gocql/gocql"

	"messaging-service/internal/config"
)

var keyspacePattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// NewSession ensures schema exists and returns a connected Scylla session.
func NewSession(cfg config.Config, logger *slog.Logger) (*gocql.Session, error) {
	if !keyspacePattern.MatchString(cfg.ScyllaKeyspace) {
		return nil, fmt.Errorf("invalid keyspace name: %s", cfg.ScyllaKeyspace)
	}

	baseCluster := gocql.NewCluster(cfg.ScyllaHosts...)
	baseCluster.Timeout = cfg.ScyllaTimeout
	baseCluster.Consistency = cfg.ScyllaConsistency
	setAuth(baseCluster, cfg)

	baseSession, err := baseCluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("connect to scylla: %w", err)
	}
	defer baseSession.Close()

	if err := ensureKeyspace(context.Background(), baseSession, cfg); err != nil {
		return nil, err
	}

	cluster := gocql.NewCluster(cfg.ScyllaHosts...)
	cluster.Timeout = cfg.ScyllaTimeout
	cluster.Keyspace = cfg.ScyllaKeyspace
	cluster.Consistency = cfg.ScyllaConsistency
	setAuth(cluster, cfg)

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("connect to keyspace %s: %w", cfg.ScyllaKeyspace, err)
	}
	if err := ensureTables(context.Background(), session, cfg); err != nil {
		session.Close()
		return nil, err
	}
	if logger != nil {
		logger.Info("scylla connected", "hosts", cfg.ScyllaHosts, "keyspace", cfg.ScyllaKeyspace)
	}
	return session, nil
}

func ensureKeyspace(ctx context.Context, session *gocql.Session, cfg config.Config) error {
	cql := fmt.Sprintf(
		"CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}",
		cfg.ScyllaKeyspace, cfg.ReplicationFactor,
	)
	if err := session.Query(cql).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("create keyspace: %w", err)
	}
	return nil
}

func ensureTables(ctx context.Context, session *gocql.Session, cfg config.Config) error {
	conversations := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.conversations (
	id uuid PRIMARY KEY,
	listing_id text,
	participants set<text>,
	created_at timestamp,
	last_message_at timestamp
);`, cfg.ScyllaKeyspace)
	if err := session.Query(conversations).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("create conversations table: %w", err)
	}

	messages := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.messages (
	conversation_id uuid,
	message_id timeuuid,
	sender_id text,
	text text,
	created_at timestamp,
	PRIMARY KEY (conversation_id, message_id)
) WITH CLUSTERING ORDER BY (message_id DESC);`, cfg.ScyllaKeyspace)
	if err := session.Query(messages).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("create messages table: %w", err)
	}
	return nil
}

func setAuth(cluster *gocql.ClusterConfig, cfg config.Config) {
	if cfg.ScyllaUsername == "" {
		return
	}
	cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: cfg.ScyllaUsername,
		Password: cfg.ScyllaPassword,
	}
	// avoid long stalls on auth/connect
	cluster.ConnectTimeout = cfg.ScyllaTimeout
	cluster.Timeout = cfg.ScyllaTimeout
}

// Conversation represents a chat thread persisted in Scylla.
type Conversation struct {
	ID            gocql.UUID
	ListingID     string
	Participants  []string
	CreatedAt     time.Time
	LastMessageAt time.Time
}

// Message represents a chat message persisted in Scylla.
type Message struct {
	ID             gocql.UUID
	ConversationID gocql.UUID
	SenderID       string
	Text           string
	CreatedAt      time.Time
}
