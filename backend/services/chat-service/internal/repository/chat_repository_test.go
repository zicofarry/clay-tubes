//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestCreateRoom_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("Success Create Room", func(mt *mtest.T) {
		repo := NewChatRepository(mt.DB)

		room := &ChatRoom{
			OrderID:   uuid.New().String(),
			OrderType: "ride",
			UserID:    uuid.New().String(),
			Status:    "active",
			CreatedAt: time.Now(),
		}

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := repo.CreateRoom(context.Background(), room)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if room.ID == primitive.NilObjectID {
			t.Error("expected room ID to be populated")
		}
	})
}

func TestGetRoomByID_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("Success Get Room", func(mt *mtest.T) {
		repo := NewChatRepository(mt.DB)

		roomID := primitive.NewObjectID()
		userID := uuid.New().String()
		orderID := uuid.New().String()

		mt.AddMockResponses(mtest.CreateCursorResponse(1, "chat.chat_rooms", mtest.FirstBatch, bson.D{
			{"_id", roomID},
			{"order_id", orderID},
			{"user_id", userID},
			{"status", "active"},
		}))

		room, err := repo.GetRoomByID(context.Background(), roomID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if room == nil {
			t.Fatal("expected room to be returned")
		}
		if room.ID != roomID {
			t.Errorf("expected room ID %v, got %v", roomID, room.ID)
		}
		if room.UserID != userID {
			t.Errorf("expected user ID %v, got %v", userID, room.UserID)
		}
	})
}

func TestInsertMessage_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("Success Insert Message", func(mt *mtest.T) {
		repo := NewChatRepository(mt.DB)

		msg := &Message{
			RoomID:     primitive.NewObjectID(),
			SenderID:   uuid.New().String(),
			SenderRole: "user",
			Content:    "Hello!",
			Type:       "text",
			IsRead:     false,
			CreatedAt:  time.Now(),
		}

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := repo.InsertMessage(context.Background(), msg)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if msg.ID == primitive.NilObjectID {
			t.Error("expected message ID to be populated")
		}
	})
}
