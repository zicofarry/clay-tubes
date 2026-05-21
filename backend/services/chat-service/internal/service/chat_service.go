package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/zicofarry/clay-app/backend/services/chat-service/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ── Service Errors ───────────────────────────────────────────────────────

type ServiceError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *ServiceError) Error() string {
	return e.Message
}

var (
	ErrRoomNotFound   = &ServiceError{"ROOM_NOT_FOUND", "chat room not found", 404}
	ErrNotParticipant = &ServiceError{"FORBIDDEN", "user is not a participant in this room", 403}
	ErrRoomClosed     = &ServiceError{"ROOM_CLOSED", "chat room is closed", 403}
	ErrInvalidRequest = &ServiceError{"INVALID_REQUEST", "invalid request parameters", 400}
)

// ── DTOs ─────────────────────────────────────────────────────────────────

type MessageDTO struct {
	ID         string    `json:"id"`
	RoomID     string    `json:"room_id"`
	SenderID   string    `json:"sender_id"`
	SenderRole string    `json:"sender_role"`
	Content    string    `json:"content"`
	Type       string    `json:"type"`
	IsRead     bool      `json:"is_read"`
	ClientID   *string   `json:"client_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type ChatRoomDTO struct {
	ID        string     `json:"id"`
	OrderID   string     `json:"order_id"`
	OrderType string     `json:"order_type"`
	UserID    string     `json:"user_id"`
	DriverID  *string    `json:"driver_id"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at"`
}

type ChatRoomWithPreviewDTO struct {
	ChatRoomDTO
	LastMessage *MessageDTO `json:"last_message"`
	UnreadCount int64       `json:"unread_count"`
}

type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type MessageListMeta struct {
	HasMore  bool   `json:"has_more"`
	OldestID string `json:"oldest_id"`
	Count    int    `json:"count"`
}

// ── Interface ────────────────────────────────────────────────────────────

//go:generate mockgen -source=chat_service.go -destination=../../mocks/chat_service_mock.go -package=mocks
type ChatServiceInterface interface {
	// Rooms
	ListMyRooms(ctx context.Context, userID, status string, page, limit int) ([]ChatRoomWithPreviewDTO, *PaginationMeta, error)
	GetRoomByOrderID(ctx context.Context, orderID, participantID string) (*ChatRoomWithPreviewDTO, error)
	GetRoomByID(ctx context.Context, roomID, participantID string) (*ChatRoomWithPreviewDTO, error)
	InternalCreateRoom(ctx context.Context, orderID, orderType, userID string, driverID *string) (*ChatRoomDTO, error)
	InternalCloseRoom(ctx context.Context, roomID string) (*ChatRoomDTO, error)
	InternalAssignDriver(ctx context.Context, orderID, driverID string) (*ChatRoomDTO, error)

	// Messages
	ListMessages(ctx context.Context, roomID, participantID string, beforeID string, limit int) ([]MessageDTO, *MessageListMeta, error)
	SendMessage(ctx context.Context, roomID, senderID, senderRole, content, msgType string, clientID *string) (*MessageDTO, error)
	MarkMessagesAsRead(ctx context.Context, roomID, participantID, upToMessageID string) (int64, error)
	GetUnreadCount(ctx context.Context, roomID, participantID string) (int64, error)
}

type chatService struct {
	repo   repository.ChatRepositoryInterface
	logger *slog.Logger
}

func NewChatService(repo repository.ChatRepositoryInterface, logger *slog.Logger) ChatServiceInterface {
	return &chatService{repo: repo, logger: logger}
}

// ── Room Implementation ──────────────────────────────────────────────────

func (s *chatService) ListMyRooms(ctx context.Context, participantID, status string, page, limit int) ([]ChatRoomWithPreviewDTO, *PaginationMeta, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	rooms, total, err := s.repo.ListRoomsByParticipant(ctx, participantID, status, page, limit)
	if err != nil {
		s.logger.Error("failed to list rooms", slog.Any("error", err))
		return nil, nil, err
	}

	var dtos []ChatRoomWithPreviewDTO
	for _, r := range rooms {
		dto := ChatRoomWithPreviewDTO{
			ChatRoomDTO: *s.roomToDTO(&r),
		}

		lastMsg, err := s.repo.GetLastMessage(ctx, r.ID)
		if err == nil && lastMsg != nil {
			dto.LastMessage = s.msgToDTO(lastMsg)
		}

		unreadCount, err := s.repo.GetUnreadCount(ctx, r.ID, participantID)
		if err == nil {
			dto.UnreadCount = unreadCount
		}

		dtos = append(dtos, dto)
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	meta := &PaginationMeta{
		Page:       page,
		Limit:      limit,
		TotalItems: total,
		TotalPages: totalPages,
	}

	return dtos, meta, nil
}

func (s *chatService) GetRoomByOrderID(ctx context.Context, orderID, participantID string) (*ChatRoomWithPreviewDTO, error) {
	room, err := s.repo.GetRoomByOrderID(ctx, orderID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	return s.enrichRoomWithPreview(ctx, room, participantID)
}

func (s *chatService) GetRoomByID(ctx context.Context, roomID, participantID string) (*ChatRoomWithPreviewDTO, error) {
	objID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return nil, ErrInvalidRequest
	}

	room, err := s.repo.GetRoomByID(ctx, objID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	return s.enrichRoomWithPreview(ctx, room, participantID)
}

func (s *chatService) InternalCreateRoom(ctx context.Context, orderID, orderType, userID string, driverID *string) (*ChatRoomDTO, error) {
	room := &repository.ChatRoom{
		OrderID:   orderID,
		OrderType: orderType,
		UserID:    userID,
		DriverID:  driverID,
		Status:    "active",
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.CreateRoom(ctx, room); err != nil {
		s.logger.Error("failed to create room", slog.Any("error", err))
		return nil, err
	}

	return s.roomToDTO(room), nil
}

func (s *chatService) InternalCloseRoom(ctx context.Context, roomID string) (*ChatRoomDTO, error) {
	objID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return nil, ErrInvalidRequest
	}

	now := time.Now().UTC()
	err = s.repo.UpdateRoomStatus(ctx, objID, "closed", &now)
	if err != nil {
		return nil, err
	}

	room, err := s.repo.GetRoomByID(ctx, objID)
	if err != nil {
		return nil, err
	}

	return s.roomToDTO(room), nil
}

func (s *chatService) InternalAssignDriver(ctx context.Context, orderID, driverID string) (*ChatRoomDTO, error) {
	room, err := s.repo.AssignDriverToRoom(ctx, orderID, driverID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}
	return s.roomToDTO(room), nil
}

// ── Messages Implementation ──────────────────────────────────────────────

func (s *chatService) ListMessages(ctx context.Context, roomID, participantID string, beforeID string, limit int) ([]MessageDTO, *MessageListMeta, error) {
	objRoomID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return nil, nil, ErrInvalidRequest
	}

	if err := s.validateParticipant(ctx, objRoomID, participantID); err != nil {
		return nil, nil, err
	}

	var beforeObjID *primitive.ObjectID
	if beforeID != "" {
		id, err := primitive.ObjectIDFromHex(beforeID)
		if err == nil {
			beforeObjID = &id
		}
	}

	if limit < 1 || limit > 100 {
		limit = 30
	}

	msgs, err := s.repo.ListMessages(ctx, objRoomID, beforeObjID, limit)
	if err != nil {
		return nil, nil, err
	}

	var dtos []MessageDTO
	for _, m := range msgs {
		dtos = append(dtos, *s.msgToDTO(&m))
	}

	meta := &MessageListMeta{
		Count: len(dtos),
	}
	if len(dtos) > 0 {
		meta.OldestID = dtos[len(dtos)-1].ID
		
		// check if has more
		oldestObjID, _ := primitive.ObjectIDFromHex(meta.OldestID)
		moreMsgs, _ := s.repo.ListMessages(ctx, objRoomID, &oldestObjID, 1)
		meta.HasMore = len(moreMsgs) > 0
	} else {
		meta.HasMore = false
	}

	return dtos, meta, nil
}

func (s *chatService) SendMessage(ctx context.Context, roomID, senderID, senderRole, content, msgType string, clientID *string) (*MessageDTO, error) {
	objRoomID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return nil, ErrInvalidRequest
	}

	room, err := s.repo.GetRoomByID(ctx, objRoomID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	if room.Status == "closed" {
		return nil, ErrRoomClosed
	}

	if room.UserID != senderID && (room.DriverID == nil || *room.DriverID != senderID) {
		return nil, ErrNotParticipant
	}

	if clientID != nil && *clientID != "" {
		existing, err := s.repo.GetMessageByClientID(ctx, objRoomID, *clientID)
		if err == nil && existing != nil {
			return s.msgToDTO(existing), nil
		}
	}

	msg := &repository.Message{
		RoomID:     objRoomID,
		SenderID:   senderID,
		SenderRole: senderRole,
		Content:    content,
		Type:       msgType,
		IsRead:     false,
		ClientID:   clientID,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.repo.InsertMessage(ctx, msg); err != nil {
		return nil, err
	}

	// In a real implementation, we would publish a Kafka event `Chat_Message_Sent` here.

	return s.msgToDTO(msg), nil
}

func (s *chatService) MarkMessagesAsRead(ctx context.Context, roomID, participantID, upToMessageID string) (int64, error) {
	objRoomID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return 0, ErrInvalidRequest
	}
	
	upToObjID, err := primitive.ObjectIDFromHex(upToMessageID)
	if err != nil {
		return 0, ErrInvalidRequest
	}

	if err := s.validateParticipant(ctx, objRoomID, participantID); err != nil {
		return 0, err
	}

	count, err := s.repo.MarkMessagesAsRead(ctx, objRoomID, upToObjID)
	if err != nil {
		return 0, err
	}
	
	return count, nil
}

func (s *chatService) GetUnreadCount(ctx context.Context, roomID, participantID string) (int64, error) {
	objRoomID, err := primitive.ObjectIDFromHex(roomID)
	if err != nil {
		return 0, ErrInvalidRequest
	}

	if err := s.validateParticipant(ctx, objRoomID, participantID); err != nil {
		return 0, err
	}

	return s.repo.GetUnreadCount(ctx, objRoomID, participantID)
}

// ── Helpers ──────────────────────────────────────────────────────────────

func (s *chatService) validateParticipant(ctx context.Context, roomID primitive.ObjectID, participantID string) error {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return ErrRoomNotFound
		}
		return err
	}

	if room.UserID != participantID && (room.DriverID == nil || *room.DriverID != participantID) {
		return ErrNotParticipant
	}
	return nil
}

func (s *chatService) enrichRoomWithPreview(ctx context.Context, room *repository.ChatRoom, participantID string) (*ChatRoomWithPreviewDTO, error) {
	if room.UserID != participantID && (room.DriverID == nil || *room.DriverID != participantID) {
		return nil, ErrNotParticipant
	}

	dto := &ChatRoomWithPreviewDTO{
		ChatRoomDTO: *s.roomToDTO(room),
	}

	lastMsg, err := s.repo.GetLastMessage(ctx, room.ID)
	if err == nil && lastMsg != nil {
		dto.LastMessage = s.msgToDTO(lastMsg)
	}

	unreadCount, err := s.repo.GetUnreadCount(ctx, room.ID, participantID)
	if err == nil {
		dto.UnreadCount = unreadCount
	}

	return dto, nil
}

func (s *chatService) roomToDTO(r *repository.ChatRoom) *ChatRoomDTO {
	return &ChatRoomDTO{
		ID:        r.ID.Hex(),
		OrderID:   r.OrderID,
		OrderType: r.OrderType,
		UserID:    r.UserID,
		DriverID:  r.DriverID,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
		ClosedAt:  r.ClosedAt,
	}
}

func (s *chatService) msgToDTO(m *repository.Message) *MessageDTO {
	return &MessageDTO{
		ID:         m.ID.Hex(),
		RoomID:     m.RoomID.Hex(),
		SenderID:   m.SenderID,
		SenderRole: m.SenderRole,
		Content:    m.Content,
		Type:       m.Type,
		IsRead:     m.IsRead,
		ClientID:   m.ClientID,
		CreatedAt:  m.CreatedAt,
	}
}
