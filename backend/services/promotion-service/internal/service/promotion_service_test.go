//go:build unit

package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/promotion-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/promotion-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func TestPromotionService_ValidatePromo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockPromotionRepositoryInterface(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewPromotionService(mockRepo, logger)

	ctx := context.Background()

	t.Run("Success Validate Promo", func(t *testing.T) {
		req := ValidatePromoRequest{
			Code:        "PROMO10",
			UserID:      uuid.New().String(),
			ServiceType: "ride",
			OrderAmount: 50000,
		}

		mockPromo := &repository.PromoCode{
			ID:          uuid.New(),
			Code:        "PROMO10",
			Type:        "fixed_off",
			Value:       10000,
			ServiceType: "all",
			IsActive:    true,
			ValidFrom:   time.Now().Add(-24 * time.Hour),
			ValidUntil:  time.Now().Add(24 * time.Hour),
			Quota:       100,
			UsedCount:   10,
		}

		mockRepo.EXPECT().GetPromoCodeByCode(ctx, req.Code).Return(mockPromo, nil)

		res, err := svc.ValidatePromo(ctx, req)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if res == nil || res.DiscountAmount != 10000 {
			t.Errorf("expected discount 10000, got %v", res)
		}
	})
}
