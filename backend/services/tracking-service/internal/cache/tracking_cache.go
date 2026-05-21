package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// OrderPosition maps to the active driver position
type OrderPosition struct {
	OrderID    string    `json:"order_id"`
	DriverID   string    `json:"driver_id"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	Bearing    float64   `json:"bearing"`
	SpeedKmh   float64   `json:"speed_kmh"`
	ETAMinutes int       `json:"eta_minutes"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TrackingCacheInterface defines volatile operations for real-time tracking
type TrackingCacheInterface interface {
	SetOrderPosition(ctx context.Context, pos *OrderPosition) error
	GetOrderPosition(ctx context.Context, orderID string) (*OrderPosition, error)
	DeleteOrderPosition(ctx context.Context, orderID string) error
	
	// Methods to accumulate route points in memory while trip is active
	AppendRoutePoint(ctx context.Context, orderID string, lat, lng float64, ts time.Time) error
	GetActiveRoutePoints(ctx context.Context, orderID string) ([]RoutePointData, error)
	ClearActiveRoutePoints(ctx context.Context, orderID string) error
}

// RoutePointData represents minimal coordinate data for cache
type RoutePointData struct {
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Timestamp time.Time `json:"timestamp"`
}

type TrackingCache struct {
	rdb *redis.Client
}

// NewTrackingCache creates a new Redis tracking cache
func NewTrackingCache(rdb *redis.Client) *TrackingCache {
	return &TrackingCache{rdb: rdb}
}

func posKey(orderID string) string {
	return fmt.Sprintf("tracking:order:%s:pos", orderID)
}

func ptsKey(orderID string) string {
	return fmt.Sprintf("tracking:order:%s:pts", orderID)
}

func (c *TrackingCache) SetOrderPosition(ctx context.Context, pos *OrderPosition) error {
	data, err := json.Marshal(pos)
	if err != nil {
		return err
	}
	// Active order position expires after 2 hours of inactivity
	return c.rdb.Set(ctx, posKey(pos.OrderID), data, 2*time.Hour).Err()
}

func (c *TrackingCache) GetOrderPosition(ctx context.Context, orderID string) (*OrderPosition, error) {
	data, err := c.rdb.Get(ctx, posKey(orderID)).Result()
	if err == redis.Nil {
		return nil, nil // Not found
	} else if err != nil {
		return nil, err
	}

	var pos OrderPosition
	if err := json.Unmarshal([]byte(data), &pos); err != nil {
		return nil, err
	}
	return &pos, nil
}

func (c *TrackingCache) DeleteOrderPosition(ctx context.Context, orderID string) error {
	return c.rdb.Del(ctx, posKey(orderID)).Err()
}

func (c *TrackingCache) AppendRoutePoint(ctx context.Context, orderID string, lat, lng float64, ts time.Time) error {
	pt := RoutePointData{Lat: lat, Lng: lng, Timestamp: ts}
	data, err := json.Marshal(pt)
	if err != nil {
		return err
	}
	key := ptsKey(orderID)
	// RPUSH adds point to the end of the list
	pipe := c.rdb.Pipeline()
	pipe.RPush(ctx, key, data)
	pipe.Expire(ctx, key, 12*time.Hour) // Points expire after 12 hours max if not archived
	_, err = pipe.Exec(ctx)
	return err
}

func (c *TrackingCache) GetActiveRoutePoints(ctx context.Context, orderID string) ([]RoutePointData, error) {
	key := ptsKey(orderID)
	strs, err := c.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	points := make([]RoutePointData, 0, len(strs))
	for _, s := range strs {
		var pt RoutePointData
		if err := json.Unmarshal([]byte(s), &pt); err == nil {
			points = append(points, pt)
		}
	}
	return points, nil
}

func (c *TrackingCache) ClearActiveRoutePoints(ctx context.Context, orderID string) error {
	return c.rdb.Del(ctx, ptsKey(orderID)).Err()
}
