// Package router builds the gateway's HTTP mux from the routes.yaml config.
// Each route entry becomes a http.Handler chain:
//
//	RequestID → Recovery → AccessLog → CORS → Auth → RateLimit → ReverseProxy
package router

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/zicofarry/clay-app/backend/services/gateway/config"
	"github.com/zicofarry/clay-app/backend/services/gateway/middleware"
	"github.com/zicofarry/clay-app/backend/services/gateway/proxy"
)

// Build constructs and returns the root http.Handler for the gateway.
// It wires up every route from routes.yaml with its full middleware chain.
func Build(
	routes []config.Route,
	cfg *config.Config,
	rateLimiter *middleware.RateLimiter,
	logger *slog.Logger,
) (http.Handler, error) {
	mux := http.NewServeMux()

	// Cache proxy handlers per upstream+prefix to reuse connections
	proxyCache := map[string]http.Handler{}

	for _, route := range routes {
		// Build/reuse proxy for this upstream
		cacheKey := route.Upstream + "|" + route.StripPrefix
		rp, ok := proxyCache[cacheKey]
		if !ok {
			var err error
			rp, err = proxy.New(route.Upstream, route.StripPrefix)
			if err != nil {
				return nil, fmt.Errorf("route %s: %w", route.Path, err)
			}
			proxyCache[cacheKey] = rp
		}

		authRule := config.ParseAuthRule(route.Auth)

		// Convert {param} → go mux pattern (1.22+ supports {param})
		pattern := convertPath(route.Path)

		// Register one pattern per method to allow different auth/rate per method
		for _, method := range route.Methods {
			handler := buildChain(
				rp,
				cfg,
				authRule,
				rateLimiter,
				route.RateLimit,
				route.Path, // used as rate-limit key (pattern, not request path)
				logger,
			)
			mux.Handle(fmt.Sprintf("%s %s", strings.ToUpper(method), pattern), handler)
		}
	}

	// Health check — no auth, no rate limit
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"clay-gateway"}`))
	})

	// 404 fallback
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"code":"NOT_FOUND","message":"route %q not found"}`, r.URL.Path)
	})

	// Apply global middleware to the entire mux
	return applyGlobal(mux, cfg, logger), nil
}

// buildChain assembles the per-route middleware chain.
// Order: Auth → RateLimit → Proxy
func buildChain(
	rp http.Handler,
	cfg *config.Config,
	authRule config.AuthRule,
	rl *middleware.RateLimiter,
	rateLimit int,
	pathPattern string,
	logger *slog.Logger,
) http.Handler {
	h := rp

	// Rate limit wraps proxy
	if rateLimit > 0 {
		h = rl.Limit(rateLimit, pathPattern)(h)
	}

	// Auth wraps rate limit
	h = middleware.Auth(cfg.JWTSecret, cfg.JWTIssuer, authRule)(h)

	return h
}

// applyGlobal wraps the entire mux with gateway-level middleware.
// Order (outermost first): Recovery → RequestID → AccessLog → CORS
func applyGlobal(mux http.Handler, cfg *config.Config, logger *slog.Logger) http.Handler {
	h := mux
	h = cors(cfg.CORSOrigins)(h)
	h = middleware.AccessLog(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.Recovery(logger)(h)
	return h
}

// cors is a minimal CORS middleware for the gateway.
func cors(origins []string) func(http.Handler) http.Handler {
	originsVal := strings.Join(origins, ", ")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", originsVal)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers",
				"Content-Type, Authorization, X-Request-ID, Idempotency-Key")
			w.Header().Set("Access-Control-Expose-Headers",
				"X-Request-ID, X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// convertPath converts OpenAPI-style {param} path params to Go 1.22 mux syntax.
// e.g. /orders/{orderId}/cancel → /orders/{orderId}/cancel  (already compatible)
// Also normalises trailing slashes.
func convertPath(path string) string {
	// Go 1.22 mux already supports {param} syntax natively — no conversion needed.
	// Strip trailing slash (except root)
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	return path
}
