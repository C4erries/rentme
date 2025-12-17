package messaging

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "messaging-service/proto"
)

// Config defines gRPC client settings.
type Config struct {
	Addr        string
	DialTimeout time.Duration
	CallTimeout time.Duration
}

// Client wraps the messaging-service gRPC API.
type Client struct {
	conn        *grpc.ClientConn
	svc         pb.MessagingServiceClient
	callTimeout time.Duration
	logger      *slog.Logger
}

// Conversation models a chat thread used by the HTTP layer.
type Conversation struct {
	ID            string
	ListingID     string
	Participants  []string
	CreatedAt     time.Time
	LastMessageAt time.Time
}

// Message models a chat message used by the HTTP layer.
type Message struct {
	ID             string
	ConversationID string
	SenderID       string
	Text           string
	CreatedAt      time.Time
}

// NewClient dials messaging-service and returns a typed client.
func NewClient(ctx context.Context, cfg Config, logger *slog.Logger) (*Client, error) {
	if cfg.Addr == "" {
		return nil, errors.New("messaging: address required")
	}
	dialTimeout := cfg.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}
	callTimeout := cfg.CallTimeout
	if callTimeout <= 0 {
		callTimeout = 5 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, cfg.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	if logger != nil {
		logger.Info("messaging grpc connected", "addr", cfg.Addr)
	}
	return &Client{
		conn:        conn,
		svc:         pb.NewMessagingServiceClient(conn),
		callTimeout: callTimeout,
		logger:      logger,
	}, nil
}

// Close releases the gRPC connection.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// GetOrCreateConversationForListing returns a listing-specific chat.
func (c *Client) GetOrCreateConversationForListing(ctx context.Context, listingID, guestID, hostID string) (Conversation, error) {
	req := &pb.GetOrCreateConversationForListingRequest{
		ListingId: listingID,
		GuestId:   guestID,
		HostId:    hostID,
	}
	callCtx, cancel := c.wrapCall(ctx)
	defer cancel()
	resp, err := c.svc.GetOrCreateConversationForListing(callCtx, req)
	if err != nil {
		return Conversation{}, err
	}
	return mapConversation(resp.GetConversation()), nil
}

// GetConversation loads conversation metadata.
func (c *Client) GetConversation(ctx context.Context, id string) (Conversation, error) {
	callCtx, cancel := c.wrapCall(ctx)
	defer cancel()
	resp, err := c.svc.GetConversation(callCtx, &pb.GetConversationRequest{ConversationId: id})
	if err != nil {
		return Conversation{}, err
	}
	return mapConversation(resp.GetConversation()), nil
}

// ListConversations returns conversations and pagination cursor.
func (c *Client) ListConversations(ctx context.Context, userID string, limit int, cursor string, includeAll bool) ([]Conversation, string, error) {
	req := &pb.ListConversationsRequest{
		UserId:     userID,
		Limit:      int32(limit),
		Cursor:     cursor,
		IncludeAll: includeAll,
	}
	callCtx, cancel := c.wrapCall(ctx)
	defer cancel()
	resp, err := c.svc.ListConversations(callCtx, req)
	if err != nil {
		return nil, "", err
	}
	items := make([]Conversation, 0, len(resp.GetConversations()))
	for _, conv := range resp.GetConversations() {
		items = append(items, mapConversation(conv))
	}
	return items, resp.GetNextCursor(), nil
}

// SendMessage posts a message to a conversation.
func (c *Client) SendMessage(ctx context.Context, conversationID, senderID, text string) (Message, error) {
	req := &pb.SendMessageRequest{
		ConversationId: conversationID,
		SenderId:       senderID,
		Text:           text,
	}
	callCtx, cancel := c.wrapCall(ctx)
	defer cancel()
	resp, err := c.svc.SendMessage(callCtx, req)
	if err != nil {
		return Message{}, err
	}
	return mapMessage(resp.GetMessage()), nil
}

// ListMessages returns messages and pagination cursor.
func (c *Client) ListMessages(ctx context.Context, conversationID string, limit int, cursor string) ([]Message, string, error) {
	req := &pb.ListMessagesRequest{
		ConversationId: conversationID,
		Limit:          int32(limit),
		Before:         cursor,
	}
	callCtx, cancel := c.wrapCall(ctx)
	defer cancel()
	resp, err := c.svc.ListMessages(callCtx, req)
	if err != nil {
		return nil, "", err
	}
	items := make([]Message, 0, len(resp.GetMessages()))
	for _, msg := range resp.GetMessages() {
		items = append(items, mapMessage(msg))
	}
	return items, resp.GetNextCursor(), nil
}

func (c *Client) wrapCall(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := c.callTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func mapConversation(conv *pb.Conversation) Conversation {
	if conv == nil {
		return Conversation{}
	}
	createdAt := time.Time{}
	if conv.CreatedAt != nil {
		createdAt = conv.CreatedAt.AsTime()
	}
	lastMessage := time.Time{}
	if conv.LastMessageAt != nil {
		lastMessage = conv.LastMessageAt.AsTime()
	}
	return Conversation{
		ID:            conv.GetId(),
		ListingID:     conv.GetListingId(),
		Participants:  append([]string(nil), conv.GetParticipants()...),
		CreatedAt:     createdAt,
		LastMessageAt: lastMessage,
	}
}

func mapMessage(msg *pb.Message) Message {
	if msg == nil {
		return Message{}
	}
	createdAt := time.Time{}
	if msg.CreatedAt != nil {
		createdAt = msg.CreatedAt.AsTime()
	}
	return Message{
		ID:             msg.GetId(),
		ConversationID: msg.GetConversationId(),
		SenderID:       msg.GetSenderId(),
		Text:           msg.GetText(),
		CreatedAt:      createdAt,
	}
}
