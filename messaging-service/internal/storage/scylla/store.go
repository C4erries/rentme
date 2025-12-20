package scylla

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

// Store wraps Scylla queries for conversations and messages.
type Store struct {
	session *gocql.Session
	logger  *slog.Logger
}

// NewStore builds a Store.
func NewStore(session *gocql.Session, logger *slog.Logger) *Store {
	return &Store{session: session, logger: logger}
}

// GetConversation returns a conversation by its identifier.
func (s *Store) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	if s.session == nil {
		return nil, errors.New("scylla session not initialized")
	}
	uuid, err := gocql.ParseUUID(strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	var row Conversation
	if err := s.session.
		Query(`SELECT id, listing_id, participants, created_at, last_message_at, last_message_id, last_message_sender_id, last_message_text FROM conversations WHERE id = ? LIMIT 1`, uuid).
		WithContext(ctx).
		Consistency(gocql.One).
		Scan(&row.ID, &row.ListingID, &row.Participants, &row.CreatedAt, &row.LastMessageAt, &row.LastMessageID, &row.LastMessageSenderID, &row.LastMessageText); err != nil {
		return nil, err
	}
	return &row, nil
}

// FindConversationByListing tries to locate an existing thread for a listing and participant set.
func (s *Store) FindConversationByListing(ctx context.Context, listingID string, participants []string) (*Conversation, error) {
	if s.session == nil {
		return nil, errors.New("scylla session not initialized")
	}
	normalizedParticipants := normalizeParticipants(participants)
	iter := s.session.
		Query(`SELECT id, listing_id, participants, created_at, last_message_at, last_message_id, last_message_sender_id, last_message_text FROM conversations WHERE listing_id = ? ALLOW FILTERING`, listingID).
		WithContext(ctx).
		Consistency(gocql.One).
		Iter()

	var (
		id            gocql.UUID
		listing       string
		storedParts   []string
		createdAt     time.Time
		lastMessageAt time.Time
		lastMessageID gocql.UUID
		lastSenderID  string
		lastText      string
	)
	for iter.Scan(&id, &listing, &storedParts, &createdAt, &lastMessageAt, &lastMessageID, &lastSenderID, &lastText) {
		if sameParticipants(storedParts, normalizedParticipants) {
			return &Conversation{
				ID:                  id,
				ListingID:           listing,
				Participants:        append([]string(nil), storedParts...),
				CreatedAt:           createdAt,
				LastMessageAt:       lastMessageAt,
				LastMessageID:       lastMessageID,
				LastMessageSenderID: lastSenderID,
				LastMessageText:     lastText,
			}, nil
		}
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return nil, gocql.ErrNotFound
}

// CreateConversation inserts a new conversation entry.
func (s *Store) CreateConversation(ctx context.Context, listingID string, participants []string, now time.Time) (*Conversation, error) {
	if s.session == nil {
		return nil, errors.New("scylla session not initialized")
	}
	id := gocql.TimeUUID()
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	normalizedParticipants := normalizeParticipants(participants)
	if err := s.session.
		Query(`INSERT INTO conversations (id, listing_id, participants, created_at, last_message_at, last_message_text) VALUES (?, ?, ?, ?, ?, ?)`,
			id, listingID, normalizedParticipants, now, now, "").
		WithContext(ctx).
		Consistency(gocql.Quorum).
		Exec(); err != nil {
		return nil, err
	}
	return &Conversation{
		ID:            id,
		ListingID:     listingID,
		Participants:  normalizedParticipants,
		CreatedAt:     now,
		LastMessageAt: now,
	}, nil
}

// ListConversations returns conversations for a participant or all when includeAll is true.
func (s *Store) ListConversations(ctx context.Context, userID string, includeAll bool) ([]Conversation, error) {
	if s.session == nil {
		return nil, errors.New("scylla session not initialized")
	}
	var iter *gocql.Iter
	if includeAll {
		iter = s.session.
			Query(`SELECT id, listing_id, participants, created_at, last_message_at, last_message_id, last_message_sender_id, last_message_text FROM conversations`).
			WithContext(ctx).
			Consistency(gocql.One).
			Iter()
	} else {
		iter = s.session.
			Query(`SELECT id, listing_id, participants, created_at, last_message_at, last_message_id, last_message_sender_id, last_message_text FROM conversations WHERE participants CONTAINS ? ALLOW FILTERING`, userID).
			WithContext(ctx).
			Consistency(gocql.One).
			Iter()
	}

	var (
		id            gocql.UUID
		listing       string
		participants  []string
		createdAt     time.Time
		lastMessageAt time.Time
		lastMessageID gocql.UUID
		lastSenderID  string
		lastText      string
	)
	conversations := make([]Conversation, 0)
	for iter.Scan(&id, &listing, &participants, &createdAt, &lastMessageAt, &lastMessageID, &lastSenderID, &lastText) {
		conversations = append(conversations, Conversation{
			ID:                  id,
			ListingID:           listing,
			Participants:        append([]string(nil), participants...),
			CreatedAt:           createdAt,
			LastMessageAt:       lastMessageAt,
			LastMessageID:       lastMessageID,
			LastMessageSenderID: lastSenderID,
			LastMessageText:     lastText,
		})
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}

	sort.Slice(conversations, func(i, j int) bool {
		return lastActivity(conversations[i]).After(lastActivity(conversations[j]))
	})
	return conversations, nil
}

// AddMessage appends a message and updates conversation activity timestamp.
func (s *Store) AddMessage(ctx context.Context, conversationID gocql.UUID, senderID, text string, at time.Time) (*Message, error) {
	if s.session == nil {
		return nil, errors.New("scylla session not initialized")
	}
	snippet := trimSnippet(text, 500)
	if at.IsZero() {
		at = time.Now()
	}
	at = at.UTC()
	messageID := gocql.TimeUUID()
	if err := s.session.
		Query(`INSERT INTO messages (conversation_id, message_id, sender_id, text, created_at) VALUES (?, ?, ?, ?, ?)`,
			conversationID, messageID, senderID, text, at).
		WithContext(ctx).
		Consistency(gocql.Quorum).
		Exec(); err != nil {
		return nil, err
	}
	// best-effort update of last_message_at
	if err := s.session.
		Query(`UPDATE conversations SET last_message_at = ?, last_message_id = ?, last_message_sender_id = ?, last_message_text = ? WHERE id = ?`,
			at, messageID, senderID, snippet, conversationID).
		WithContext(ctx).
		Consistency(gocql.One).
		Exec(); err != nil && s.logger != nil {
		s.logger.Warn("failed to update last message meta", "error", err, "conversation_id", conversationID)
	}
	return &Message{
		ID:             messageID,
		ConversationID: conversationID,
		SenderID:       senderID,
		Text:           text,
		CreatedAt:      at,
	}, nil
}

func trimSnippet(text string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}

// MarkConversationRead upserts read position for a user.
func (s *Store) MarkConversationRead(ctx context.Context, conversationID gocql.UUID, userID string, lastRead gocql.UUID, at time.Time) error {
	if s.session == nil {
		return errors.New("scylla session not initialized")
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return s.session.
		Query(`INSERT INTO conversation_reads (user_id, conversation_id, last_read_message_id, updated_at) VALUES (?, ?, ?, ?)`,
			userID, conversationID, lastRead, at).
		WithContext(ctx).
		Consistency(gocql.Quorum).
		Exec()
}

// ListConversationReads returns last read markers for the user.
func (s *Store) ListConversationReads(ctx context.Context, userID string) (map[gocql.UUID]ConversationRead, error) {
	if s.session == nil {
		return nil, errors.New("scylla session not initialized")
	}
	iter := s.session.
		Query(`SELECT user_id, conversation_id, last_read_message_id, updated_at FROM conversation_reads WHERE user_id = ?`, userID).
		WithContext(ctx).
		Consistency(gocql.One).
		Iter()
	result := make(map[gocql.UUID]ConversationRead)
	var (
		readUserID string
		convID     gocql.UUID
		lastRead   gocql.UUID
		updatedAt  time.Time
	)
	for iter.Scan(&readUserID, &convID, &lastRead, &updatedAt) {
		result[convID] = ConversationRead{
			ConversationID:    convID,
			UserID:            readUserID,
			LastReadMessageID: lastRead,
			UpdatedAt:         updatedAt,
		}
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return result, nil
}

// ListMessages returns messages ordered from newest to oldest with optional cursor.
func (s *Store) ListMessages(ctx context.Context, conversationID gocql.UUID, limit int, before *gocql.UUID) ([]Message, error) {
	if s.session == nil {
		return nil, errors.New("scylla session not initialized")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var iter *gocql.Iter
	if before != nil {
		iter = s.session.
			Query(`SELECT conversation_id, message_id, sender_id, text, created_at FROM messages WHERE conversation_id = ? AND message_id < ? ORDER BY message_id DESC LIMIT ?`,
				conversationID, *before, limit).
			WithContext(ctx).
			Consistency(gocql.One).
			Iter()
	} else {
		iter = s.session.
			Query(`SELECT conversation_id, message_id, sender_id, text, created_at FROM messages WHERE conversation_id = ? ORDER BY message_id DESC LIMIT ?`,
				conversationID, limit).
			WithContext(ctx).
			Consistency(gocql.One).
			Iter()
	}

	messages := make([]Message, 0, limit)
	var (
		cID       gocql.UUID
		messageID gocql.UUID
		sender    string
		text      string
		createdAt time.Time
	)
	for iter.Scan(&cID, &messageID, &sender, &text, &createdAt) {
		messages = append(messages, Message{
			ID:             messageID,
			ConversationID: cID,
			SenderID:       sender,
			Text:           text,
			CreatedAt:      createdAt,
		})
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return messages, nil
}

func normalizeParticipants(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func sameParticipants(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aNorm := normalizeParticipants(a)
	bNorm := normalizeParticipants(b)
	if len(aNorm) != len(bNorm) {
		return false
	}
	for i := range aNorm {
		if aNorm[i] != bNorm[i] {
			return false
		}
	}
	return true
}

func lastActivity(c Conversation) time.Time {
	if !c.LastMessageAt.IsZero() {
		return c.LastMessageAt
	}
	return c.CreatedAt
}
