package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ── Models ───────────────────────────────────────────────────────────────────

// AuditLog represents a document in the `audit_logs` MongoDB collection.
type AuditLog struct {
	ID           primitive.ObjectID     `bson:"_id,omitempty"`
	EventID      string                 `bson:"event_id"`
	Service      string                 `bson:"service"`
	Action       string                 `bson:"action"`
	ActorID      string                 `bson:"actor_id"`
	ActorType    string                 `bson:"actor_type"`
	ResourceType string                 `bson:"resource_type"`
	ResourceID   string                 `bson:"resource_id"`
	OldValue     map[string]interface{} `bson:"old_value"` // object or null
	NewValue     map[string]interface{} `bson:"new_value"` // object
	IPAddress    string                 `bson:"ip_address"`
	Metadata     map[string]interface{} `bson:"metadata"`
	CreatedAt    time.Time              `bson:"created_at"`
}

// ── Interface ────────────────────────────────────────────────────────────────

// AuditRepositoryInterface defines the contract for audit log data access.
//go:generate mockgen -source=audit_repository.go -destination=../../mocks/repomock/mock_audit_repository.go -package=repomock
type AuditRepositoryInterface interface {
	Insert(ctx context.Context, log *AuditLog) error
	InsertBatch(ctx context.Context, logs []interface{}) (int, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*AuditLog, error)
	Search(ctx context.Context, filter bson.M, skip, limit int64) ([]*AuditLog, int64, error)
	ExistsByEventID(ctx context.Context, eventID string) (bool, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

type AuditRepository struct {
	collection *mongo.Collection
}

func NewAuditRepository(db *mongo.Database) *AuditRepository {
	return &AuditRepository{
		collection: db.Collection("audit_logs"),
	}
}

func (r *AuditRepository) Insert(ctx context.Context, log *AuditLog) error {
	res, err := r.collection.InsertOne(ctx, log)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		log.ID = oid
	}
	return nil
}

func (r *AuditRepository) InsertBatch(ctx context.Context, logs []interface{}) (int, error) {
	// Use ordered: false to continue inserting even if some fail (e.g. duplicate key)
	opts := options.InsertMany().SetOrdered(false)
	res, err := r.collection.InsertMany(ctx, logs, opts)
	if err != nil {
		// Ignore duplicate key errors if using ordered=false and event_id is unique index
		if mongo.IsDuplicateKeyError(err) {
			return len(res.InsertedIDs), nil
		}
		return len(res.InsertedIDs), err
	}
	return len(res.InsertedIDs), nil
}

func (r *AuditRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*AuditLog, error) {
	var log AuditLog
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&log)
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *AuditRepository) Search(ctx context.Context, filter bson.M, skip, limit int64) ([]*AuditLog, int64, error) {
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit)

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var logs []*AuditLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *AuditRepository) ExistsByEventID(ctx context.Context, eventID string) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"event_id": eventID}, options.Count().SetLimit(1))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
