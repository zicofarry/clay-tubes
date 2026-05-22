//go:build unit

package kafka

import (
	"encoding/json"
	"testing"
)

func TestNewEvent(t *testing.T) {
	data := map[string]string{"order_id": "abc-123"}

	event, err := NewEvent("order.created", "clay-ride-order-service", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.EventID == "" {
		t.Error("expected generated event ID")
	}
	if event.EventType != "order.created" {
		t.Errorf("expected order.created, got %s", event.EventType)
	}
	if event.Source != "clay-ride-order-service" {
		t.Errorf("expected clay-ride-order-service, got %s", event.Source)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestEvent_ParseData(t *testing.T) {
	payload := map[string]string{"order_id": "xyz-789"}
	event, _ := NewEvent("order.created", "test", payload)

	var result map[string]string
	if err := event.ParseData(&result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["order_id"] != "xyz-789" {
		t.Errorf("expected xyz-789, got %s", result["order_id"])
	}
}

func TestEvent_JSONRoundTrip(t *testing.T) {
	data := map[string]interface{}{
		"amount":   50000,
		"currency": "IDR",
	}
	event, _ := NewEvent("payment.held", "clay-payment-service", data)

	// Marshal
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Unmarshal
	var decoded Event
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.EventType != "payment.held" {
		t.Errorf("expected payment.held, got %s", decoded.EventType)
	}
	if decoded.EventID != event.EventID {
		t.Errorf("event ID mismatch: %s vs %s", decoded.EventID, event.EventID)
	}

	// Verify data integrity
	var result map[string]interface{}
	if err := decoded.ParseData(&result); err != nil {
		t.Fatalf("parse data failed: %v", err)
	}
	if result["currency"] != "IDR" {
		t.Errorf("expected IDR, got %v", result["currency"])
	}
}
