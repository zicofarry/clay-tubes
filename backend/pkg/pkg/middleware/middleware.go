// Package middleware provides shared HTTP middleware for all Clay microservices.
//
// Available middleware:
//   - AuthContext: Extracts X-User-ID header (set by API Gateway) into request context
//   - RequestID: Generates or propagates X-Request-ID for distributed tracing
//   - Logger: Structured request logging with duration, status, path
//   - Recovery: Catches panics and returns 500 instead of crashing
//   - CORS: Cross-origin headers for browser clients
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ───── Context Keys ──────────────────────────────────────────────────────────

type contextKey string

const (
	// UserIDKey is the context key for the authenticated user's ID.
	UserIDKey contextKey = "user_id"
	// RequestIDKey is the context key for the request trace ID.
	RequestIDKey contextKey = "request_id"
)

// GetUserID extracts the user ID from the request context.
// Returns empty string if not present.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

// GetRequestID extracts the request ID from the request context.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}

// ───── AuthContext ────────────────────────────────────────────────────────────

// AuthContext extracts the X-User-ID header (injected by API Gateway after JWT
// validation) and places it into the request context.
//
// If the header is missing and required is true, returns 401.
// For internal/health endpoints, set required to false.
func AuthContext(required bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Header.Get("X-User-ID")

			if userID == "" && required {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":"UNAUTHORIZED","message":"missing X-User-ID header"}`))
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ───── RequestID ──────────────────────────────────────────────────────────────

// RequestID ensures every request has a unique X-Request-ID for distributed
// tracing. If the incoming request already has one, it's reused; otherwise a
// new UUID is generated.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		// Set on response so downstream services / clients can correlate.
		w.Header().Set("X-Request-ID", reqID)

		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ───── Logger ─────────────────────────────────────────────────────────────────

// statusRecorder captures the HTTP status code written by downstream handlers.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// Logger logs every request with structured fields:
// method, path, status, duration, request_id.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rec, r)

			duration := time.Since(start)
			logger.Info("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.statusCode),
				slog.Duration("duration", duration),
				slog.String("request_id", GetRequestID(r.Context())),
				slog.String("user_id", GetUserID(r.Context())),
			)
		})
	}
}

// ───── Recovery ───────────────────────────────────────────────────────────────

// Recovery catches panics in downstream handlers and returns a 500 error
// instead of crashing the process. Logs the panic with stack info.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						slog.Any("error", err),
						slog.String("path", r.URL.Path),
						slog.String("request_id", GetRequestID(r.Context())),
					)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"internal server error"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// ───── CORS ───────────────────────────────────────────────────────────────────

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	AllowOrigins []string
	AllowMethods []string
	AllowHeaders []string
}

// DefaultCORSConfig returns a sensible default CORS config for Clay services.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type", "Authorization", "X-User-ID", "X-Request-ID", "Idempotency-Key"},
	}
}

// CORS adds Cross-Origin Resource Sharing headers.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	origins := joinStrings(cfg.AllowOrigins)
	methods := joinStrings(cfg.AllowMethods)
	headers := joinStrings(cfg.AllowHeaders)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origins)
			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
