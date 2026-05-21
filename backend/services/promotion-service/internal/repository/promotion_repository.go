package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ── Models ───────────────────────────────────────────────────────────────

type PromoCode struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code             string    `gorm:"type:varchar(50);uniqueIndex"`
	Type             string    `gorm:"type:varchar(30)"` // percentage_off, fixed_off, free_delivery, cashback
	Value            float64   `gorm:"type:decimal(10,2);not null"`
	MinOrderAmount   *float64  `gorm:"type:decimal(12,2)"`
	MaxDiscount      *float64  `gorm:"type:decimal(12,2)"`
	Quota            int       `gorm:"type:integer"`
	UsedCount        int       `gorm:"type:integer"`
	ServiceType      string    `gorm:"type:varchar(20)"`
	ValidFrom        time.Time `gorm:"type:timestamp;index"`
	ValidUntil       time.Time `gorm:"type:timestamp;index"`
	IsActive         bool      `gorm:"type:boolean;index"`
}

func (PromoCode) TableName() string {
	return "promo_codes"
}

type PromoTarget struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	PromoID     uuid.UUID `gorm:"type:uuid;index"`
	TargetType  string    `gorm:"type:varchar(30);index"` // all, specific_user
	TargetValue string    `gorm:"type:varchar(100)"`
}

func (PromoTarget) TableName() string {
	return "promo_targets"
}

type UserPromo struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_user_promo"`
	PromoID    uuid.UUID `gorm:"type:uuid;uniqueIndex:idx_user_promo"`
	Status     string    `gorm:"type:varchar(20);index:idx_user_status"` // available, used, expired
	AssignedAt time.Time `gorm:"type:timestamp"`
	ExpiresAt  time.Time `gorm:"type:timestamp;index"`
	Promo      PromoCode `gorm:"foreignKey:PromoID"`
}

func (UserPromo) TableName() string {
	return "user_promos"
}

type PromoUsage struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	PromoID        uuid.UUID `gorm:"type:uuid;index"`
	UserID         uuid.UUID `gorm:"type:uuid;index"`
	OrderID        uuid.UUID `gorm:"type:uuid;uniqueIndex"`
	DiscountAmount float64   `gorm:"type:decimal(12,2)"`
	UsedAt         time.Time `gorm:"type:timestamp"`
}

func (PromoUsage) TableName() string {
	return "promo_usages"
}

// ── Interface ────────────────────────────────────────────────────────────

//go:generate mockgen -source=promotion_repository.go -destination=../../mocks/repomock/promotion_repository_mock.go -package=repomock
type PromotionRepositoryInterface interface {
	// Promo Code (Admin)
	CreatePromoCode(ctx context.Context, promo *PromoCode) error
	UpdatePromoCode(ctx context.Context, promo *PromoCode) error
	GetPromoCodeByID(ctx context.Context, id uuid.UUID) (*PromoCode, error)
	GetPromoCodeByCode(ctx context.Context, code string) (*PromoCode, error)
	ListPromoCodes(ctx context.Context, status string, limit, offset int) ([]PromoCode, int64, error)

	// User Promo (Voucher)
	CreateUserPromo(ctx context.Context, up *UserPromo) error
	ListUserPromos(ctx context.Context, userID uuid.UUID, serviceType, status string) ([]UserPromo, error)
	GetUserPromo(ctx context.Context, userID, promoID uuid.UUID) (*UserPromo, error)

	// Usage & Internal
	CreatePromoUsage(ctx context.Context, usage *PromoUsage) error
	IncrementPromoUsage(ctx context.Context, promoID uuid.UUID) error
	ReleasePromoUsage(ctx context.Context, promoID uuid.UUID, orderID uuid.UUID) error
	GetPromoUsageByOrder(ctx context.Context, orderID uuid.UUID) (*PromoUsage, error)
	
	// Transaction
	RunInTx(ctx context.Context, fn func(txRepo PromotionRepositoryInterface) error) error
}

type promotionRepository struct {
	db *gorm.DB
}

func NewPromotionRepository(db *gorm.DB) PromotionRepositoryInterface {
	return &promotionRepository{db: db}
}

// ── Implementation ───────────────────────────────────────────────────────

func (r *promotionRepository) CreatePromoCode(ctx context.Context, promo *PromoCode) error {
	return r.db.WithContext(ctx).Create(promo).Error
}

func (r *promotionRepository) UpdatePromoCode(ctx context.Context, promo *PromoCode) error {
	return r.db.WithContext(ctx).Save(promo).Error
}

func (r *promotionRepository) GetPromoCodeByID(ctx context.Context, id uuid.UUID) (*PromoCode, error) {
	var p PromoCode
	err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *promotionRepository) GetPromoCodeByCode(ctx context.Context, code string) (*PromoCode, error) {
	var p PromoCode
	err := r.db.WithContext(ctx).First(&p, "code = ?", code).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *promotionRepository) ListPromoCodes(ctx context.Context, status string, limit, offset int) ([]PromoCode, int64, error) {
	var promos []PromoCode
	var count int64

	query := r.db.WithContext(ctx).Model(&PromoCode{})
	now := time.Now().UTC()

	if status == "active" {
		query = query.Where("is_active = ? AND valid_until > ? AND valid_from <= ?", true, now, now)
	} else if status == "inactive" {
		query = query.Where("is_active = ?", false)
	} else if status == "expired" {
		query = query.Where("valid_until <= ?", now)
	}

	err := query.Count(&count).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&promos).Error
	return promos, count, err
}

func (r *promotionRepository) CreateUserPromo(ctx context.Context, up *UserPromo) error {
	return r.db.WithContext(ctx).Create(up).Error
}

func (r *promotionRepository) ListUserPromos(ctx context.Context, userID uuid.UUID, serviceType, status string) ([]UserPromo, error) {
	var userPromos []UserPromo

	// Join with promo_codes to filter by service_type
	query := r.db.WithContext(ctx).Model(&UserPromo{}).
		Joins("JOIN promo_codes pc ON pc.id = user_promos.promo_id").
		Where("user_promos.user_id = ?", userID)

	if serviceType != "" && serviceType != "all" {
		query = query.Where("pc.service_type IN (?, 'all')", serviceType)
	}

	if status != "" {
		query = query.Where("user_promos.status = ?", status)
	}

	err := query.Preload("Promo").Find(&userPromos).Error
	return userPromos, err
}

func (r *promotionRepository) GetUserPromo(ctx context.Context, userID, promoID uuid.UUID) (*UserPromo, error) {
	var up UserPromo
	err := r.db.WithContext(ctx).First(&up, "user_id = ? AND promo_id = ?", userID, promoID).Error
	if err != nil {
		return nil, err
	}
	return &up, nil
}

func (r *promotionRepository) CreatePromoUsage(ctx context.Context, usage *PromoUsage) error {
	return r.db.WithContext(ctx).Create(usage).Error
}

func (r *promotionRepository) IncrementPromoUsage(ctx context.Context, promoID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&PromoCode{}).Where("id = ?", promoID).
		Update("used_count", gorm.Expr("used_count + 1")).Error
}

func (r *promotionRepository) ReleasePromoUsage(ctx context.Context, promoID uuid.UUID, orderID uuid.UUID) error {
	return r.RunInTx(ctx, func(txRepo PromotionRepositoryInterface) error {
		// Decrement usage count
		if err := txRepo.(*promotionRepository).db.Model(&PromoCode{}).Where("id = ?", promoID).
			Update("used_count", gorm.Expr("used_count - 1")).Error; err != nil {
			return err
		}
		// Delete usage log
		if err := txRepo.(*promotionRepository).db.Where("order_id = ?", orderID).Delete(&PromoUsage{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *promotionRepository) GetPromoUsageByOrder(ctx context.Context, orderID uuid.UUID) (*PromoUsage, error) {
	var usage PromoUsage
	err := r.db.WithContext(ctx).First(&usage, "order_id = ?", orderID).Error
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (r *promotionRepository) RunInTx(ctx context.Context, fn func(txRepo PromotionRepositoryInterface) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&promotionRepository{db: tx})
	})
}
