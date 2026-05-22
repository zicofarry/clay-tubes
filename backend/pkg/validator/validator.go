// Package validator provides request body validation helpers for Clay
// microservices. Wraps the go-playground/validator library with
// Clay-specific error formatting.
package validator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ───── Request Body Parsing ──────────────────────────────────────────────────

// DecodeJSON reads the request body as JSON into the target struct.
// Returns a user-friendly error message if decoding fails.
func DecodeJSON(r *http.Request, target interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// ───── Query Parameter Helpers ───────────────────────────────────────────────

// QueryInt reads an integer query parameter with a default value.
//
// Usage:
//
//	page := validator.QueryInt(r, "page", 1)
//	limit := validator.QueryInt(r, "limit", 20)
func QueryInt(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return val
}

// QueryString reads a string query parameter with a default value.
func QueryString(r *http.Request, key, defaultVal string) string {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	return raw
}

// QueryFloat64 reads a float64 query parameter with a default value.
func QueryFloat64(r *http.Request, key string, defaultVal float64) float64 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return defaultVal
	}
	return val
}

// QueryBool reads a boolean query parameter with a default value.
func QueryBool(r *http.Request, key string, defaultVal bool) bool {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(raw)
	if err != nil {
		return defaultVal
	}
	return val
}

// ───── Pagination ────────────────────────────────────────────────────────────

// Pagination holds validated page/limit values.
type Pagination struct {
	Page   int
	Limit  int
	Offset int
}

// ParsePagination extracts and validates page/limit from query parameters.
// Enforces maxLimit to prevent abuse.
func ParsePagination(r *http.Request, maxLimit int) Pagination {
	page := QueryInt(r, "page", 1)
	limit := QueryInt(r, "limit", 20)

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	return Pagination{
		Page:   page,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
}

// ───── Validation Errors ─────────────────────────────────────────────────────

// ValidationError represents a field-level validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors is a collection of field-level errors.
type ValidationErrors []ValidationError

// Error implements the error interface.
func (ve ValidationErrors) Error() string {
	var msgs []string
	for _, e := range ve {
		msgs = append(msgs, fmt.Sprintf("%s: %s", e.Field, e.Message))
	}
	return strings.Join(msgs, "; ")
}

// Add appends a validation error.
func (ve *ValidationErrors) Add(field, message string) {
	*ve = append(*ve, ValidationError{Field: field, Message: message})
}

// HasErrors returns true if there are any validation errors.
func (ve ValidationErrors) HasErrors() bool {
	return len(ve) > 0
}
