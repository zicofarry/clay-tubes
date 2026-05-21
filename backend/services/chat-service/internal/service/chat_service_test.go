//go:build unit

package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/chat-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/chat-service/mocks/repomock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"
)

func TestChatService_SendMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockChatRepositoryInterface(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewChatService(mockRepo, logger)

	ctx := context.Background()
	roomID := primitive.NewObjectID()
	senderID := uuid.New().String()

	t.Run("Success Send Message", func(t *testing.T) {
		mockRoom := &repository.ChatRoom{
			ID:     roomID,
			UserID: senderID,
			Status: "active",
		}

		mockRepo.EXPECT().GetRoomByID(ctx, roomID).Return(mockRoom, nil)
		mockRepo.EXPECT().GetMessageByClientID(ctx, roomID, "local_client_1").Return(nil, errors.New("not found"))
		mockRepo.EXPECT().InsertMessage(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, msg *repository.Message) error {
			msg.ID = primitive.NewObjectID()
			return nil
		})

		clientID := "local_client_1"
		res, err := svc.SendMessage(ctx, roomID.Hex(), senderID, "user", "Hello driver!", "text", &clientID)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if res == nil || res.Content != "Hello driver!" {
			t.Errorf("expected message content, got %v", res)
		}
	})

	t.Run("Room Closed", func(t *testing.T) {
		mockRoom := &repository.ChatRoom{
			ID:     roomID,
			UserID: senderID,
			Status: "closed",
		}

		mockRepo.EXPECT().GetRoomByID(ctx, roomID).Return(mockRoom, nil)

		res, err := svc.SendMessage(ctx, roomID.Hex(), senderID, "user", "Hello driver!", "text", nil)

		if err != ErrRoomClosed {
			t.Errorf("expected ErrRoomClosed, got %v", err)
		}
		if res != nil {
			t.Errorf("expected nil result")
		}
	})
}
