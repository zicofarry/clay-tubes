// Package broker provides a Kafka producer for the Payment Service.
//
// Implements the kafka.Producer interface from clay-shared.
//
// Produced events:
//   - payment.charged   → consumed by order service to proceed
//   - payment.failed    → consumed by order service to cancel
//   - payment.refunded  → consumed by order service / notification
//   - payment.held      → consumed by order service (hold confirmed)
//   - payment.captured  → consumed by order service (capture confirmed)
//   - payment.released  → consumed by order service (hold voided)
//   - settlement.created → consumed by Wallet Service (credit driver)
package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/zicofarry/clay-app/backend/pkg/pkg/kafka"
)

// ── Kafka Topics ─────────────────────────────────────────────────────────────

const (
	TopicPaymentCharged  = "payment.charged"
	TopicPaymentFailed   = "payment.failed"
	TopicPaymentRefunded = "payment.refunded"
	TopicPaymentHeld     = "payment.held"
	TopicPaymentCaptured = "payment.captured"
	TopicPaymentReleased = "payment.released"
	TopicSettlementCreated = "settlement.created"
)

// ServiceName is the source identifier in Kafka event envelopes.
const ServiceName = "clay-payment-service"

// ── Event Payloads ───────────────────────────────────────────────────────────

// ChargeEvent is the payload for payment.charged and payment.failed events.
type ChargeEvent struct {
	TransactionID string `json:"transaction_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	Amount        int    `json:"amount"`
	PaymentMethod string `json:"payment_method"`
	Status        string `json:"status"`
}

// RefundEvent is the payload for payment.refunded events.
type RefundEvent struct {
	RefundID      string `json:"refund_id"`
	TransactionID string `json:"transaction_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	Amount        int    `json:"amount"`
	Reason        string `json:"reason"`
	Status        string `json:"status"`
}

// HoldEvent is the payload for payment.held, payment.captured, payment.released events.
type HoldEvent struct {
	HoldID        string `json:"hold_id"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	Amount        int    `json:"amount"`
	Status        string `json:"status"`
	TransactionID string `json:"transaction_id,omitempty"` // only for captured
}

// SettlementEvent is the payload for settlement.created events.
type SettlementEvent struct {
	SettlementID string `json:"settlement_id"`
	OrderID      string `json:"order_id"`
	DriverID     string `json:"driver_id"`
	GrossFare    int    `json:"gross_fare"`
	PlatformFee  int    `json:"platform_fee"`
	DriverPayout int    `json:"driver_payout"`
	ServiceType  string `json:"service_type"`
}

// ── PaymentProducer ──────────────────────────────────────────────────────────

// PaymentProducer wraps the kafka.Producer interface from clay-shared
// with convenience methods for publishing payment-specific events.
type PaymentProducer struct {
	producer kafka.Producer
	logger   *slog.Logger
}

// NewPaymentProducer creates a new PaymentProducer.
// Pass a real kafka.Producer implementation in production, or
// use NewLogProducer for local development.
func NewPaymentProducer(producer kafka.Producer, logger *slog.Logger) *PaymentProducer {
	return &PaymentProducer{producer: producer, logger: logger}
}

// PublishChargeEvent publishes a payment.charged or payment.failed event.
func (p *PaymentProducer) PublishChargeEvent(ctx context.Context, event *ChargeEvent) error {
	topic := TopicPaymentCharged
	if event.Status == "failed" {
		topic = TopicPaymentFailed
	}
	return p.publish(ctx, topic, event.OrderID, event)
}

// PublishRefundEvent publishes a payment.refunded event.
func (p *PaymentProducer) PublishRefundEvent(ctx context.Context, event *RefundEvent) error {
	return p.publish(ctx, TopicPaymentRefunded, event.OrderID, event)
}

// PublishHoldEvent publishes a payment.held event.
func (p *PaymentProducer) PublishHoldEvent(ctx context.Context, event *HoldEvent) error {
	return p.publish(ctx, TopicPaymentHeld, event.OrderID, event)
}

// PublishCaptureEvent publishes a payment.captured event.
func (p *PaymentProducer) PublishCaptureEvent(ctx context.Context, event *HoldEvent) error {
	return p.publish(ctx, TopicPaymentCaptured, event.OrderID, event)
}

// PublishReleaseEvent publishes a payment.released event.
func (p *PaymentProducer) PublishReleaseEvent(ctx context.Context, event *HoldEvent) error {
	return p.publish(ctx, TopicPaymentReleased, event.OrderID, event)
}

// PublishSettlementEvent publishes a settlement.created event.
func (p *PaymentProducer) PublishSettlementEvent(ctx context.Context, event *SettlementEvent) error {
	return p.publish(ctx, TopicSettlementCreated, event.OrderID, event)
}

// Close gracefully shuts down the underlying Kafka producer.
func (p *PaymentProducer) Close() error {
	if p.producer != nil {
		return p.producer.Close()
	}
	return nil
}

func (p *PaymentProducer) publish(ctx context.Context, topic, key string, data interface{}) error {
	event, err := kafka.NewEvent(topic, ServiceName, data)
	if err != nil {
		return fmt.Errorf("create kafka event: %w", err)
	}

	if err := p.producer.Publish(ctx, topic, key, event); err != nil {
		p.logger.Error("failed to publish kafka event",
			slog.String("topic", topic),
			slog.String("key", key),
			slog.Any("error", err),
		)
		return fmt.Errorf("publish to %s: %w", topic, err)
	}

	p.logger.Info("kafka event published",
		slog.String("topic", topic),
		slog.String("event_id", event.EventID),
		slog.String("key", key),
	)
	return nil
}

// ── LogProducer (local dev / testing) ────────────────────────────────────────

// LogProducer implements kafka.Producer by logging events instead of sending
// them to a real Kafka cluster. Use this for local development and testing.
type LogProducer struct {
	logger *slog.Logger
}

// NewLogProducer creates a producer that logs events instead of sending to Kafka.
func NewLogProducer(logger *slog.Logger) *LogProducer {
	return &LogProducer{logger: logger}
}

// Publish logs the event details.
func (p *LogProducer) Publish(_ context.Context, topic string, key string, event *kafka.Event) error {
	data, _ := json.Marshal(event)
	p.logger.Info("[KAFKA-LOG] event published",
		slog.String("topic", topic),
		slog.String("key", key),
		slog.String("event_id", event.EventID),
		slog.String("event_type", event.EventType),
		slog.String("payload", string(data)),
	)
	return nil
}

// Close is a no-op for the log producer.
func (p *LogProducer) Close() error {
	return nil
}
