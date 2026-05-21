//go:build unit

package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/history-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/history-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func TestHistoryService_InternalSyncOrderHistory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockHistoryRepositoryInterface(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewHistoryService(mockRepo, logger)

	ctx := context.Background()

	t.Run("Success Sync History", func(t *testing.T) {
		orderID := uuid.New().String()
		userID := uuid.New().String()
		
		req := InternalCreateOrderHistoryRequest{
			OrderID:       orderID,
			UserID:        userID,
			OrderType:     "ride",
			ServiceType:   "goride",
			FinalStatus:   "completed",
			CompletedAt:   time.Now().Format(time.RFC3339),
		}

		mockRepo.EXPECT().CreateOrUpdateOrderHistory(ctx, gomock.Any()).Return(nil)
		
		mockOrder := &repository.OrderHistory{
			ID:          uuid.New(),
			OrderID:     uuid.MustParse(orderID),
			UserID:      uuid.MustParse(userID),
			OrderType:   "ride",
			FinalStatus: "completed",
		}
		mockRepo.EXPECT().GetOrderHistoryByOrderID(ctx, uuid.MustParse(orderID)).Return(mockOrder, nil)

		res, err := svc.InternalSyncOrderHistory(ctx, req)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if res == nil || res.OrderID != orderID {
			t.Errorf("expected history result with order_id %s, got %v", orderID, res)
		}
	})
}
