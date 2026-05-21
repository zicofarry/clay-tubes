// Package broker provides Kafka consumer handling for the Geo Service.
// Consumes: driver.location.updated, order.driver_assigned, order.completed, order.cancelled.
package broker

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/cache"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/kafka"
)

const (
	TopicDriverLocationUpdated = "driver.location.updated"
	TopicOrderDriverAssigned   = "order.driver_assigned"
	TopicOrderCompleted        = "order.completed"
	TopicOrderCancelled        = "order.cancelled"
)

// ── Event Payloads ───────────────────────────────────────────────────────────

type DriverLocationEvent struct {
	DriverID    string  `json:"driver_id"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	ServiceType string  `json:"service_type"`
	Bearing     float64 `json:"bearing"`
	SpeedKmh    float64 `json:"speed_kmh"`
	Status      string  `json:"status"` // online, offline
}

type OrderDriverAssignedEvent struct {
	OrderID  string  `json:"order_id"`
	DriverID string  `json:"driver_id"`
	DestLat  float64 `json:"dest_lat"`
	DestLng  float64 `json:"dest_lng"`
}

type OrderCompletedEvent struct {
	OrderID  string `json:"order_id"`
	DriverID string `json:"driver_id"`
}

// ── GeoConsumer ──────────────────────────────────────────────────────────────

// GeoConsumer processes incoming Kafka events for the Geo Service.
type GeoConsumer struct {
	geoCache cache.GeoCacheInterface
	logger   *slog.Logger
}

// NewGeoConsumer creates a new GeoConsumer.
func NewGeoConsumer(geoCache cache.GeoCacheInterface, logger *slog.Logger) *GeoConsumer {
	return &GeoConsumer{geoCache: geoCache, logger: logger}
}

// HandleDriverLocationUpdated processes driver.location.updated events.
func (c *GeoConsumer) HandleDriverLocationUpdated(ctx context.Context, event *kafka.Event) error {
	var payload DriverLocationEvent
	if err := event.ParseData(&payload); err != nil {
		c.logger.Error("failed to parse driver location event", slog.Any("error", err))
		return err
	}

	if payload.Status == "offline" {
		c.geoCache.RemoveDriver(ctx, payload.ServiceType, payload.DriverID)
		c.logger.Info("driver removed (offline)", slog.String("driver_id", payload.DriverID))
		return nil
	}

	loc := &cache.DriverLocation{
		DriverID: payload.DriverID, Lat: payload.Lat, Lng: payload.Lng,
		Bearing: payload.Bearing, SpeedKmh: payload.SpeedKmh,
	}
	if err := c.geoCache.UpdateDriverLocation(ctx, payload.ServiceType, loc); err != nil {
		c.logger.Error("failed to update driver location from event", slog.Any("error", err))
		return err
	}
	return nil
}

// HandleOrderDriverAssigned initializes ETA tracking for a new assignment.
func (c *GeoConsumer) HandleOrderDriverAssigned(ctx context.Context, event *kafka.Event) error {
	var payload OrderDriverAssignedEvent
	if err := event.ParseData(&payload); err != nil {
		return err
	}

	eta := &cache.ETAData{
		DriverID: payload.DriverID, OrderID: payload.OrderID,
		ETASeconds: 0, ETAText: "Menghitung...",
		DestinationType: "pickup",
	}
	c.geoCache.SetETA(ctx, eta)
	c.logger.Info("ETA tracking initialized", slog.String("driver_id", payload.DriverID), slog.String("order_id", payload.OrderID))
	return nil
}

// HandleOrderCompleted cleans up ETA cache when order is completed or cancelled.
func (c *GeoConsumer) HandleOrderCompleted(ctx context.Context, event *kafka.Event) error {
	var payload OrderCompletedEvent
	if err := event.ParseData(&payload); err != nil {
		return err
	}
	c.geoCache.DeleteETA(ctx, payload.DriverID, payload.OrderID)
	c.logger.Info("ETA tracking cleaned up", slog.String("driver_id", payload.DriverID), slog.String("order_id", payload.OrderID))
	return nil
}

// RegisterHandlers registers all Kafka topic handlers with a consumer.
func (c *GeoConsumer) RegisterHandlers(consumer kafka.Consumer) {
	consumer.Subscribe(TopicDriverLocationUpdated, c.HandleDriverLocationUpdated)
	consumer.Subscribe(TopicOrderDriverAssigned, c.HandleOrderDriverAssigned)
	consumer.Subscribe(TopicOrderCompleted, c.HandleOrderCompleted)
	consumer.Subscribe(TopicOrderCancelled, c.HandleOrderCompleted) // same cleanup logic
}

// ── LogConsumer (local dev) ──────────────────────────────────────────────────

// LogConsumer implements kafka.Consumer by logging instead of consuming.
type LogConsumer struct {
	logger   *slog.Logger
	handlers map[string]kafka.Handler
}

func NewLogConsumer(logger *slog.Logger) *LogConsumer {
	return &LogConsumer{logger: logger, handlers: make(map[string]kafka.Handler)}
}

func (c *LogConsumer) Subscribe(topic string, handler kafka.Handler) {
	c.handlers[topic] = handler
	c.logger.Info("[KAFKA-LOG] subscribed", slog.String("topic", topic))
}

func (c *LogConsumer) Start(_ context.Context) error {
	c.logger.Info("[KAFKA-LOG] consumer started (log-only mode)")
	return nil
}

func (c *LogConsumer) Close() error { return nil }

// SimulateEvent simulates receiving a Kafka event (for testing).
func (c *LogConsumer) SimulateEvent(ctx context.Context, topic string, data interface{}) error {
	handler, ok := c.handlers[topic]
	if !ok {
		return nil
	}
	event, err := kafka.NewEvent(topic, "test", data)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(event)
	c.logger.Info("[KAFKA-LOG] simulated event", slog.String("topic", topic), slog.String("payload", string(raw)))
	return handler(ctx, event)
}
