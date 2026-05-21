//go:build unit

package geo

import (
	"context"
	"testing"
)

// The NoopClient is the default in-process geo client used when no upstream
// geo service is wired up. It must satisfy the Client interface and never
// return an error.
func TestNoopClient_SatisfiesContract(t *testing.T) {
	var c Client = NewNoopClient()

	if err := c.RegisterDriver(context.Background(), LocationUpdate{DriverID: "d1"}); err != nil {
		t.Errorf("RegisterDriver should succeed silently, got %v", err)
	}
	if err := c.UpdateLocation(context.Background(), LocationUpdate{DriverID: "d1"}); err != nil {
		t.Errorf("UpdateLocation should succeed silently, got %v", err)
	}
	if err := c.UnregisterDriver(context.Background(), "d1"); err != nil {
		t.Errorf("UnregisterDriver should succeed silently, got %v", err)
	}
}
