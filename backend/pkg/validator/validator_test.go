//go:build unit

package validator

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON_Valid(t *testing.T) {
	body := strings.NewReader(`{"name":"test","age":25}`)
	req := httptest.NewRequest("POST", "/test", body)

	var target struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	if err := DecodeJSON(req, &target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", target.Name)
	}
	if target.Age != 25 {
		t.Errorf("expected age 25, got %d", target.Age)
	}
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`{invalid}`)
	req := httptest.NewRequest("POST", "/test", body)

	var target struct{}
	if err := DecodeJSON(req, &target); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeJSON_NilBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", nil)
	req.Body = nil

	var target struct{}
	if err := DecodeJSON(req, &target); err == nil {
		t.Error("expected error for nil body")
	}
}

func TestQueryInt_Default(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	val := QueryInt(req, "page", 1)
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}
}

func TestQueryInt_Provided(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?page=5", nil)
	val := QueryInt(req, "page", 1)
	if val != 5 {
		t.Errorf("expected 5, got %d", val)
	}
}

func TestQueryInt_Invalid(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?page=abc", nil)
	val := QueryInt(req, "page", 1)
	if val != 1 {
		t.Errorf("expected default 1, got %d", val)
	}
}

func TestQueryFloat64(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?lat=-6.175", nil)
	val := QueryFloat64(req, "lat", 0)
	if val != -6.175 {
		t.Errorf("expected -6.175, got %f", val)
	}
}

func TestQueryBool(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?active=true", nil)
	val := QueryBool(req, "active", false)
	if !val {
		t.Error("expected true")
	}
}

func TestParsePagination_Defaults(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	p := ParsePagination(req, 50)

	if p.Page != 1 {
		t.Errorf("expected page 1, got %d", p.Page)
	}
	if p.Limit != 20 {
		t.Errorf("expected limit 20, got %d", p.Limit)
	}
	if p.Offset != 0 {
		t.Errorf("expected offset 0, got %d", p.Offset)
	}
}

func TestParsePagination_MaxLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?limit=999", nil)
	p := ParsePagination(req, 50)

	if p.Limit != 50 {
		t.Errorf("expected limit capped at 50, got %d", p.Limit)
	}
}

func TestParsePagination_Offset(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?page=3&limit=10", nil)
	p := ParsePagination(req, 50)

	if p.Offset != 20 {
		t.Errorf("expected offset 20, got %d", p.Offset)
	}
}

func TestValidationErrors(t *testing.T) {
	var errs ValidationErrors
	errs.Add("name", "is required")
	errs.Add("age", "must be positive")

	if !errs.HasErrors() {
		t.Error("expected HasErrors to be true")
	}
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errs))
	}

	msg := errs.Error()
	if !strings.Contains(msg, "name: is required") {
		t.Errorf("expected error message to contain field name, got: %s", msg)
	}
}
