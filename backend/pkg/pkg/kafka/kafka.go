// Package kafka provides a thin wrapper around confluent-kafka-go (or
// segmentio/kafka-go) for producing and consuming Kafka messages in Clay
// microservices.
//
// Design decisions:
//   - JSON serialization for all events (simple, debuggable)
//   - Event envelope with standard metadata (event_type, timestamp, source)
//   - Sync producer for simplicity; async can be added later
//   - Consumer group abstraction for topic subscription
package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ───── Event Envelope ─────────────────────────────────────────────────────────

// Event is the standard Kafka event envelope used across all Clay services.
// Every Kafka message MUST be wrapped in this envelope.
type Event struct {
	// EventID is a unique ID for this event instance (for deduplication).
	EventID string `json:"event_id"`
	// EventType identifies the event, e.g. "order.created", "payment.held".
	EventType string `json:"event_type"`
	// Source is the service that produced this event, e.g. "clay-ride-order-service".
	Source string `json:"source"`
	// Timestamp is when the event was produced.
	Timestamp time.Time `json:"timestamp"`
	// Data holds the event-specific payload.
	Data json.RawMessage `json:"data"`
}

// NewEvent creates a new Event with a generated ID and current timestamp.
func NewEvent(eventType, source string, data interface{}) (*Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Event{
		EventID:   uuid.New().String(),
		EventType: eventType,
		Source:    source,
		Timestamp: time.Now().UTC(),
		Data:      raw,
	}, nil
}

// ParseData unmarshals the event data into the provided target struct.
func (e *Event) ParseData(target interface{}) error {
	return json.Unmarshal(e.Data, target)
}

// ───── Producer Interface ─────────────────────────────────────────────────────

// Producer publishes events to Kafka topics.
type Producer interface {
	// Publish sends an event to the specified topic.
	Publish(ctx context.Context, topic string, key string, event *Event) error
	// Close gracefully shuts down the producer.
	Close() error
}

// ───── Consumer Interface ─────────────────────────────────────────────────────

// Handler processes a single Kafka event.
// Returning an error will cause the message to be retried (depending on config).
type Handler func(ctx context.Context, event *Event) error

// Consumer subscribes to Kafka topics and processes messages.
type Consumer interface {
	// Subscribe registers a handler for a specific topic.
	Subscribe(topic string, handler Handler)
	// Start begins consuming messages. Blocks until ctx is cancelled.
	Start(ctx context.Context) error
	// Close gracefully shuts down the consumer.
	Close() error
}

// ───── Config ─────────────────────────────────────────────────────────────────

// Config holds Kafka connection configuration.
type Config struct {
	Brokers       []string `json:"brokers" env:"KAFKA_BROKERS"`
	ConsumerGroup string   `json:"consumer_group" env:"KAFKA_CONSUMER_GROUP"`
	// ClientID identifies this service instance to the Kafka cluster.
	ClientID string `json:"client_id" env:"KAFKA_CLIENT_ID"`
}
