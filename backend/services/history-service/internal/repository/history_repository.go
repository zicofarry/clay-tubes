package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ── Models ───────────────────────────────────────────────────────────────

type OrderHistory struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID `gorm:"type:uuid;index"`
	DriverID      *uuid.UUID `gorm:"type:uuid;index"`
	OrderID       uuid.UUID `gorm:"type:uuid;uniqueIndex"`
	OrderType     string    `gorm:"type:varchar(30)"`
	ServiceType   string    `gorm:"type:varchar(30)"`
	FinalStatus   string    `gorm:"type:varchar(30);index"`
	OriginAddress string    `gorm:"type:text"`
	DestAddress   string    `gorm:"type:text"`
	FareTotal     *float64  `gorm:"type:decimal(12,2)"`
	PaymentMethod string    `gorm:"type:varchar(20)"`
	RatingScore   *int16    `gorm:"type:smallint"`
	CompletedAt   time.Time `gorm:"type:timestamp;index"`
}

func (OrderHistory) TableName() string {
	return "order_history"
}

// JSONB definition for GORM
type Metadata map[string]interface{}

type ActivityFeed struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;index:idx_user_id_created_at,sort:desc"`
	EventType   string    `gorm:"type:varchar(50);index"`
	Title       string    `gorm:"type:varchar(200);not null"`
	Description *string   `gorm:"type:text"`
	Metadata    Metadata  `gorm:"type:jsonb;serializer:json"`
	OrderID     *uuid.UUID `gorm:"type:uuid"`
	CreatedAt   time.Time `gorm:"type:timestamp;index:idx_user_id_created_at,sort:desc"`
}

func (ActivityFeed) TableName() string {
	return "activity_feed"
}

// ── Interface ────────────────────────────────────────────────────────────

//go:generate mockgen -source=history_repository.go -destination=../../mocks/repomock/history_repository_mock.go -package=repomock
type HistoryRepositoryInterface interface {
	// Order History
	CreateOrUpdateOrderHistory(ctx context.Context, history *OrderHistory) error
	GetOrderHistoryByID(ctx context.Context, id uuid.UUID) (*OrderHistory, error)
	GetOrderHistoryByOrderID(ctx context.Context, orderID uuid.UUID) (*OrderHistory, error)
	ListOrderHistoryByUser(ctx context.Context, userID uuid.UUID, orderType, status string, limit, offset int) ([]OrderHistory, int64, error)
	ListOrderHistoryByDriver(ctx context.Context, driverID uuid.UUID, orderType, status string, limit, offset int) ([]OrderHistory, int64, error)

	// Activity Feed
	CreateActivityFeed(ctx context.Context, feed *ActivityFeed) error
	GetActivityFeedByID(ctx context.Context, id uuid.UUID) (*ActivityFeed, error)
	ListActivityFeedByUser(ctx context.Context, userID uuid.UUID, eventType string, limit int, beforeCreatedAt *time.Time) ([]ActivityFeed, error)
}

type historyRepository struct {
	db *gorm.DB
}

func NewHistoryRepository(db *gorm.DB) HistoryRepositoryInterface {
	return &historyRepository{db: db}
}

// ── Order History Implementation ─────────────────────────────────────────

func (r *historyRepository) CreateOrUpdateOrderHistory(ctx context.Context, history *OrderHistory) error {
	// Upsert based on order_id
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "order_id"}},
		UpdateAll: true,
	}).Create(history).Error
}

func (r *historyRepository) GetOrderHistoryByID(ctx context.Context, id uuid.UUID) (*OrderHistory, error) {
	var history OrderHistory
	err := r.db.WithContext(ctx).First(&history, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

func (r *historyRepository) GetOrderHistoryByOrderID(ctx context.Context, orderID uuid.UUID) (*OrderHistory, error) {
	var history OrderHistory
	err := r.db.WithContext(ctx).First(&history, "order_id = ?", orderID).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

func (r *historyRepository) ListOrderHistoryByUser(ctx context.Context, userID uuid.UUID, orderType, status string, limit, offset int) ([]OrderHistory, int64, error) {
	var histories []OrderHistory
	var count int64

	query := r.db.WithContext(ctx).Model(&OrderHistory{}).Where("user_id = ?", userID)
	if orderType != "" {
		query = query.Where("order_type = ?", orderType)
	}
	if status != "" {
		query = query.Where("final_status = ?", status)
	}

	err := query.Count(&count).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("completed_at DESC").Limit(limit).Offset(offset).Find(&histories).Error
	return histories, count, err
}

func (r *historyRepository) ListOrderHistoryByDriver(ctx context.Context, driverID uuid.UUID, orderType, status string, limit, offset int) ([]OrderHistory, int64, error) {
	var histories []OrderHistory
	var count int64

	query := r.db.WithContext(ctx).Model(&OrderHistory{}).Where("driver_id = ?", driverID)
	if orderType != "" {
		query = query.Where("order_type = ?", orderType)
	}
	if status != "" {
		query = query.Where("final_status = ?", status)
	}

	err := query.Count(&count).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("completed_at DESC").Limit(limit).Offset(offset).Find(&histories).Error
	return histories, count, err
}

// ── Activity Feed Implementation ─────────────────────────────────────────

func (r *historyRepository) CreateActivityFeed(ctx context.Context, feed *ActivityFeed) error {
	return r.db.WithContext(ctx).Create(feed).Error
}

func (r *historyRepository) GetActivityFeedByID(ctx context.Context, id uuid.UUID) (*ActivityFeed, error) {
	var feed ActivityFeed
	err := r.db.WithContext(ctx).First(&feed, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &feed, nil
}

func (r *historyRepository) ListActivityFeedByUser(ctx context.Context, userID uuid.UUID, eventType string, limit int, beforeCreatedAt *time.Time) ([]ActivityFeed, error) {
	var feeds []ActivityFeed

	query := r.db.WithContext(ctx).Model(&ActivityFeed{}).Where("user_id = ?", userID)
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if beforeCreatedAt != nil {
		query = query.Where("created_at < ?", beforeCreatedAt)
	}

	err := query.Order("created_at DESC").Limit(limit).Find(&feeds).Error
	return feeds, err
}
