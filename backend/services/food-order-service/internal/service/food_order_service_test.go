//go:build unit

package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/mocks/repomock"
	sharedKafka "github.com/zicofarry/clay-app/backend/pkg/pkg/kafka"
	"go.uber.org/mock/gomock"
)

func setupTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func TestEstimateFare_Success(t *testing.T) {
	svc := NewFoodOrderService(nil, sharedKafka.NewNoopProducer(), setupTestLogger())

	req := model.FareEstimateRequest{
		UserLat: -6.2,
		UserLng: 106.8,
	}

	resp, err := svc.EstimateFare(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DeliveryFeeCents < 5000 {
		t.Errorf("expected delivery fee to be at least 5000, got %d", resp.DeliveryFeeCents)
	}
	if resp.ServiceFeeCents != 1000 {
		t.Errorf("expected service fee to be 1000, got %d", resp.ServiceFeeCents)
	}
}

func TestCreateOrder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockFoodOrderRepositoryInterface(ctrl)
	
	// 1. Check active order
	mockRepo.EXPECT().GetActiveOrderID(gomock.Any(), "user-123").Return("", nil)
	// 2. Create order in DB
	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, order *model.FoodOrder) error {
		order.ID = "order-123"
		return nil
	})
	// 3. Save items to Mongo
	mockRepo.EXPECT().SaveItems(gomock.Any(), gomock.Any()).Return(nil)
	// 4. Set active order
	mockRepo.EXPECT().SetActiveOrder(gomock.Any(), "user-123", "order-123").Return(nil)
	// 5. Add to merchant queue
	mockRepo.EXPECT().AddToMerchantQueue(gomock.Any(), "merchant-1", "order-123", gomock.Any()).Return(nil)

	svc := NewFoodOrderService(mockRepo, sharedKafka.NewNoopProducer(), setupTestLogger())

	req := model.CreateFoodOrderRequest{
		MerchantID: "merchant-1",
		Items: []model.CreateFoodOrderItem{
			{MenuItemID: "item-1", Quantity: 2},
		},
	}

	order, err := svc.CreateOrder(context.Background(), "user-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order == nil {
		t.Fatal("expected order, got nil")
	}
	if order.ID != "order-123" {
		t.Errorf("expected order ID 'order-123', got %s", order.ID)
	}
	if order.Status != model.StatusPending {
		t.Errorf("expected status pending, got %s", order.Status)
	}
}

func TestCreateOrder_ActiveOrderExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockFoodOrderRepositoryInterface(ctrl)
	
	// Check active order returns existing
	mockRepo.EXPECT().GetActiveOrderID(gomock.Any(), "user-123").Return("existing-order", nil)

	svc := NewFoodOrderService(mockRepo, sharedKafka.NewNoopProducer(), setupTestLogger())

	req := model.CreateFoodOrderRequest{
		MerchantID: "merchant-1",
		Items: []model.CreateFoodOrderItem{
			{MenuItemID: "item-1", Quantity: 1},
		},
	}

	_, err := svc.CreateOrder(context.Background(), "user-123", req)
	if err != ErrActiveOrderExists {
		t.Fatalf("expected ErrActiveOrderExists, got %v", err)
	}
}

func TestCancelOrder_Pending_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockFoodOrderRepositoryInterface(ctrl)
	
	order := &model.FoodOrder{
		ID: "order-123",
		UserID: "user-123",
		MerchantID: "merchant-1",
		Status: model.StatusPending,
	}

	mockRepo.EXPECT().GetByID(gomock.Any(), "order-123").Return(order, nil)
	mockRepo.EXPECT().UpdateCancelled(gomock.Any(), "order-123", model.CancelledByUser, "Changed mind").Return(nil)
	mockRepo.EXPECT().ClearActiveOrder(gomock.Any(), "user-123").Return(nil)
	mockRepo.EXPECT().RemoveFromMerchantQueue(gomock.Any(), "merchant-1", "order-123").Return(nil)

	svc := NewFoodOrderService(mockRepo, sharedKafka.NewNoopProducer(), setupTestLogger())

	reason := "Changed mind"
	req := model.CancelOrderRequest{Reason: &reason}

	cancelledOrder, err := svc.CancelOrder(context.Background(), "order-123", "user-123", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cancelledOrder.Status != model.StatusCancelled {
		t.Errorf("expected status cancelled, got %s", cancelledOrder.Status)
	}
}

func TestCancelOrder_Confirmed_GraceExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockFoodOrderRepositoryInterface(ctrl)
	
	pastDeadline := time.Now().Add(-5 * time.Minute)
	order := &model.FoodOrder{
		ID: "order-123",
		UserID: "user-123",
		Status: model.StatusConfirmed,
		CancelDeadline: &pastDeadline,
	}

	mockRepo.EXPECT().GetByID(gomock.Any(), "order-123").Return(order, nil)

	svc := NewFoodOrderService(mockRepo, sharedKafka.NewNoopProducer(), setupTestLogger())

	_, err := svc.CancelOrder(context.Background(), "order-123", "user-123", model.CancelOrderRequest{})
	if err != ErrCancelGraceExpired {
		t.Fatalf("expected ErrCancelGraceExpired, got %v", err)
	}
}

func TestMerchantConfirmOrder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockFoodOrderRepositoryInterface(ctrl)
	
	order := &model.FoodOrder{
		ID: "order-123",
		UserID: "user-123",
		MerchantID: "merchant-1",
		Status: model.StatusPending,
	}

	mockRepo.EXPECT().GetByID(gomock.Any(), "order-123").Return(order, nil)
	mockRepo.EXPECT().UpdateConfirmed(gomock.Any(), "order-123", 20).Return(nil)
	mockRepo.EXPECT().RemoveFromMerchantQueue(gomock.Any(), "merchant-1", "order-123").Return(nil)

	svc := NewFoodOrderService(mockRepo, sharedKafka.NewNoopProducer(), setupTestLogger())

	req := model.MerchantConfirmRequest{EstPrepTimeMin: 20}

	confirmedOrder, err := svc.MerchantConfirmOrder(context.Background(), "order-123", "merchant-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if confirmedOrder.Status != model.StatusConfirmed {
		t.Errorf("expected status confirmed, got %s", confirmedOrder.Status)
	}
}
