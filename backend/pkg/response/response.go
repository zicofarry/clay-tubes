// Package response provides standardized API response structures
// for all Clay microservices.
//
// Usage:
//
//	response.Success(w, http.StatusOK, data)
//	response.Error(w, http.StatusBadRequest, "INVALID_INPUT", "name is required")
//	response.Paginated(w, http.StatusOK, items, total, page, limit)
package response

import (
	"encoding/json"
	"net/http"
)

// SuccessResp is the standard success response envelope.
type SuccessResp struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResp is the standard error response envelope.
type ErrorResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PaginatedResp wraps a list of items with pagination metadata.
type PaginatedResp struct {
	Data  interface{}    `json:"data"`
	Meta  PaginationMeta `json:"meta"`
}

// PaginationMeta holds pagination info returned to the client.
type PaginationMeta struct {
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// HealthResp is the standard health check response.
type HealthResp struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

// Success writes a successful JSON response.
func Success(w http.ResponseWriter, statusCode int, data interface{}) {
	writeJSON(w, statusCode, SuccessResp{
		Success: true,
		Data:    data,
	})
}

// Error writes an error JSON response.
func Error(w http.ResponseWriter, statusCode int, code, message string) {
	writeJSON(w, statusCode, ErrorResp{
		Code:    code,
		Message: message,
	})
}

// Paginated writes a paginated list JSON response.
func Paginated(w http.ResponseWriter, statusCode int, data interface{}, total, page, limit int) {
	writeJSON(w, statusCode, PaginatedResp{
		Data: data,
		Meta: PaginationMeta{
			Total: total,
			Page:  page,
			Limit: limit,
		},
	})
}

// Health writes the standard health check response.
func Health(w http.ResponseWriter, version string) {
	writeJSON(w, http.StatusOK, HealthResp{
		Status:  "ok",
		Version: version,
	})
}

// JSON writes any value as a JSON response with the given status code.
// This is the low-level helper; prefer Success/Error/Paginated for consistency.
func JSON(w http.ResponseWriter, statusCode int, v interface{}) {
	writeJSON(w, statusCode, v)
}

func writeJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// If encoding fails, we can't do much — the header is already sent.
		http.Error(w, `{"code":"INTERNAL_ERROR","message":"failed to encode response"}`, http.StatusInternalServerError)
	}
}
