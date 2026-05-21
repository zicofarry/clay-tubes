package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/audit-log-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/audit-log-service/mocks/repomock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"
)

func TestAuditService_CreateLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockAuditRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewAuditService(mockRepo, logger)

	ctx := context.Background()
	req := &CreateAuditLogRequest{
		ActorID:      uuid.New().String(),
		ActorType:    "admin",
		Action:       "user.suspended",
		ResourceType: "user",
		ResourceID:   uuid.New().String(),
		Changes:      map[string]interface{}{"status": map[string]interface{}{"old": "active", "new": "suspended"}},
		IPAddress:    "127.0.0.1",
		UserAgent:    "Go-http-client/1.1",
		CreatedAt:    time.Now(),
	}

	mockRepo.EXPECT().
		Insert(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, log *repository.AuditLog) error {
			// Mock DB setting ID
			log.ID = primitive.NewObjectID()
			return nil
		})

	resp, err := service.CreateLog(ctx, req)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp == nil {
		t.Fatalf("expected response, got nil")
	}

	if resp.Action != req.Action {
		t.Errorf("expected action %s, got %s", req.Action, resp.Action)
	}
}

func TestAuditService_GetLog(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockAuditRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewAuditService(mockRepo, logger)

	ctx := context.Background()
	oid := primitive.NewObjectID()
	logID := oid.Hex()

	expectedLog := &repository.AuditLog{
		ID:           oid,
		EventID:      uuid.New().String(),
		Action:       "order.cancelled",
		ActorID:      uuid.New().String(),
		ActorType:    "user",
		ResourceType: "order",
		ResourceID:   uuid.New().String(),
		CreatedAt:    time.Now(),
	}

	mockRepo.EXPECT().
		FindByID(ctx, oid).
		Return(expectedLog, nil)

	resp, err := service.GetLog(ctx, logID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.ID != logID {
		t.Errorf("expected log ID %s, got %s", logID, resp.ID)
	}
	if resp.Action != expectedLog.Action {
		t.Errorf("expected action %s, got %s", expectedLog.Action, resp.Action)
	}
}
