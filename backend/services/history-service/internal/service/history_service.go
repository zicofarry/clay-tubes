package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/history-service/internal/repository"
	"gorm.io/gorm"
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
	ErrNotFound       = &ServiceError{"NOT_FOUND", "resource not found", 404}
	ErrInvalidRequest = &ServiceError{"INVALID_REQUEST", "invalid request parameters", 400}
	ErrForbidden      = &ServiceError{"FORBIDDEN", "access denied", 403}
)

// ── DTOs ─────────────────────────────────────────────────────────────────

type OrderHistoryDTO struct {
	ID            string     `json:"id"`
	OrderID       string     `json:"order_id"`
	UserID        string     `json:"user_id"`
	DriverID      *string    `json:"driver_id"`
	OrderType     string     `json:"order_type"`
	ServiceType   string     `json:"service_type"`
	FinalStatus   string     `json:"final_status"`
	OriginAddress string     `json:"origin_address"`
	DestAddress   string     `json:"dest_address"`
	FareTotal     *float64   `json:"fare_total"`
	PaymentMethod string     `json:"payment_method"`
	RatingScore   *int16     `json:"rating_score"`
	CompletedAt   time.Time  `json:"completed_at"`
}

type ActivityFeedDTO struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	EventType   string                 `json:"event_type"`
	Title       string                 `json:"title"`
	Description *string                `json:"description"`
	Metadata    map[string]interface{} `json:"metadata"`
	OrderID     *string                `json:"order_id"`
	CreatedAt   time.Time              `json:"created_at"`
}

type PaginationMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type InternalCreateOrderHistoryRequest struct {
	OrderID       string   `json:"order_id"`
	UserID        string   `json:"user_id"`
	DriverID      *string  `json:"driver_id"`
	OrderType     string   `json:"order_type"`
	ServiceType   string   `json:"service_type"`
	FinalStatus   string   `json:"final_status"`
	OriginAddress string   `json:"origin_address"`
	DestAddress   string   `json:"dest_address"`
	FareTotal     *float64 `json:"fare_total"`
	PaymentMethod string   `json:"payment_method"`
	RatingScore   *int16   `json:"rating_score"`
	CompletedAt   string   `json:"completed_at"`
}

type InternalCreateFeedEntryRequest struct {
	UserID      string                 `json:"user_id"`
	EventType   string                 `json:"event_type"`
	Title       string                 `json:"title"`
	Description *string                `json:"description"`
	OrderID     *string                `json:"order_id"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ── Interface ────────────────────────────────────────────────────────────

//go:generate mockgen -source=history_service.go -destination=../../mocks/history_service_mock.go -package=mocks
type HistoryServiceInterface interface {
	// Orders
	ListMyOrderHistory(ctx context.Context, userID string, orderType, status string, page, limit int) ([]OrderHistoryDTO, *PaginationMeta, error)
	GetOrderHistoryDetail(ctx context.Context, orderID, participantID string) (*OrderHistoryDTO, error)
	ListDriverTripHistory(ctx context.Context, driverID string, orderType, status string, page, limit int) ([]OrderHistoryDTO, *PaginationMeta, error)
	
	// Activity Feed
	GetMyActivityFeed(ctx context.Context, userID string, eventType string, beforeID string, limit int) ([]ActivityFeedDTO, error)
	GetActivityFeedEntry(ctx context.Context, feedID, userID string) (*ActivityFeedDTO, error)

	// Internal Sync
	InternalSyncOrderHistory(ctx context.Context, req InternalCreateOrderHistoryRequest) (*OrderHistoryDTO, error)
	InternalCreateFeedEntry(ctx context.Context, req InternalCreateFeedEntryRequest) (*ActivityFeedDTO, error)
}

type historyService struct {
	repo   repository.HistoryRepositoryInterface
	logger *slog.Logger
}

func NewHistoryService(repo repository.HistoryRepositoryInterface, logger *slog.Logger) HistoryServiceInterface {
	return &historyService{repo: repo, logger: logger}
}

// ── Implementation ───────────────────────────────────────────────────────

func (s *historyService) ListMyOrderHistory(ctx context.Context, userID string, orderType, status string, page, limit int) ([]OrderHistoryDTO, *PaginationMeta, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, nil, ErrInvalidRequest
	}

	histories, total, err := s.repo.ListOrderHistoryByUser(ctx, userUUID, orderType, status, limit, offset)
	if err != nil {
		return nil, nil, err
	}

	dtos := make([]OrderHistoryDTO, len(histories))
	for i, h := range histories {
		dtos[i] = *s.orderHistoryToDTO(&h)
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

func (s *historyService) ListDriverTripHistory(ctx context.Context, driverID string, orderType, status string, page, limit int) ([]OrderHistoryDTO, *PaginationMeta, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	driverUUID, err := uuid.Parse(driverID)
	if err != nil {
		return nil, nil, ErrInvalidRequest
	}

	histories, total, err := s.repo.ListOrderHistoryByDriver(ctx, driverUUID, orderType, status, limit, offset)
	if err != nil {
		return nil, nil, err
	}

	dtos := make([]OrderHistoryDTO, len(histories))
	for i, h := range histories {
		dtos[i] = *s.orderHistoryToDTO(&h)
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

func (s *historyService) GetOrderHistoryDetail(ctx context.Context, orderID, participantID string) (*OrderHistoryDTO, error) {
	orderUUID, err := uuid.Parse(orderID)
	if err != nil {
		return nil, ErrInvalidRequest
	}

	history, err := s.repo.GetOrderHistoryByOrderID(ctx, orderUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Simple authorization: verify the user requesting is either the user or the driver
	if history.UserID.String() != participantID && (history.DriverID == nil || history.DriverID.String() != participantID) {
		return nil, ErrForbidden
	}

	return s.orderHistoryToDTO(history), nil
}

func (s *historyService) GetMyActivityFeed(ctx context.Context, userID string, eventType string, beforeID string, limit int) ([]ActivityFeedDTO, error) {
	if limit < 1 || limit > 50 {
		limit = 20
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrInvalidRequest
	}

	var beforeTime *time.Time
	if beforeID != "" {
		beforeUUID, err := uuid.Parse(beforeID)
		if err == nil {
			feed, err := s.repo.GetActivityFeedByID(ctx, beforeUUID)
			if err == nil {
				beforeTime = &feed.CreatedAt
			}
		}
	}

	feeds, err := s.repo.ListActivityFeedByUser(ctx, userUUID, eventType, limit, beforeTime)
	if err != nil {
		return nil, err
	}

	dtos := make([]ActivityFeedDTO, len(feeds))
	for i, f := range feeds {
		dtos[i] = *s.activityFeedToDTO(&f)
	}

	return dtos, nil
}

func (s *historyService) GetActivityFeedEntry(ctx context.Context, feedID, userID string) (*ActivityFeedDTO, error) {
	feedUUID, err := uuid.Parse(feedID)
	if err != nil {
		return nil, ErrInvalidRequest
	}

	feed, err := s.repo.GetActivityFeedByID(ctx, feedUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if feed.UserID.String() != userID {
		return nil, ErrForbidden
	}

	return s.activityFeedToDTO(feed), nil
}

func (s *historyService) InternalSyncOrderHistory(ctx context.Context, req InternalCreateOrderHistoryRequest) (*OrderHistoryDTO, error) {
	orderUUID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, ErrInvalidRequest
	}

	userUUID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, ErrInvalidRequest
	}

	var driverUUID *uuid.UUID
	if req.DriverID != nil && *req.DriverID != "" {
		dID, err := uuid.Parse(*req.DriverID)
		if err == nil {
			driverUUID = &dID
		}
	}

	completedAt, err := time.Parse(time.RFC3339, req.CompletedAt)
	if err != nil {
		completedAt = time.Now().UTC()
	}

	history := &repository.OrderHistory{
		ID:            uuid.New(),
		OrderID:       orderUUID,
		UserID:        userUUID,
		DriverID:      driverUUID,
		OrderType:     req.OrderType,
		ServiceType:   req.ServiceType,
		FinalStatus:   req.FinalStatus,
		OriginAddress: req.OriginAddress,
		DestAddress:   req.DestAddress,
		FareTotal:     req.FareTotal,
		PaymentMethod: req.PaymentMethod,
		RatingScore:   req.RatingScore,
		CompletedAt:   completedAt,
	}

	err = s.repo.CreateOrUpdateOrderHistory(ctx, history)
	if err != nil {
		s.logger.Error("failed to create or update order history", slog.Any("error", err))
		return nil, err
	}

	// Fetch again to ensure we get the real ID if it was an update
	savedHistory, _ := s.repo.GetOrderHistoryByOrderID(ctx, orderUUID)
	if savedHistory != nil {
		history = savedHistory
	}

	return s.orderHistoryToDTO(history), nil
}

func (s *historyService) InternalCreateFeedEntry(ctx context.Context, req InternalCreateFeedEntryRequest) (*ActivityFeedDTO, error) {
	userUUID, err := uuid.Parse(req.UserID)
	if err != nil {
		return nil, ErrInvalidRequest
	}

	var orderUUID *uuid.UUID
	if req.OrderID != nil && *req.OrderID != "" {
		oID, err := uuid.Parse(*req.OrderID)
		if err == nil {
			orderUUID = &oID
		}
	}

	feed := &repository.ActivityFeed{
		ID:          uuid.New(),
		UserID:      userUUID,
		EventType:   req.EventType,
		Title:       req.Title,
		Description: req.Description,
		Metadata:    req.Metadata,
		OrderID:     orderUUID,
		CreatedAt:   time.Now().UTC(),
	}

	err = s.repo.CreateActivityFeed(ctx, feed)
	if err != nil {
		s.logger.Error("failed to create feed entry", slog.Any("error", err))
		return nil, err
	}

	return s.activityFeedToDTO(feed), nil
}

// ── Helpers ──────────────────────────────────────────────────────────────

func (s *historyService) orderHistoryToDTO(h *repository.OrderHistory) *OrderHistoryDTO {
	var driverID *string
	if h.DriverID != nil {
		dStr := h.DriverID.String()
		driverID = &dStr
	}

	return &OrderHistoryDTO{
		ID:            h.ID.String(),
		OrderID:       h.OrderID.String(),
		UserID:        h.UserID.String(),
		DriverID:      driverID,
		OrderType:     h.OrderType,
		ServiceType:   h.ServiceType,
		FinalStatus:   h.FinalStatus,
		OriginAddress: h.OriginAddress,
		DestAddress:   h.DestAddress,
		FareTotal:     h.FareTotal,
		PaymentMethod: h.PaymentMethod,
		RatingScore:   h.RatingScore,
		CompletedAt:   h.CompletedAt,
	}
}

func (s *historyService) activityFeedToDTO(f *repository.ActivityFeed) *ActivityFeedDTO {
	var orderID *string
	if f.OrderID != nil {
		oStr := f.OrderID.String()
		orderID = &oStr
	}

	return &ActivityFeedDTO{
		ID:          f.ID.String(),
		UserID:      f.UserID.String(),
		EventType:   f.EventType,
		Title:       f.Title,
		Description: f.Description,
		Metadata:    f.Metadata,
		OrderID:     orderID,
		CreatedAt:   f.CreatedAt,
	}
}
