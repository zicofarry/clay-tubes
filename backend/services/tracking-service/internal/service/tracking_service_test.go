//go:build unit

package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/cache"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/mocks/cachemock"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func setupService(t *testing.T) (*TrackingService, *repomock.MockTrackingRepositoryInterface, *cachemock.MockTrackingCacheInterface) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockTrackingRepositoryInterface(ctrl)
	mockCache := cachemock.NewMockTrackingCacheInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewTrackingService(mockRepo, mockCache, logger)
	return svc, mockRepo, mockCache
}

// ── GetOrderPosition ─────────────────────────────────────────────────────────

func TestGetOrderPosition_Success(t *testing.T) {
	svc, _, mockCache := setupService(t)
	mockCache.EXPECT().GetOrderPosition(gomock.Any(), "order-1").Return(&cache.OrderPosition{
		OrderID: "order-1", Lat: -6.9, Lng: 107.6,
	}, nil)

	pos, err := svc.GetOrderPosition(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos.OrderID != "order-1" {
		t.Errorf("expected order-1, got %s", pos.OrderID)
	}
}

func TestGetOrderPosition_NotFound(t *testing.T) {
	svc, _, mockCache := setupService(t)
	mockCache.EXPECT().GetOrderPosition(gomock.Any(), "order-missing").Return(nil, nil)

	_, err := svc.GetOrderPosition(context.Background(), "order-missing")
	if err != ErrOrderNotFound {
		t.Errorf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestGetOrderPosition_CacheError(t *testing.T) {
	svc, _, mockCache := setupService(t)
	mockCache.EXPECT().GetOrderPosition(gomock.Any(), "order-err").Return(nil, errors.New("redis connection refused"))

	_, err := svc.GetOrderPosition(context.Background(), "order-err")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if err == ErrOrderNotFound {
		t.Error("expected a cache error, not ErrOrderNotFound")
	}
}

// ── GetOrderETA ──────────────────────────────────────────────────────────────

func TestGetOrderETA_Success(t *testing.T) {
	svc, _, mockCache := setupService(t)
	mockCache.EXPECT().GetOrderPosition(gomock.Any(), "order-1").Return(&cache.OrderPosition{
		OrderID: "order-1", Lat: -6.9, Lng: 107.6,
	}, nil)

	eta, err := svc.GetOrderETA(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eta.OrderID != "order-1" {
		t.Errorf("expected order-1, got %s", eta.OrderID)
	}
	if eta.ETAMinutes <= 0 {
		t.Errorf("expected positive ETA, got %d", eta.ETAMinutes)
	}
}

func TestGetOrderETA_OrderNotFound(t *testing.T) {
	svc, _, mockCache := setupService(t)
	mockCache.EXPECT().GetOrderPosition(gomock.Any(), "order-missing").Return(nil, nil)

	_, err := svc.GetOrderETA(context.Background(), "order-missing")
	if err != ErrOrderNotFound {
		t.Errorf("expected ErrOrderNotFound, got %v", err)
	}
}

// ── GetOrderRoute ────────────────────────────────────────────────────────────

func TestGetOrderRoute_Success(t *testing.T) {
	svc, _, mockCache := setupService(t)
	mockCache.EXPECT().GetOrderPosition(gomock.Any(), "order-1").Return(&cache.OrderPosition{
		OrderID: "order-1", Lat: -6.9, Lng: 107.6,
	}, nil)

	route, err := svc.GetOrderRoute(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.OrderID != "order-1" {
		t.Errorf("expected order-1, got %s", route.OrderID)
	}
	if len(route.Steps) == 0 {
		t.Error("expected at least one route step")
	}
}

func TestGetOrderRoute_OrderNotFound(t *testing.T) {
	svc, _, mockCache := setupService(t)
	mockCache.EXPECT().GetOrderPosition(gomock.Any(), "order-missing").Return(nil, nil)

	_, err := svc.GetOrderRoute(context.Background(), "order-missing")
	if err != ErrOrderNotFound {
		t.Errorf("expected ErrOrderNotFound, got %v", err)
	}
}

// ── GetTripRoute ─────────────────────────────────────────────────────────────

func TestGetTripRoute_Success(t *testing.T) {
	svc, mockRepo, _ := setupService(t)
	mockRepo.EXPECT().GetTripRoute(gomock.Any(), "order-1").Return(&repository.TripRoute{
		OrderID:         "order-1",
		TotalDistanceKm: 5.0,
		DurationMinutes: 15,
	}, nil)

	route, err := svc.GetTripRoute(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.OrderID != "order-1" {
		t.Errorf("expected order-1, got %s", route.OrderID)
	}
}

func TestGetTripRoute_NotFound(t *testing.T) {
	svc, mockRepo, _ := setupService(t)
	mockRepo.EXPECT().GetTripRoute(gomock.Any(), "order-missing").Return(nil, nil)

	_, err := svc.GetTripRoute(context.Background(), "order-missing")
	if err != ErrRouteNotFound {
		t.Errorf("expected ErrRouteNotFound, got %v", err)
	}
}

func TestGetTripRoute_DBError(t *testing.T) {
	svc, mockRepo, _ := setupService(t)
	mockRepo.EXPECT().GetTripRoute(gomock.Any(), "order-err").Return(nil, errors.New("mongo connection failed"))

	_, err := svc.GetTripRoute(context.Background(), "order-err")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ── StartTracking ────────────────────────────────────────────────────────────

func TestStartTracking_Success(t *testing.T) {
	svc, _, mockCache := setupService(t)

	mockCache.EXPECT().SetOrderPosition(gomock.Any(), gomock.Any()).Return(nil)
	mockCache.EXPECT().AppendRoutePoint(gomock.Any(), "order-1", -6.9, 107.6, gomock.Any()).Return(nil)

	err := svc.StartTracking(context.Background(), &StartTrackingRequest{
		OrderID: "order-1", DriverID: "driver-1", PickupLat: -6.9, PickupLng: 107.6,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStartTracking_CacheError(t *testing.T) {
	svc, _, mockCache := setupService(t)

	mockCache.EXPECT().SetOrderPosition(gomock.Any(), gomock.Any()).Return(errors.New("redis unavailable"))

	err := svc.StartTracking(context.Background(), &StartTrackingRequest{
		OrderID: "order-1", DriverID: "driver-1", PickupLat: -6.9, PickupLng: 107.6,
	})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ── PushLocationUpdate ───────────────────────────────────────────────────────

func TestPushLocationUpdate_Success(t *testing.T) {
	svc, _, mockCache := setupService(t)

	mockCache.EXPECT().GetOrderPosition(gomock.Any(), "order-1").Return(&cache.OrderPosition{
		OrderID: "order-1",
	}, nil)
	mockCache.EXPECT().SetOrderPosition(gomock.Any(), gomock.Any()).Return(nil)
	mockCache.EXPECT().AppendRoutePoint(gomock.Any(), "order-1", -6.91, 107.61, gomock.Any()).Return(nil)

	ts := time.Now()
	err := svc.PushLocationUpdate(context.Background(), "order-1", &LocationUpdateEvent{
		DriverID: "driver-1", Lat: -6.91, Lng: 107.61, Timestamp: ts,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPushLocationUpdate_CacheGetError(t *testing.T) {
	svc, _, mockCache := setupService(t)

	mockCache.EXPECT().GetOrderPosition(gomock.Any(), "order-err").Return(nil, errors.New("redis timeout"))

	err := svc.PushLocationUpdate(context.Background(), "order-err", &LocationUpdateEvent{
		DriverID: "driver-1", Lat: -6.91, Lng: 107.61, Timestamp: time.Now(),
	})
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ── StopTracking ─────────────────────────────────────────────────────────────

func TestStopTracking_Success(t *testing.T) {
	svc, mockRepo, mockCache := setupService(t)

	ts := time.Now()
	pts := []cache.RoutePointData{
		{Lat: -6.9, Lng: 107.6, Timestamp: ts},
		{Lat: -6.91, Lng: 107.61, Timestamp: ts.Add(5 * time.Minute)},
	}

	mockCache.EXPECT().GetActiveRoutePoints(gomock.Any(), "order-1").Return(pts, nil)
	mockRepo.EXPECT().SaveTripRoute(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, route *repository.TripRoute) error {
		if route.OrderID != "order-1" {
			t.Errorf("expected order-1")
		}
		if route.DurationMinutes != 5 {
			t.Errorf("expected 5 mins duration")
		}
		if len(route.Points) != 2 {
			t.Errorf("expected 2 points")
		}
		return nil
	})
	mockCache.EXPECT().DeleteOrderPosition(gomock.Any(), "order-1").Return(nil)
	mockCache.EXPECT().ClearActiveRoutePoints(gomock.Any(), "order-1").Return(nil)

	err := svc.StopTracking(context.Background(), "order-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStopTracking_NoPoints(t *testing.T) {
	svc, _, mockCache := setupService(t)

	mockCache.EXPECT().GetActiveRoutePoints(gomock.Any(), "order-empty").Return([]cache.RoutePointData{}, nil)
	mockCache.EXPECT().DeleteOrderPosition(gomock.Any(), "order-empty").Return(nil)
	mockCache.EXPECT().ClearActiveRoutePoints(gomock.Any(), "order-empty").Return(nil)

	err := svc.StopTracking(context.Background(), "order-empty")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStopTracking_DBSaveError(t *testing.T) {
	svc, mockRepo, mockCache := setupService(t)

	ts := time.Now()
	pts := []cache.RoutePointData{
		{Lat: -6.9, Lng: 107.6, Timestamp: ts},
		{Lat: -6.91, Lng: 107.61, Timestamp: ts.Add(5 * time.Minute)},
	}

	mockCache.EXPECT().GetActiveRoutePoints(gomock.Any(), "order-1").Return(pts, nil)
	mockRepo.EXPECT().SaveTripRoute(gomock.Any(), gomock.Any()).Return(errors.New("mongo write error"))
	mockCache.EXPECT().DeleteOrderPosition(gomock.Any(), "order-1").Return(nil)
	mockCache.EXPECT().ClearActiveRoutePoints(gomock.Any(), "order-1").Return(nil)

	// StopTracking should NOT fail even if DB save fails (as per code logic)
	err := svc.StopTracking(context.Background(), "order-1")
	if err != nil {
		t.Errorf("expected no error (DB failure is non-blocking), got %v", err)
	}
}
