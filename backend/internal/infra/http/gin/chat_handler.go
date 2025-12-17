package ginserver

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	gin "github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"rentme/internal/app/dto"
	"rentme/internal/app/uow"
	domainlistings "rentme/internal/domain/listings"
	"rentme/internal/infra/messaging"
)

// ChatHTTP exposes chat endpoints.
type ChatHTTP interface {
	ListMyConversations(c *gin.Context)
	ListMessages(c *gin.Context)
	SendMessage(c *gin.Context)
	CreateListingConversation(c *gin.Context)
	CreateDirectConversation(c *gin.Context)
}

// ChatHandler bridges HTTP with messaging gRPC client.
type ChatHandler struct {
	Messaging  *messaging.Client
	UoWFactory uow.UoWFactory
	Logger     *slog.Logger
}

// ListMyConversations returns conversations for the current user (or all for admins).
func (h ChatHandler) ListMyConversations(c *gin.Context) {
	principal, ok := requireRole(c, "")
	if !ok {
		return
	}
	if h.Messaging == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "messaging unavailable"})
		return
	}
	targetUser := principal.ID
	includeAll := false
	if principal.HasRole("admin") {
		if userFilter := strings.TrimSpace(c.Query("user_id")); userFilter != "" {
			targetUser = userFilter
		} else {
			includeAll = true
			targetUser = ""
		}
	}
	limit := parsePositiveIntStrict(c.Query("limit"), 20)
	cursor := c.Query("cursor")

	conversations, next, err := h.Messaging.ListConversations(c.Request.Context(), targetUser, limit, cursor, includeAll)
	if err != nil {
		h.respondMessagingError(c, err, "list conversations")
		return
	}
	collection := dto.ConversationList{
		Items:      make([]dto.Conversation, 0, len(conversations)),
		NextCursor: next,
	}
	for _, conv := range conversations {
		collection.Items = append(collection.Items, dto.Conversation{
			ID:            conv.ID,
			ListingID:     conv.ListingID,
			Participants:  append([]string(nil), conv.Participants...),
			CreatedAt:     conv.CreatedAt,
			LastMessageAt: conv.LastMessageAt,
		})
	}
	c.JSON(http.StatusOK, collection)
}

// ListMessages returns messages for a conversation if the user is a participant or admin.
func (h ChatHandler) ListMessages(c *gin.Context) {
	principal, ok := requireRole(c, "")
	if !ok {
		return
	}
	if h.Messaging == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "messaging unavailable"})
		return
	}
	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation id is required"})
		return
	}

	conversation, err := h.Messaging.GetConversation(c.Request.Context(), conversationID)
	if err != nil {
		h.respondMessagingError(c, err, "load conversation")
		return
	}
	if !principal.HasRole("admin") && !contains(conversation.Participants, principal.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a chat participant"})
		return
	}
	limit := parsePositiveIntStrict(c.Query("limit"), 50)
	cursor := c.Query("cursor")

	messages, next, err := h.Messaging.ListMessages(c.Request.Context(), conversationID, limit, cursor)
	if err != nil {
		h.respondMessagingError(c, err, "list messages")
		return
	}
	collection := dto.ChatMessageList{
		Items:      make([]dto.ChatMessage, 0, len(messages)),
		NextCursor: next,
	}
	for _, msg := range messages {
		collection.Items = append(collection.Items, dto.ChatMessage{
			ID:             msg.ID,
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			Text:           msg.Text,
			CreatedAt:      msg.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, collection)
}

// SendMessage posts a message to a conversation if allowed.
func (h ChatHandler) SendMessage(c *gin.Context) {
	principal, ok := requireRole(c, "")
	if !ok {
		return
	}
	if h.Messaging == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "messaging unavailable"})
		return
	}
	conversationID := c.Param("id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation id is required"})
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}

	conversation, err := h.Messaging.GetConversation(c.Request.Context(), conversationID)
	if err != nil {
		h.respondMessagingError(c, err, "load conversation")
		return
	}
	if !principal.HasRole("admin") && !contains(conversation.Participants, principal.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a chat participant"})
		return
	}
	message, err := h.Messaging.SendMessage(c.Request.Context(), conversationID, principal.ID, req.Text)
	if err != nil {
		h.respondMessagingError(c, err, "send message")
		return
	}
	c.JSON(http.StatusCreated, dto.ChatMessage{
		ID:             message.ID,
		ConversationID: message.ConversationID,
		SenderID:       message.SenderID,
		Text:           message.Text,
		CreatedAt:      message.CreatedAt,
	})
}

// CreateListingConversation gets or creates a host/guest conversation for a listing.
func (h ChatHandler) CreateListingConversation(c *gin.Context) {
	principal, ok := requireRole(c, "")
	if !ok {
		return
	}
	if h.Messaging == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "messaging unavailable"})
		return
	}
	listingID := strings.TrimSpace(c.Param("id"))
	if listingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "listing id is required"})
		return
	}
	if h.UoWFactory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "listings unavailable"})
		return
	}
	unit, err := h.UoWFactory.Begin(c.Request.Context(), uow.TxOptions{ReadOnly: true})
	if err != nil {
		h.logError("begin uow failed", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot load listing"})
		return
	}
	defer unit.Rollback(c.Request.Context())

	listing, err := unit.Listings().ByID(c.Request.Context(), domainlistings.ListingID(listingID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "listing not found"})
		return
	}
	hostID := string(listing.Host)
	if hostID == principal.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot start chat with yourself"})
		return
	}
	conversation, err := h.Messaging.GetOrCreateConversationForListing(c.Request.Context(), listingID, principal.ID, hostID)
	if err != nil {
		h.respondMessagingError(c, err, "create conversation")
		return
	}
	response := dto.Conversation{
		ID:            conversation.ID,
		ListingID:     conversation.ListingID,
		Participants:  append([]string(nil), conversation.Participants...),
		CreatedAt:     conversation.CreatedAt,
		LastMessageAt: conversation.LastMessageAt,
	}
	c.JSON(http.StatusOK, response)
}

// CreateDirectConversation lets admins start a thread with any user.
func (h ChatHandler) CreateDirectConversation(c *gin.Context) {
	principal, ok := requireRole(c, "")
	if !ok {
		return
	}
	if !principal.HasRole("admin") {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}
	if h.Messaging == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "messaging unavailable"})
		return
	}
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	if req.UserID == principal.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot chat with yourself"})
		return
	}
	conversation, err := h.Messaging.GetOrCreateConversationForListing(c.Request.Context(), "", principal.ID, req.UserID)
	if err != nil {
		h.respondMessagingError(c, err, "create direct conversation")
		return
	}
	response := dto.Conversation{
		ID:            conversation.ID,
		ListingID:     conversation.ListingID,
		Participants:  append([]string(nil), conversation.Participants...),
		CreatedAt:     conversation.CreatedAt,
		LastMessageAt: conversation.LastMessageAt,
	}
	c.JSON(http.StatusOK, response)
}

func (h ChatHandler) respondMessagingError(c *gin.Context, err error, action string) {
	if h.Logger != nil {
		h.Logger.Error("messaging call failed", "action", action, "error", err)
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.NotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		case codes.InvalidArgument:
			c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
			return
		case codes.Unauthenticated, codes.PermissionDenied:
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": "messaging unavailable"})
}

func (h ChatHandler) logError(msg string, err error) {
	if h.Logger != nil {
		h.Logger.Error(msg, "error", err)
	}
}

func parsePositiveIntStrict(raw string, def int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return def
	}
	return value
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

var _ ChatHTTP = (*ChatHandler)(nil)
