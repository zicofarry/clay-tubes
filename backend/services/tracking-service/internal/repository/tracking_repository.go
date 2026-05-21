package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RoutePoint represents a single GPS coordinate at a specific time
type RoutePoint struct {
	Lat       float64   `bson:"lat" json:"lat"`
	Lng       float64   `bson:"lng" json:"lng"`
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`
}

// TripRoute represents the historical route of a completed order
type TripRoute struct {
	OrderID          string       `bson:"_id" json:"order_id"`
	Points           []RoutePoint `bson:"points" json:"points"`
	TotalDistanceKm  float64      `bson:"total_distance_km" json:"total_distance_km"`
	DurationMinutes  int          `bson:"duration_minutes" json:"duration_minutes"`
	StartedAt        time.Time    `bson:"started_at" json:"started_at"`
	EndedAt          time.Time    `bson:"ended_at" json:"ended_at"`
}

// TrackingRepositoryInterface defines data operations for trip routes
type TrackingRepositoryInterface interface {
	SaveTripRoute(ctx context.Context, route *TripRoute) error
	GetTripRoute(ctx context.Context, orderID string) (*TripRoute, error)
}

// TrackingRepository implements TrackingRepositoryInterface using MongoDB
type TrackingRepository struct {
	collection *mongo.Collection
}

// NewTrackingRepository creates a new MongoDB tracking repository
func NewTrackingRepository(db *mongo.Database) *TrackingRepository {
	return &TrackingRepository{
		collection: db.Collection("trip_routes"),
	}
}

// SaveTripRoute inserts or replaces a completed trip route in MongoDB
func (r *TrackingRepository) SaveTripRoute(ctx context.Context, route *TripRoute) error {
	opts := options.Replace().SetUpsert(true)
	filter := bson.M{"_id": route.OrderID}
	
	_, err := r.collection.ReplaceOne(ctx, filter, route, opts)
	return err
}

// GetTripRoute fetches a completed trip route by order ID
func (r *TrackingRepository) GetTripRoute(ctx context.Context, orderID string) (*TripRoute, error) {
	var route TripRoute
	err := r.collection.FindOne(ctx, bson.M{"_id": orderID}).Decode(&route)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Return nil, nil when not found
		}
		return nil, err
	}
	return &route, nil
}
