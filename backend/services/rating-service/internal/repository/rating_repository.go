package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// ── Models ───────────────────────────────────────────────────────────────

type Rating struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey"`
	OrderID    uuid.UUID      `gorm:"type:uuid;index:idx_order_rater,unique"`
	OrderType  string         `gorm:"type:varchar(20)"` // ride, food, delivery
	RaterID    uuid.UUID      `gorm:"type:uuid;index:idx_order_rater,unique;index:idx_rater"`
	RateeID    uuid.UUID      `gorm:"type:uuid;index:idx_ratee"`
	RateeType  string         `gorm:"type:varchar(20);index:idx_ratee"` // user, driver, merchant
	Score      int16          `gorm:"type:smallint;not null"`
	Comment    string         `gorm:"type:text"`
	Tags       pq.StringArray `gorm:"type:text[]"`
	CreatedAt  time.Time      `gorm:"type:timestamp;index"`
}

func (Rating) TableName() string {
	return "ratings"
}

type DriverScoreAggregate struct {
	DriverID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	AvgScore     float64   `gorm:"type:decimal(3,2)"`
	TotalRatings int       `gorm:"type:integer"`
	Last30dAvg   float64   `gorm:"type:decimal(3,2)"`
	Score1Count  int       `gorm:"type:integer"`
	Score2Count  int       `gorm:"type:integer"`
	Score3Count  int       `gorm:"type:integer"`
	Score4Count  int       `gorm:"type:integer"`
	Score5Count  int       `gorm:"type:integer"`
	UpdatedAt    time.Time `gorm:"type:timestamp"`
}

func (DriverScoreAggregate) TableName() string {
	return "driver_score_aggregates"
}

type MerchantScoreAggregate struct {
	MerchantID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	AvgScore     float64   `gorm:"type:decimal(3,2)"`
	TotalRatings int       `gorm:"type:integer"`
	Last30dAvg   float64   `gorm:"type:decimal(3,2)"`
	UpdatedAt    time.Time `gorm:"type:timestamp"`
}

func (MerchantScoreAggregate) TableName() string {
	return "merchant_score_aggregates"
}

// ── Interface ────────────────────────────────────────────────────────────

//go:generate mockgen -source=rating_repository.go -destination=../../mocks/repomock/rating_repository_mock.go -package=repomock
type RatingRepositoryInterface interface {
	CreateRating(ctx context.Context, rating *Rating) error
	GetRatingsBySubject(ctx context.Context, subjectType string, subjectID uuid.UUID, limit, offset int) ([]Rating, int64, error)
	GetRatingsByOrder(ctx context.Context, orderID uuid.UUID) ([]Rating, error)
	GetRatingsByRater(ctx context.Context, raterID uuid.UUID, limit, offset int) ([]Rating, int64, error)
	
	GetDriverAggregate(ctx context.Context, driverID uuid.UUID) (*DriverScoreAggregate, error)
	GetMerchantAggregate(ctx context.Context, merchantID uuid.UUID) (*MerchantScoreAggregate, error)
	
	RunInTx(ctx context.Context, fn func(txRepo RatingRepositoryInterface) error) error
}

type ratingRepository struct {
	db *gorm.DB
}

func NewRatingRepository(db *gorm.DB) RatingRepositoryInterface {
	return &ratingRepository{db: db}
}

// ── Implementation ───────────────────────────────────────────────────────

func (r *ratingRepository) CreateRating(ctx context.Context, rating *Rating) error {
	return r.db.WithContext(ctx).Create(rating).Error
}

func (r *ratingRepository) GetRatingsBySubject(ctx context.Context, subjectType string, subjectID uuid.UUID, limit, offset int) ([]Rating, int64, error) {
	var ratings []Rating
	var count int64

	query := r.db.WithContext(ctx).Model(&Rating{}).Where("ratee_type = ? AND ratee_id = ?", subjectType, subjectID)

	err := query.Count(&count).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&ratings).Error
	return ratings, count, err
}

func (r *ratingRepository) GetRatingsByOrder(ctx context.Context, orderID uuid.UUID) ([]Rating, error) {
	var ratings []Rating
	err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Find(&ratings).Error
	return ratings, err
}

func (r *ratingRepository) GetRatingsByRater(ctx context.Context, raterID uuid.UUID, limit, offset int) ([]Rating, int64, error) {
	var ratings []Rating
	var count int64

	query := r.db.WithContext(ctx).Model(&Rating{}).Where("rater_id = ?", raterID)

	err := query.Count(&count).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&ratings).Error
	return ratings, count, err
}

func (r *ratingRepository) GetDriverAggregate(ctx context.Context, driverID uuid.UUID) (*DriverScoreAggregate, error) {
	var agg DriverScoreAggregate
	err := r.db.WithContext(ctx).First(&agg, "driver_id = ?", driverID).Error
	if err != nil {
		return nil, err
	}
	return &agg, nil
}

func (r *ratingRepository) GetMerchantAggregate(ctx context.Context, merchantID uuid.UUID) (*MerchantScoreAggregate, error) {
	var agg MerchantScoreAggregate
	err := r.db.WithContext(ctx).First(&agg, "merchant_id = ?", merchantID).Error
	if err != nil {
		return nil, err
	}
	return &agg, nil
}

func (r *ratingRepository) RunInTx(ctx context.Context, fn func(txRepo RatingRepositoryInterface) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&ratingRepository{db: tx})
	})
}
