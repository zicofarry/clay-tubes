//go:build functional

package functional

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupMongoDB(t *testing.T) *mongo.Database {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27019"))
	if err != nil {
		t.Fatalf("failed to connect to mongo: %v", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		t.Fatalf("failed to ping mongo: %v", err)
	}

	db := client.Database("tracking_db")
	
	// Clean up collection before test
	db.Collection("trip_routes").DeleteMany(ctx, bson.M{})

	return db
}

func TestTrackingRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Tracking Service (MongoDB Integration)...")
	db := setupMongoDB(t)
	repo := repository.NewTrackingRepository(db)
	ctx := context.Background()

	orderID := uuid.New().String()

	t.Run("Save Trip Route", func(t *testing.T) {
		ts := time.Now()
		route := &repository.TripRoute{
			OrderID: orderID,
			Points: []repository.RoutePoint{
				{Lat: -6.9, Lng: 107.6, Timestamp: ts},
				{Lat: -6.91, Lng: 107.61, Timestamp: ts.Add(5 * time.Minute)},
			},
			TotalDistanceKm: 1.5,
			DurationMinutes: 5,
			StartedAt:       ts,
			EndedAt:         ts.Add(5 * time.Minute),
		}

		err := repo.SaveTripRoute(ctx, route)
		if err != nil {
			t.Fatalf("failed to save trip route: %v", err)
		}
		t.Logf("Saved trip route for order ID: %s", orderID)
	})

	t.Run("Get Trip Route", func(t *testing.T) {
		route, err := repo.GetTripRoute(ctx, orderID)
		if err != nil {
			t.Fatalf("failed to get trip route: %v", err)
		}
		if route == nil {
			t.Fatal("expected route, got nil")
		}
		if route.OrderID != orderID {
			t.Errorf("expected order ID %s, got %s", orderID, route.OrderID)
		}
		if len(route.Points) != 2 {
			t.Errorf("expected 2 points, got %d", len(route.Points))
		}
		t.Logf("Got trip route with %d points, distance: %.2f km", len(route.Points), route.TotalDistanceKm)
	})

	t.Run("Get Non-Existent Trip Route", func(t *testing.T) {
		route, err := repo.GetTripRoute(ctx, "non-existent")
		if err != nil {
			t.Fatalf("expected nil error for not found, got: %v", err)
		}
		if route != nil {
			t.Fatal("expected nil route for non-existent ID")
		}
	})
}
