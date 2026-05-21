package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ── Models ───────────────────────────────────────────────────────────────

type ChatRoom struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	OrderID   string             `bson:"order_id"`
	OrderType string             `bson:"order_type"`
	UserID    string             `bson:"user_id"`
	DriverID  *string            `bson:"driver_id"` // can be null
	Status    string             `bson:"status"`    // active, closed
	CreatedAt time.Time          `bson:"created_at"`
	ClosedAt  *time.Time         `bson:"closed_at"` // can be null
}

type Message struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	RoomID     primitive.ObjectID `bson:"room_id"`
	SenderID   string             `bson:"sender_id"`
	SenderRole string             `bson:"sender_role"`
	Content    string             `bson:"content"`
	Type       string             `bson:"type"`
	IsRead     bool               `bson:"is_read"`
	ClientID   *string            `bson:"client_id"` // used for idempotency
	CreatedAt  time.Time          `bson:"created_at"`
}

// ── Interface ────────────────────────────────────────────────────────────

//go:generate mockgen -source=chat_repository.go -destination=../../mocks/repomock/chat_repository_mock.go -package=repomock
type ChatRepositoryInterface interface {
	// Rooms
	CreateRoom(ctx context.Context, room *ChatRoom) error
	GetRoomByID(ctx context.Context, roomID primitive.ObjectID) (*ChatRoom, error)
	GetRoomByOrderID(ctx context.Context, orderID string) (*ChatRoom, error)
	ListRoomsByParticipant(ctx context.Context, participantID string, status string, page, limit int) ([]ChatRoom, int64, error)
	UpdateRoomStatus(ctx context.Context, roomID primitive.ObjectID, status string, closedAt *time.Time) error
	AssignDriverToRoom(ctx context.Context, orderID, driverID string) (*ChatRoom, error)

	// Messages
	InsertMessage(ctx context.Context, msg *Message) error
	GetMessageByClientID(ctx context.Context, roomID primitive.ObjectID, clientID string) (*Message, error)
	ListMessages(ctx context.Context, roomID primitive.ObjectID, beforeID *primitive.ObjectID, limit int) ([]Message, error)
	MarkMessagesAsRead(ctx context.Context, roomID primitive.ObjectID, upToMessageID primitive.ObjectID) (int64, error)
	GetUnreadCount(ctx context.Context, roomID primitive.ObjectID, participantID string) (int64, error)
	GetLastMessage(ctx context.Context, roomID primitive.ObjectID) (*Message, error)
}

type chatRepository struct {
	db *mongo.Database
}

func NewChatRepository(db *mongo.Database) ChatRepositoryInterface {
	return &chatRepository{db: db}
}

// ── Rooms Implementation ─────────────────────────────────────────────────

func (r *chatRepository) CreateRoom(ctx context.Context, room *ChatRoom) error {
	res, err := r.db.Collection("chat_rooms").InsertOne(ctx, room)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// Find existing and set ID to simulate idempotent creation
			existing, _ := r.GetRoomByOrderID(ctx, room.OrderID)
			if existing != nil {
				room.ID = existing.ID
				return nil
			}
		}
		return err
	}
	room.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *chatRepository) GetRoomByID(ctx context.Context, roomID primitive.ObjectID) (*ChatRoom, error) {
	var room ChatRoom
	err := r.db.Collection("chat_rooms").FindOne(ctx, bson.M{"_id": roomID}).Decode(&room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *chatRepository) GetRoomByOrderID(ctx context.Context, orderID string) (*ChatRoom, error) {
	var room ChatRoom
	err := r.db.Collection("chat_rooms").FindOne(ctx, bson.M{"order_id": orderID}).Decode(&room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *chatRepository) ListRoomsByParticipant(ctx context.Context, participantID string, status string, page, limit int) ([]ChatRoom, int64, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"user_id": participantID},
			{"driver_id": participantID},
		},
	}
	if status != "" {
		filter["status"] = status
	}

	total, err := r.db.Collection("chat_rooms").CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip := (page - 1) * limit
	opts := options.Find().
		SetSort(bson.M{"created_at": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.db.Collection("chat_rooms").Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var rooms []ChatRoom
	if err := cursor.All(ctx, &rooms); err != nil {
		return nil, 0, err
	}

	return rooms, total, nil
}

func (r *chatRepository) UpdateRoomStatus(ctx context.Context, roomID primitive.ObjectID, status string, closedAt *time.Time) error {
	update := bson.M{"$set": bson.M{"status": status, "closed_at": closedAt}}
	_, err := r.db.Collection("chat_rooms").UpdateOne(ctx, bson.M{"_id": roomID}, update)
	return err
}

func (r *chatRepository) AssignDriverToRoom(ctx context.Context, orderID, driverID string) (*ChatRoom, error) {
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	update := bson.M{"$set": bson.M{"driver_id": driverID}}
	
	var room ChatRoom
	err := r.db.Collection("chat_rooms").FindOneAndUpdate(ctx, bson.M{"order_id": orderID}, update, opts).Decode(&room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// ── Messages Implementation ──────────────────────────────────────────────

func (r *chatRepository) InsertMessage(ctx context.Context, msg *Message) error {
	res, err := r.db.Collection("messages").InsertOne(ctx, msg)
	if err != nil {
		return err
	}
	msg.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *chatRepository) GetMessageByClientID(ctx context.Context, roomID primitive.ObjectID, clientID string) (*Message, error) {
	var msg Message
	err := r.db.Collection("messages").FindOne(ctx, bson.M{"room_id": roomID, "client_id": clientID}).Decode(&msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func (r *chatRepository) ListMessages(ctx context.Context, roomID primitive.ObjectID, beforeID *primitive.ObjectID, limit int) ([]Message, error) {
	filter := bson.M{"room_id": roomID}
	if beforeID != nil {
		filter["_id"] = bson.M{"$lt": *beforeID}
	}

	opts := options.Find().
		SetSort(bson.M{"_id": -1}). // newest first
		SetLimit(int64(limit))

	cursor, err := r.db.Collection("messages").Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var messages []Message
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, err
	}

	return messages, nil
}

func (r *chatRepository) MarkMessagesAsRead(ctx context.Context, roomID primitive.ObjectID, upToMessageID primitive.ObjectID) (int64, error) {
	filter := bson.M{
		"room_id": roomID,
		"_id":     bson.M{"$lte": upToMessageID},
		"is_read": false,
	}
	update := bson.M{"$set": bson.M{"is_read": true}}

	res, err := r.db.Collection("messages").UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

func (r *chatRepository) GetUnreadCount(ctx context.Context, roomID primitive.ObjectID, participantID string) (int64, error) {
	// A message is unread for a participant if the sender is NOT the participant and is_read == false
	filter := bson.M{
		"room_id":   roomID,
		"sender_id": bson.M{"$ne": participantID},
		"is_read":   false,
	}
	return r.db.Collection("messages").CountDocuments(ctx, filter)
}

func (r *chatRepository) GetLastMessage(ctx context.Context, roomID primitive.ObjectID) (*Message, error) {
	opts := options.FindOne().SetSort(bson.M{"_id": -1})
	var msg Message
	err := r.db.Collection("messages").FindOne(ctx, bson.M{"room_id": roomID}, opts).Decode(&msg)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Return nil if no messages
		}
		return nil, err
	}
	return &msg, nil
}