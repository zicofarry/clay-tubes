//go:build functional

package functional

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/audit-log-service/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupTestDB(t *testing.T) *mongo.Database {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to local MongoDB instance
	uri := "mongodb://localhost:27018"
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		t.Fatalf("failed to connect to mongodb: %v", err)
	}

	// Wait for db to be ready
	err = client.Ping(ctx, nil)
	if err != nil {
		t.Fatalf("failed to ping mongodb: %v", err)
	}

	db := client.Database("audit_db_test")

	// Clean up collection before test to ensure isolation
	err = db.Collection("audit_logs").Drop(ctx)
	if err != nil {
		t.Logf("collection drop error (ignore if not exists): %v", err)
	}

	// Create unique index on event_id, similar to auth service's SQL schema setup
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "event_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err = db.Collection("audit_logs").Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	return db
}

func TestAuditRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Audit Service (Database Integration)...")
	
	db := setupTestDB(t)
	defer func() {
		if err := db.Client().Disconnect(context.Background()); err != nil {
			t.Logf("error disconnecting from db: %v", err)
		}
	}()

	repo := repository.NewAuditRepository(db)
	ctx := context.Background()

	t.Run("Create and Find Audit Log", func(t *testing.T) {
		eventID := uuid.New().String()
		log := &repository.AuditLog{
			EventID:      eventID,
			Service:      "auth-service",
			Action:       "auth.login_success",
			ActorID:      uuid.New().String(),
			ActorType:    "user",
			ResourceType: "user",
			ResourceID:   uuid.New().String(),
			NewValue:     map[string]interface{}{"status": "logged_in"},
			IPAddress:    "103.28.14.52",
			Metadata:     map[string]interface{}{"user_agent": "Mozilla/5.0"},
			CreatedAt:    time.Now().UTC().Truncate(time.Millisecond), // Truncated to avoid precision mismatch in assertions
		}

		// 1. Create
		err := repo.Insert(ctx, log)
		if err != nil {
			t.Fatalf("failed to insert audit log: %v", err)
		}
		t.Logf("Successfully inserted audit log with ID: %s", log.ID.Hex())

		if log.ID.IsZero() {
			t.Error("expected generated ObjectID")
		}

		// 2. Find by ID
		found, err := repo.FindByID(ctx, log.ID)
		if err != nil {
			t.Fatalf("failed to find audit log by ID: %v", err)
		}
		
		if found.EventID != eventID {
			t.Errorf("expected EventID '%s', got '%s'", eventID, found.EventID)
		}
		if found.Action != "auth.login_success" {
			t.Errorf("expected Action 'auth.login_success', got '%s'", found.Action)
		}

		// 3. Search logs
		filter := bson.M{"event_id": eventID}
		results, count, err := repo.Search(ctx, filter, 0, 10)
		if err != nil {
			t.Fatalf("failed to search audit log: %v", err)
		}

		if count != 1 {
			t.Errorf("expected count 1, got %d", count)
		}
		if len(results) != 1 || results[0].ID != log.ID {
			t.Errorf("expected result to contain the inserted log")
		}

		t.Log("Successfully retrieved audit log from MongoDB")
	})
}
