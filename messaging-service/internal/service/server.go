package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gocql/gocql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"messaging-service/internal/storage/scylla"
	pb "messaging-service/proto"
)

// Server implements the MessagingService gRPC contract.
type Server struct {
	pb.UnimplementedMessagingServiceServer
	Store  *scylla.Store
	Logger *slog.Logger
}

// GetOrCreateConversationForListing returns an existing host<->guest thread or creates a new one.
func (s *Server) GetOrCreateConversationForListing(ctx context.Context, req *pb.GetOrCreateConversationForListingRequest) (*pb.GetConversationResponse, error) {
	if s.Store == nil {
		return nil, status.Error(codes.Unavailable, "store unavailable")
	}
	listingID := strings.TrimSpace(req.GetListingId())
	guestID := strings.TrimSpace(req.GetGuestId())
	hostID := strings.TrimSpace(req.GetHostId())
	if guestID == "" || hostID == "" {
		return nil, status.Error(codes.InvalidArgument, "guest_id and host_id are required")
	}
	participants := []string{guestID, hostID}

	conversation, err := s.Store.FindConversationByListing(ctx, listingID, participants)
	if err != nil && err != gocql.ErrNotFound {
		return nil, status.Errorf(codes.Internal, "lookup conversation: %v", err)
	}
	if conversation == nil {
		conversation, err = s.Store.CreateConversation(ctx, listingID, participants, time.Now())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "create conversation: %v", err)
		}
		if s.Logger != nil {
			s.Logger.Info("conversation created", "id", conversation.ID.String(), "listing_id", listingID, "participants", conversation.Participants)
		}
	}
	return &pb.GetConversationResponse{Conversation: toProtoConversation(conversation)}, nil
}

// GetConversation fetches a conversation by id.
func (s *Server) GetConversation(ctx context.Context, req *pb.GetConversationRequest) (*pb.GetConversationResponse, error) {
	if s.Store == nil {
		return nil, status.Error(codes.Unavailable, "store unavailable")
	}
	conversation, err := s.Store.GetConversation(ctx, req.GetConversationId())
	if err != nil {
		if errorsIsNotFound(err) {
			return nil, status.Error(codes.NotFound, "conversation not found")
		}
		return nil, status.Errorf(codes.Internal, "load conversation: %v", err)
	}
	return &pb.GetConversationResponse{Conversation: toProtoConversation(conversation)}, nil
}

// SendMessage stores a message inside the conversation.
func (s *Server) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	if s.Store == nil {
		return nil, status.Error(codes.Unavailable, "store unavailable")
	}
	conversationID := strings.TrimSpace(req.GetConversationId())
	senderID := strings.TrimSpace(req.GetSenderId())
	text := strings.TrimSpace(req.GetText())
	if conversationID == "" || senderID == "" || text == "" {
		return nil, status.Error(codes.InvalidArgument, "conversation_id, sender_id and text are required")
	}
	conversation, err := s.Store.GetConversation(ctx, conversationID)
	if err != nil {
		if errorsIsNotFound(err) {
			return nil, status.Error(codes.NotFound, "conversation not found")
		}
		return nil, status.Errorf(codes.Internal, "load conversation: %v", err)
	}
	msg, err := s.Store.AddMessage(ctx, conversation.ID, senderID, text, time.Now())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save message: %v", err)
	}
	return &pb.SendMessageResponse{Message: toProtoMessage(msg, conversation)}, nil
}

// ListMessages returns messages in reverse chronological order with cursor pagination.
func (s *Server) ListMessages(ctx context.Context, req *pb.ListMessagesRequest) (*pb.ListMessagesResponse, error) {
	if s.Store == nil {
		return nil, status.Error(codes.Unavailable, "store unavailable")
	}
	conversationID := strings.TrimSpace(req.GetConversationId())
	if conversationID == "" {
		return nil, status.Error(codes.InvalidArgument, "conversation_id is required")
	}
	conversation, err := s.Store.GetConversation(ctx, conversationID)
	if err != nil {
		if errorsIsNotFound(err) {
			return nil, status.Error(codes.NotFound, "conversation not found")
		}
		return nil, status.Errorf(codes.Internal, "load conversation: %v", err)
	}
	var before *gocql.UUID
	if trimmed := strings.TrimSpace(req.GetBefore()); trimmed != "" {
		cursor, err := gocql.ParseUUID(trimmed)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid before cursor")
		}
		before = &cursor
	}
	limit := int(req.GetLimit())
	messages, err := s.Store.ListMessages(ctx, conversation.ID, limit, before)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list messages: %v", err)
	}
	resp := &pb.ListMessagesResponse{Messages: make([]*pb.Message, 0, len(messages))}
	for _, msg := range messages {
		resp.Messages = append(resp.Messages, toProtoMessage(&msg, conversation))
	}
	if len(messages) == normalizeLimit(limit) {
		resp.NextCursor = messages[len(messages)-1].ID.String()
	}
	return resp, nil
}

// ListConversations returns conversations for a user or all conversations for admins.
func (s *Server) ListConversations(ctx context.Context, req *pb.ListConversationsRequest) (*pb.ListConversationsResponse, error) {
	if s.Store == nil {
		return nil, status.Error(codes.Unavailable, "store unavailable")
	}
	userID := strings.TrimSpace(req.GetUserId())
	includeAll := req.GetIncludeAll()
	if userID == "" && !includeAll {
		return nil, status.Error(codes.InvalidArgument, "user_id is required unless include_all is true")
	}

	conversations, err := s.Store.ListConversations(ctx, userID, includeAll)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list conversations: %v", err)
	}
	cursorTime, cursorID, _ := parseCursor(req.GetCursor())
	limit := normalizeLimit(int(req.GetLimit()))

	resp := &pb.ListConversationsResponse{Conversations: make([]*pb.Conversation, 0, limit)}
	for _, conv := range conversations {
		if cursorID != "" {
			activity := lastActivity(conv)
			if activity.After(cursorTime) {
				continue
			}
			if activity.Equal(cursorTime) && conv.ID.String() >= cursorID {
				continue
			}
		}
		resp.Conversations = append(resp.Conversations, toProtoConversation(&conv))
		if len(resp.Conversations) == limit {
			break
		}
	}
	if len(resp.Conversations) == limit {
		lastConv := resp.Conversations[len(resp.Conversations)-1]
		resp.NextCursor = buildCursor(lastConv)
	}
	return resp, nil
}

func toProtoConversation(conv *scylla.Conversation) *pb.Conversation {
	if conv == nil {
		return nil
	}
	return &pb.Conversation{
		Id:            conv.ID.String(),
		ListingId:     conv.ListingID,
		Participants:  append([]string(nil), conv.Participants...),
		CreatedAt:     tsOrNil(conv.CreatedAt),
		LastMessageAt: tsOrNil(conv.LastMessageAt),
	}
}

func toProtoMessage(msg *scylla.Message, conv *scylla.Conversation) *pb.Message {
	if msg == nil {
		return nil
	}
	return &pb.Message{
		Id:             msg.ID.String(),
		ConversationId: msg.ConversationID.String(),
		SenderId:       msg.SenderID,
		Text:           msg.Text,
		CreatedAt:      tsOrNil(msg.CreatedAt),
	}
}

func tsOrNil(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func normalizeLimit(limit int) int {
	if limit <= 0 || limit > 200 {
		return 50
	}
	return limit
}

func errorsIsNotFound(err error) bool {
	return err == gocql.ErrNotFound || strings.Contains(strings.ToLower(err.Error()), "not found")
}

func parseCursor(raw string) (time.Time, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, "", nil
	}
	parts := strings.Split(trimmed, "|")
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, "", err
	}
	cursorTime := time.Unix(0, nanos).UTC()
	return cursorTime, parts[1], nil
}

func buildCursor(conv *pb.Conversation) string {
	if conv == nil || conv.CreatedAt == nil {
		return ""
	}
	activity := conv.CreatedAt.AsTime()
	if conv.LastMessageAt != nil {
		activity = conv.LastMessageAt.AsTime()
	}
	return fmt.Sprintf("%d|%s", activity.UTC().UnixNano(), conv.GetId())
}

func lastActivity(conv scylla.Conversation) time.Time {
	if !conv.LastMessageAt.IsZero() {
		return conv.LastMessageAt
	}
	return conv.CreatedAt
}
