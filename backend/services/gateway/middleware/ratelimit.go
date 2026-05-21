package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter holds a Redis client for rate limit counters.
type RateLimiter struct {
	rdb *redis.Client
}

// NewRateLimiter creates a RateLimiter backed by Redis.
func NewRateLimiter(rdb *redis.Client) *RateLimiter {
	return &RateLimiter{rdb: rdb}
}

// Limit returns a middleware that enforces requests-per-minute using a sliding
// window counter in Redis.
//
// limitPerMinute == 0 → no limiting (skip).
//
// Key strategy:
//   - Authenticated users:  rate:{user_id}:{path_pattern}
//   - Unauthenticated:      rate:{client_ip}:{path_pattern}
//
// This scopes limits per-user so one user cannot starve another.
func (rl *RateLimiter) Limit(limitPerMinute int, pathPattern string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limitPerMinute <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			identifier := rateLimitIdentifier(r, pathPattern)
			allowed, remaining, resetAfter, err := rl.checkAndIncrement(r.Context(), identifier, limitPerMinute)
			if err != nil {
				// Redis failure → fail open (don't block requests if Redis is down)
				next.ServeHTTP(w, r)
				return
			}

			// Always set rate limit headers so clients can adapt
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limitPerMinute))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAfter))

			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", resetAfter))
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprintf(w, `{"code":"RATE_LIMITED","message":"too many requests, retry after %d seconds"}`, resetAfter)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// checkAndIncrement performs an atomic INCR + EXPIRE using a Redis pipeline.
// Returns (allowed, remaining, resetAfterSeconds, error).
func (rl *RateLimiter) checkAndIncrement(ctx context.Context, key string, limit int) (bool, int, int64, error) {
	const windowSecs = 60

	pipe := rl.rdb.Pipeline()
	incrCmd := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Duration(windowSecs)*time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, 0, err
	}

	count := int(incrCmd.Val())
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	// Fetch actual TTL so reset header is accurate
	ttl, err := rl.rdb.TTL(ctx, key).Result()
	var resetAfter int64
	if err == nil && ttl > 0 {
		resetAfter = int64(ttl.Seconds())
	} else {
		resetAfter = windowSecs
	}

	return count <= limit, remaining, resetAfter, nil
}

// rateLimitIdentifier returns a Redis key scoped to the user (or IP if not authed).
func rateLimitIdentifier(r *http.Request, pathPattern string) string {
	userID := r.Header.Get("X-User-ID")
	if userID != "" {
		return fmt.Sprintf("rate:%s:%s", userID, pathPattern)
	}
	return fmt.Sprintf("rate:%s:%s", clientIP(r), pathPattern)
}

// clientIP extracts the real client IP, respecting X-Forwarded-For.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may be "client, proxy1, proxy2" — take the first
		if idx := len(xff); idx > 0 {
			for i, c := range xff {
				if c == ',' {
					return xff[:i]
				}
			}
			return xff
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
