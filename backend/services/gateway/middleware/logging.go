package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseRecorder captures status code and bytes written for logging.
type responseRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	n, err := rr.ResponseWriter.Write(b)
	rr.bytesWritten += n
	return n, err
}

// AccessLog logs every proxied request with structured fields.
// Designed for the gateway — logs upstream target in addition to standard fields.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rec, r)

			duration := time.Since(start)

			level := slog.LevelInfo
			if rec.statusCode >= 500 {
				level = slog.LevelError
			} else if rec.statusCode >= 400 {
				level = slog.LevelWarn
			}

			logger.Log(r.Context(), level, "access",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("query", r.URL.RawQuery),
				slog.Int("status", rec.statusCode),
				slog.Int("bytes", rec.bytesWritten),
				slog.Duration("duration", duration),
				slog.String("request_id", r.Header.Get("X-Request-ID")),
				slog.String("user_id", r.Header.Get("X-User-ID")),
				slog.String("user_role", r.Header.Get("X-User-Role")),
				slog.String("remote_addr", clientIP(r)),
				slog.String("user_agent", r.Header.Get("User-Agent")),
				slog.String("upstream", r.Header.Get("X-Upstream-Target")),
			)
		})
	}
}

// RequestID ensures every request carries a unique X-Request-ID for tracing.
// Reuses the incoming ID if present; generates a new one otherwise.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", reqID)
		r.Header.Set("X-Request-ID", reqID) // forward to upstream
		next.ServeHTTP(w, r)
	})
}

// Recovery catches panics and returns 500 without crashing the gateway.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("gateway panic",
						slog.Any("error", err),
						slog.String("path", r.URL.Path),
						slog.String("request_id", r.Header.Get("X-Request-ID")),
					)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"code":"INTERNAL_ERROR","message":"gateway internal error"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// generateRequestID produces a short unique ID using timestamp nanoseconds.
// Enough for log correlation across services.
func generateRequestID() string {
	const hex = "0123456789abcdef"
	ns := time.Now().UnixNano()
	buf := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		buf[i] = hex[ns&0xf]
		ns >>= 4
	}
	return string(buf)
}
