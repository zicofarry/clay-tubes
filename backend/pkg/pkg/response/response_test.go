//go:build unit

package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"name": "test"}
	Success(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected application/json content type, got %s", ct)
	}

	var resp SuccessResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()

	Error(w, http.StatusBadRequest, "INVALID_INPUT", "name is required")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp ErrorResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "INVALID_INPUT" {
		t.Errorf("expected code INVALID_INPUT, got %s", resp.Code)
	}
	if resp.Message != "name is required" {
		t.Errorf("expected message 'name is required', got %s", resp.Message)
	}
}

func TestPaginated(t *testing.T) {
	w := httptest.NewRecorder()

	items := []string{"a", "b", "c"}
	Paginated(w, http.StatusOK, items, 100, 1, 20)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp PaginatedResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Meta.Total != 100 {
		t.Errorf("expected total 100, got %d", resp.Meta.Total)
	}
	if resp.Meta.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Meta.Page)
	}
	if resp.Meta.Limit != 20 {
		t.Errorf("expected limit 20, got %d", resp.Meta.Limit)
	}
}

func TestHealth(t *testing.T) {
	w := httptest.NewRecorder()

	Health(w, "1.0.0")

	var resp HealthResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %s", resp.Status)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", resp.Version)
	}
}
