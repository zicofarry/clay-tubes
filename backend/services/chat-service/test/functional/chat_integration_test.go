//go:build functional

package functional

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/chat-service/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupTestDB(t *testing.T) *mongo.Database {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uri := "mongodb://localhost:27017" // Using standard internal port mapping for test DB
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		t.Fatalf("failed to connect to mongodb: %v", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		t.Fatalf("failed to ping mongodb: %v", err)
	}

	db := client.Database("chat_db_test")

	// Clean up collection before test
	_ = db.Collection("chat_rooms").Drop(ctx)
	_ = db.Collection("messages").Drop(ctx)

	// Create Indexes
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "order_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err = db.Collection("chat_rooms").Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	return db
}

func TestChatRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Chat Service (Database Integration)...")
	
	db := setupTestDB(t)
	defer func() {
		if err := db.Client().Disconnect(context.Background()); err != nil {
			t.Logf("error disconnecting from db: %v", err)
		}
	}()

	repo := repository.NewChatRepository(db)
	ctx := context.Background()

	t.Run("Create Room and Send Message", func(t *testing.T) {
		orderID := uuid.New().String()
		userID := uuid.New().String()

		room := &repository.ChatRoom{
			OrderID:   orderID,
			OrderType: "ride",
			UserID:    userID,
			Status:    "active",
			CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		}

		// 1. Create Room
		err := repo.CreateRoom(ctx, room)
		if err != nil {
			t.Fatalf("failed to create chat room: %v", err)
		}

		if room.ID.IsZero() {
			t.Error("expected generated ObjectID for room")
		}

		// 2. Insert Message
		clientID := "client_msg_1"
		msg := &repository.Message{
			RoomID:     room.ID,
			SenderID:   userID,
			SenderRole: "user",
			Content:    "Hello Driver",
			Type:       "text",
			IsRead:     false,
			ClientID:   &clientID,
			CreatedAt:  time.Now().UTC().Truncate(time.Millisecond),
		}

		err = repo.InsertMessage(ctx, msg)
		if err != nil {
			t.Fatalf("failed to insert message: %v", err)
		}

		if msg.ID.IsZero() {
			t.Error("expected generated ObjectID for message")
		}

		// 3. Find Room by Order ID
		foundRoom, err := repo.GetRoomByOrderID(ctx, orderID)
		if err != nil {
			t.Fatalf("failed to find room: %v", err)
		}

		if foundRoom.ID != room.ID {
			t.Errorf("expected room ID %s, got %s", room.ID.Hex(), foundRoom.ID.Hex())
		}

		t.Log("Successfully tested MongoDB integration for chat rooms and messages")
	})
}
