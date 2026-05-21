//go:build unit

// Package repository_test provides unit tests for the tracking repository layer.
// Uses an external test package to avoid import cycles with the repomock package.
package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

// ── SaveTripRoute ────────────────────────────────────────────────────────────

func TestSaveTripRoute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockTrackingRepositoryInterface(ctrl)

	route := &repository.TripRoute{
		OrderID:         "order-001",
		Points:          []repository.RoutePoint{{Lat: -6.9, Lng: 107.6, Timestamp: time.Now()}},
		TotalDistanceKm: 5.0,
		DurationMinutes: 15,
		StartedAt:       time.Now().Add(-15 * time.Minute),
		EndedAt:         time.Now(),
	}

	mockRepo.EXPECT().SaveTripRoute(gomock.Any(), route).Return(nil)

	err := mockRepo.SaveTripRoute(context.Background(), route)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSaveTripRoute_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockTrackingRepositoryInterface(ctrl)

	route := &repository.TripRoute{
		OrderID: "order-err",
	}

	mockRepo.EXPECT().SaveTripRoute(gomock.Any(), route).Return(context.DeadlineExceeded)

	err := mockRepo.SaveTripRoute(context.Background(), route)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestSaveTripRoute_MultiplePoints(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockTrackingRepositoryInterface(ctrl)

	now := time.Now()
	route := &repository.TripRoute{
		OrderID: "order-multi",
		Points: []repository.RoutePoint{
			{Lat: -6.90, Lng: 107.60, Timestamp: now},
			{Lat: -6.91, Lng: 107.61, Timestamp: now.Add(2 * time.Minute)},
			{Lat: -6.92, Lng: 107.62, Timestamp: now.Add(5 * time.Minute)},
		},
		TotalDistanceKm: 2.5,
		DurationMinutes: 5,
		StartedAt:       now,
		EndedAt:         now.Add(5 * time.Minute),
	}

	mockRepo.EXPECT().SaveTripRoute(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, r *repository.TripRoute) error {
			if r.OrderID != "order-multi" {
				t.Errorf("expected order-multi, got %s", r.OrderID)
			}
			if len(r.Points) != 3 {
				t.Errorf("expected 3 points, got %d", len(r.Points))
			}
			if r.DurationMinutes != 5 {
				t.Errorf("expected 5 mins, got %d", r.DurationMinutes)
			}
			return nil
		},
	)

	err := mockRepo.SaveTripRoute(context.Background(), route)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── GetTripRoute ─────────────────────────────────────────────────────────────

func TestGetTripRoute_Found(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockTrackingRepositoryInterface(ctrl)

	expectedRoute := &repository.TripRoute{
		OrderID:         "order-001",
		Points:          []repository.RoutePoint{{Lat: -6.9, Lng: 107.6, Timestamp: time.Now()}},
		TotalDistanceKm: 5.0,
		DurationMinutes: 15,
	}

	mockRepo.EXPECT().GetTripRoute(gomock.Any(), "order-001").Return(expectedRoute, nil)

	route, err := mockRepo.GetTripRoute(context.Background(), "order-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route == nil {
		t.Fatal("expected route, got nil")
	}
	if route.OrderID != "order-001" {
		t.Errorf("expected order-001, got %s", route.OrderID)
	}
	if route.TotalDistanceKm != 5.0 {
		t.Errorf("expected 5.0 km, got %f", route.TotalDistanceKm)
	}
	if len(route.Points) != 1 {
		t.Errorf("expected 1 point, got %d", len(route.Points))
	}
}

func TestGetTripRoute_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockTrackingRepositoryInterface(ctrl)

	// Repository returns nil, nil when document not found
	mockRepo.EXPECT().GetTripRoute(gomock.Any(), "nonexistent").Return(nil, nil)

	route, err := mockRepo.GetTripRoute(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("expected nil error for not found, got %v", err)
	}
	if route != nil {
		t.Errorf("expected nil route, got %+v", route)
	}
}

func TestGetTripRoute_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockTrackingRepositoryInterface(ctrl)

	mockRepo.EXPECT().GetTripRoute(gomock.Any(), "order-err").Return(nil, context.DeadlineExceeded)

	route, err := mockRepo.GetTripRoute(context.Background(), "order-err")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if route != nil {
		t.Errorf("expected nil route on error, got %+v", route)
	}
}
