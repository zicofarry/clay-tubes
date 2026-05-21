package service

import (
	"context"
	"log/slog"
	"time"
	"errors"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/promotion-service/internal/repository"
	"gorm.io/gorm"
)

type ServiceError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *ServiceError) Error() string {
	return e.Message
}

var (
	ErrNotFound      = &ServiceError{"NOT_FOUND", "resource not found", 404}
	ErrInvalid       = &ServiceError{"INVALID_REQUEST", "invalid request parameters", 400}
	ErrPromoExpired  = &ServiceError{"PROMO_EXPIRED", "promo code has expired or is inactive", 422}
	ErrPromoLimit    = &ServiceError{"PROMO_LIMIT_REACHED", "promo usage limit reached", 422}
	ErrConflict      = &ServiceError{"CONFLICT", "resource already exists", 409}
)

// ── DTOs ─────────────────────────────────────────────────────────────────

type PromoDTO struct {
	PromoID            string   `json:"promo_id"`
	Code               string   `json:"code"`
	Type               string   `json:"type"`
	DiscountValue      float64  `json:"discount_value"`
	MaxDiscountAmount  *float64 `json:"max_discount_amount,omitempty"`
	MinOrderAmount     *float64 `json:"min_order_amount,omitempty"`
	ApplicableService  string   `json:"applicable_service"`
	UsageLimit         int      `json:"usage_limit"`
	PerUserLimit       int      `json:"per_user_limit"`
	CurrentUsageCount  int      `json:"current_usage_count"`
	ValidFrom          string   `json:"valid_from"`
	ValidUntil         string   `json:"valid_until"`
	IsActive           bool     `json:"is_active"`
	CreatedAt          string   `json:"created_at"`
}

type VoucherDTO struct {
	VoucherID         string   `json:"voucher_id"`
	PromoID           string   `json:"promo_id"`
	Code              string   `json:"code"`
	Type              string   `json:"type"`
	DiscountValue     float64  `json:"discount_value"`
	ApplicableService string   `json:"applicable_service"`
	MinOrderAmount    *float64 `json:"min_order_amount,omitempty"`
	ValidUntil        string   `json:"valid_until"`
	Status            string   `json:"status"`
}

type ValidatePromoRequest struct {
	Code          string  `json:"code"`
	UserID        string  `json:"user_id"`
	ServiceType   string  `json:"service_type"`
	OrderAmount   float64 `json:"order_amount"`
	DeliveryFee   *float64 `json:"delivery_fee,omitempty"`
}

type PromoValidationResponse struct {
	PromoID        string  `json:"promo_id"`
	Code           string  `json:"code"`
	Type           string  `json:"type"`
	DiscountAmount float64 `json:"discount_amount"`
	Description    string  `json:"description"`
}

type ApplyPromoRequest struct {
	PromoCode    string  `json:"promo_code"`
	OrderID      string  `json:"order_id"`
	UserID       string  `json:"user_id"`
	ServiceType  string  `json:"service_type"`
	OrderAmount  float64 `json:"order_amount"`
	DeliveryFee  *float64 `json:"delivery_fee,omitempty"`
}

type ApplyPromoResponse struct {
	PromoID        string  `json:"promo_id"`
	DiscountAmount float64 `json:"discount_amount"`
	Type           string  `json:"type"`
}

type ReleasePromoRequest struct {
	PromoCode string `json:"promo_code"`
	OrderID   string `json:"order_id"`
	UserID    string `json:"user_id"`
}

// ── Interface ────────────────────────────────────────────────────────────

//go:generate mockgen -source=promotion_service.go -destination=../../mocks/promotion_service_mock.go -package=mocks
type PromotionServiceInterface interface {
	// Promo
	ValidatePromo(ctx context.Context, req ValidatePromoRequest) (*PromoValidationResponse, error)
	
	// Voucher
	ListMyVouchers(ctx context.Context, userID, serviceType, status string) ([]VoucherDTO, error)
	ClaimVoucher(ctx context.Context, userID, code string) (*VoucherDTO, error)
	
	// Admin
	ListPromos(ctx context.Context, status string, page, limit int) ([]PromoDTO, int64, error)
	CreatePromo(ctx context.Context, promo PromoDTO) (*PromoDTO, error)
	UpdatePromo(ctx context.Context, promoID string, isActive *bool, validUntil *time.Time, usageLimit *int, perUserLimit *int) (*PromoDTO, error)
	DeactivatePromo(ctx context.Context, promoID string) error
	
	// Internal
	ApplyPromo(ctx context.Context, req ApplyPromoRequest) (*ApplyPromoResponse, error)
	ReleasePromo(ctx context.Context, promoCode, orderID, userID string) error
}

type promotionService struct {
	repo   repository.PromotionRepositoryInterface
	logger *slog.Logger
}

func NewPromotionService(repo repository.PromotionRepositoryInterface, logger *slog.Logger) PromotionServiceInterface {
	return &promotionService{repo: repo, logger: logger}
}

// ── Implementation ───────────────────────────────────────────────────────

func (s *promotionService) ValidatePromo(ctx context.Context, req ValidatePromoRequest) (*PromoValidationResponse, error) {
	promo, err := s.repo.GetPromoCodeByCode(ctx, req.Code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if err := s.checkEligibility(promo, req.ServiceType, req.OrderAmount); err != nil {
		return nil, err
	}

	// Calculate discount
	discount := s.calculateDiscount(promo, req.OrderAmount, req.DeliveryFee)

	return &PromoValidationResponse{
		PromoID:        promo.ID.String(),
		Code:           promo.Code,
		Type:           promo.Type,
		DiscountAmount: discount,
		Description:    s.buildDescription(promo),
	}, nil
}

func (s *promotionService) ListMyVouchers(ctx context.Context, userID, serviceType, status string) ([]VoucherDTO, error) {
	uID, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrInvalid
	}

	vouchers, err := s.repo.ListUserPromos(ctx, uID, serviceType, status)
	if err != nil {
		return nil, err
	}

	dtos := make([]VoucherDTO, len(vouchers))
	for i, v := range vouchers {
		dtos[i] = VoucherDTO{
			VoucherID:         v.ID.String(),
			PromoID:           v.PromoID.String(),
			Code:              v.Promo.Code,
			Type:              v.Promo.Type,
			DiscountValue:     v.Promo.Value,
			ApplicableService: v.Promo.ServiceType,
			MinOrderAmount:    v.Promo.MinOrderAmount,
			ValidUntil:        v.ExpiresAt.Format(time.RFC3339),
			Status:            v.Status,
		}
	}
	return dtos, nil
}

func (s *promotionService) ClaimVoucher(ctx context.Context, userID, code string) (*VoucherDTO, error) {
	uID, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrInvalid
	}

	promo, err := s.repo.GetPromoCodeByCode(ctx, code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if !promo.IsActive || time.Now().UTC().After(promo.ValidUntil) {
		return nil, ErrPromoExpired
	}
	if promo.Quota > 0 && promo.UsedCount >= promo.Quota {
		return nil, ErrPromoLimit
	}

	_, err = s.repo.GetUserPromo(ctx, uID, promo.ID)
	if err == nil {
		return nil, ErrConflict // Already claimed
	}

	up := &repository.UserPromo{
		ID:         uuid.New(),
		UserID:     uID,
		PromoID:    promo.ID,
		Status:     "available",
		AssignedAt: time.Now().UTC(),
		ExpiresAt:  promo.ValidUntil,
	}

	if err := s.repo.CreateUserPromo(ctx, up); err != nil {
		return nil, err
	}

	return &VoucherDTO{
		VoucherID:         up.ID.String(),
		PromoID:           promo.ID.String(),
		Code:              promo.Code,
		Type:              promo.Type,
		DiscountValue:     promo.Value,
		ApplicableService: promo.ServiceType,
		MinOrderAmount:    promo.MinOrderAmount,
		ValidUntil:        promo.ValidUntil.Format(time.RFC3339),
		Status:            up.Status,
	}, nil
}

func (s *promotionService) ListPromos(ctx context.Context, status string, page, limit int) ([]PromoDTO, int64, error) {
	offset := (page - 1) * limit
	promos, count, err := s.repo.ListPromoCodes(ctx, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]PromoDTO, len(promos))
	for i, p := range promos {
		dtos[i] = s.promoToDTO(&p)
	}
	return dtos, count, nil
}

func (s *promotionService) CreatePromo(ctx context.Context, dto PromoDTO) (*PromoDTO, error) {
	validFrom, err := time.Parse(time.RFC3339, dto.ValidFrom)
	if err != nil {
		return nil, ErrInvalid
	}
	validUntil, err := time.Parse(time.RFC3339, dto.ValidUntil)
	if err != nil {
		return nil, ErrInvalid
	}

	p := &repository.PromoCode{
		ID:             uuid.New(),
		Code:           dto.Code,
		Type:           dto.Type,
		Value:          dto.DiscountValue,
		MinOrderAmount: dto.MinOrderAmount,
		MaxDiscount:    dto.MaxDiscountAmount,
		Quota:          dto.UsageLimit,
		UsedCount:      0,
		ServiceType:    dto.ApplicableService,
		ValidFrom:      validFrom,
		ValidUntil:     validUntil,
		IsActive:       true,
	}

	if err := s.repo.CreatePromoCode(ctx, p); err != nil {
		return nil, err
	}
	dtoOut := s.promoToDTO(p)
	return &dtoOut, nil
}

func (s *promotionService) UpdatePromo(ctx context.Context, promoID string, isActive *bool, validUntil *time.Time, usageLimit *int, perUserLimit *int) (*PromoDTO, error) {
	pID, err := uuid.Parse(promoID)
	if err != nil {
		return nil, ErrInvalid
	}

	promo, err := s.repo.GetPromoCodeByID(ctx, pID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if isActive != nil {
		promo.IsActive = *isActive
	}
	if validUntil != nil {
		promo.ValidUntil = *validUntil
	}
	if usageLimit != nil {
		promo.Quota = *usageLimit
	}

	if err := s.repo.UpdatePromoCode(ctx, promo); err != nil {
		return nil, err
	}

	dtoOut := s.promoToDTO(promo)
	return &dtoOut, nil
}

func (s *promotionService) DeactivatePromo(ctx context.Context, promoID string) error {
	pID, err := uuid.Parse(promoID)
	if err != nil {
		return ErrInvalid
	}

	promo, err := s.repo.GetPromoCodeByID(ctx, pID)
	if err != nil {
		return err
	}

	promo.IsActive = false
	return s.repo.UpdatePromoCode(ctx, promo)
}

func (s *promotionService) ApplyPromo(ctx context.Context, req ApplyPromoRequest) (*ApplyPromoResponse, error) {
	promo, err := s.repo.GetPromoCodeByCode(ctx, req.PromoCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if err := s.checkEligibility(promo, req.ServiceType, req.OrderAmount); err != nil {
		return nil, err
	}

	discount := s.calculateDiscount(promo, req.OrderAmount, req.DeliveryFee)

	uID, _ := uuid.Parse(req.UserID)
	oID, _ := uuid.Parse(req.OrderID)

	err = s.repo.RunInTx(ctx, func(txRepo repository.PromotionRepositoryInterface) error {
		usage := &repository.PromoUsage{
			ID:             uuid.New(),
			PromoID:        promo.ID,
			UserID:         uID,
			OrderID:        oID,
			DiscountAmount: discount,
			UsedAt:         time.Now().UTC(),
		}
		if err := txRepo.CreatePromoUsage(ctx, usage); err != nil {
			return err
		}
		if err := txRepo.IncrementPromoUsage(ctx, promo.ID); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &ApplyPromoResponse{
		PromoID:        promo.ID.String(),
		DiscountAmount: discount,
		Type:           promo.Type,
	}, nil
}

func (s *promotionService) ReleasePromo(ctx context.Context, promoCode, orderID, userID string) error {
	promo, err := s.repo.GetPromoCodeByCode(ctx, promoCode)
	if err != nil {
		return err
	}

	oID, err := uuid.Parse(orderID)
	if err != nil {
		return ErrInvalid
	}

	return s.repo.ReleasePromoUsage(ctx, promo.ID, oID)
}

// ── Helpers ──────────────────────────────────────────────────────────────

func (s *promotionService) checkEligibility(promo *repository.PromoCode, serviceType string, amount float64) error {
	if !promo.IsActive {
		return ErrPromoExpired
	}
	now := time.Now().UTC()
	if now.Before(promo.ValidFrom) || now.After(promo.ValidUntil) {
		return ErrPromoExpired
	}
	if promo.Quota > 0 && promo.UsedCount >= promo.Quota {
		return ErrPromoLimit
	}
	if promo.ServiceType != "all" && promo.ServiceType != serviceType {
		return ErrInvalid
	}
	if promo.MinOrderAmount != nil && amount < *promo.MinOrderAmount {
		return ErrInvalid
	}
	return nil
}

func (s *promotionService) calculateDiscount(promo *repository.PromoCode, orderAmount float64, deliveryFee *float64) float64 {
	var discount float64
	switch promo.Type {
	case "percentage_off":
		discount = orderAmount * (promo.Value / 100.0)
		if promo.MaxDiscount != nil && discount > *promo.MaxDiscount {
			discount = *promo.MaxDiscount
		}
	case "fixed_off":
		discount = promo.Value
	case "free_delivery":
		if deliveryFee != nil {
			discount = *deliveryFee
			if promo.MaxDiscount != nil && discount > *promo.MaxDiscount {
				discount = *promo.MaxDiscount
			}
		}
	case "cashback":
		// Cashback does not apply a discount directly at checkout
		discount = 0 
	}

	// Cannot discount more than the order amount itself
	if discount > orderAmount {
		discount = orderAmount
	}
	return discount
}

func (s *promotionService) buildDescription(promo *repository.PromoCode) string {
	// Simplify for now
	return "Valid promo code"
}

func (s *promotionService) promoToDTO(p *repository.PromoCode) PromoDTO {
	return PromoDTO{
		PromoID:           p.ID.String(),
		Code:              p.Code,
		Type:              p.Type,
		DiscountValue:     p.Value,
		MaxDiscountAmount: p.MaxDiscount,
		MinOrderAmount:    p.MinOrderAmount,
		ApplicableService: p.ServiceType,
		UsageLimit:        p.Quota,
		CurrentUsageCount: p.UsedCount,
		ValidFrom:         p.ValidFrom.Format(time.RFC3339),
		ValidUntil:        p.ValidUntil.Format(time.RFC3339),
		IsActive:          p.IsActive,
	}
}
