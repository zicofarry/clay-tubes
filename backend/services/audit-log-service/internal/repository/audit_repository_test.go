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

func TestInsert_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("Success Insert Log", func(mt *mtest.T) {
		repo := NewAuditRepository(mt.DB)

		log := &AuditLog{
			EventID:      uuid.New().String(),
			Service:      "auth-service",
			Action:       "LOGIN",
			ActorID:      uuid.New().String(),
			ActorType:    "user",
			ResourceType: "session",
			ResourceID:   uuid.New().String(),
			IPAddress:    "192.168.1.1",
			CreatedAt:    time.Now(),
		}

		mt.AddMockResponses(mtest.CreateSuccessResponse())

		err := repo.Insert(context.Background(), log)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if log.ID == primitive.NilObjectID {
			t.Error("expected log ID to be populated")
		}
	})
}

func TestFindByID_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("Success Find Log By ID", func(mt *mtest.T) {
		repo := NewAuditRepository(mt.DB)

		logID := primitive.NewObjectID()
		actorID := uuid.New().String()

		mt.AddMockResponses(mtest.CreateCursorResponse(1, "audit.audit_logs", mtest.FirstBatch, bson.D{
			{"_id", logID},
			{"actor_id", actorID},
			{"action", "UPDATE_PROFILE"},
			{"service", "user-service"},
		}))

		log, err := repo.FindByID(context.Background(), logID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if log == nil {
			t.Fatal("expected log to be returned")
		}
		if log.ID != logID {
			t.Errorf("expected log ID %v, got %v", logID, log.ID)
		}
		if log.ActorID != actorID {
			t.Errorf("expected actor ID %v, got %v", actorID, log.ActorID)
		}
	})
}

func TestExistsByEventID_True(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("Returns True when exists", func(mt *mtest.T) {
		repo := NewAuditRepository(mt.DB)

		eventID := uuid.New().String()

		mt.AddMockResponses(mtest.CreateCursorResponse(1, "audit.audit_logs", mtest.FirstBatch, bson.D{
			{"n", 1},
		}))

		exists, err := repo.ExistsByEventID(context.Background(), eventID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !exists {
			t.Error("expected true")
		}
	})
}
