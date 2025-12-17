package dto

import "time"

// Conversation describes chat metadata.
type Conversation struct {
	ID                 string    `json:"id"`
	ListingID          string    `json:"listing_id,omitempty"`
	Participants       []string  `json:"participants"`
	CreatedAt          time.Time `json:"created_at"`
	LastMessageAt      time.Time `json:"last_message_at,omitempty"`
	LastMessageID      string    `json:"last_message_id,omitempty"`
	LastMessageSender  string    `json:"last_message_sender_id,omitempty"`
	HasUnread          bool      `json:"has_unread,omitempty"`
}

// ConversationList is a paginated collection.
type ConversationList struct {
	Items      []Conversation `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// ChatMessage contains a single message payload.
type ChatMessage struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	Text           string    `json:"text"`
	CreatedAt      time.Time `json:"created_at"`
}

// ChatMessageList is a paginated message list.
type ChatMessageList struct {
	Items      []ChatMessage `json:"items"`
	NextCursor string        `json:"next_cursor,omitempty"`
}
