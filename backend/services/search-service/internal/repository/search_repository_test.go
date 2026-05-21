//go:build unit

package repository

import (
	"context"
	"testing"
)

func TestSearchRepository_Ping(t *testing.T) {
	repo := NewSearchRepository(nil)

	err := repo.Ping(context.Background())
	if err == nil {
		t.Errorf("expected error from Ping with nil client, got nil")
	}
}
