// Package router builds the gateway's HTTP mux from the routes.yaml config.
// Each route entry becomes a http.Handler chain:
//
//	RequestID → Recovery → AccessLog → CORS → Auth → RateLimit → ReverseProxy
package router

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/zicofarry/clay-tubes/backend/services/gateway/config"
	"github.com/zicofarry/clay-tubes/backend/services/gateway/middleware"
	"github.com/zicofarry/clay-tubes/backend/services/gateway/proxy"
)

type routeMatcher struct {
	pathPattern string
	methods     map[string]bool
	regex       *regexp.Regexp
	handler     http.Handler
}

// Build constructs and returns the root http.Handler for the gateway.
// It wires up every route from routes.yaml using a regex-based matcher.
func Build(
	routes []config.Route,
	cfg *config.Config,
	rateLimiter *middleware.RateLimiter,
	logger *slog.Logger,
) (http.Handler, error) {
	var matchers []routeMatcher
	proxyCache := map[string]http.Handler{}

	// Compile regex helper to replace {param} with [^/]+
	paramRegex := regexp.MustCompile(`\\\{[^\\\}]+\\\}`)

	for _, route := range routes {
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

		// Create the route's handler chain
		handler := buildChain(
			rp,
			cfg,
			authRule,
			rateLimiter,
			route.RateLimit,
			route.Path,
			logger,
		)

		// Clean and compile the path pattern to regex
		cleanPattern := cleanPath(route.Path)
		escapedPattern := regexp.QuoteMeta(cleanPattern)
		regexPattern := "^" + paramRegex.ReplaceAllString(escapedPattern, `[^/]+`) + "$"
		
		compiledRegex, err := regexp.Compile(regexPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regex for route %s: %w", route.Path, err)
		}

		methodsMap := make(map[string]bool)
		for _, m := range route.Methods {
			methodsMap[strings.ToUpper(m)] = true
		}

		matchers = append(matchers, routeMatcher{
			pathPattern: route.Path,
			methods:     methodsMap,
			regex:       compiledRegex,
			handler:     handler,
		})
	}

	// Define the root dynamic router handler
	routerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqPath := cleanPath(r.URL.Path)

		// 1. Health check (highest priority, no auth/rate limit)
		if r.Method == http.MethodGet && reqPath == "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","service":"clay-gateway"}`))
			return
		}

		// 2. Match request against routes in order
		reqMethod := strings.ToUpper(r.Method)
		for _, matcher := range matchers {
			if matcher.methods[reqMethod] && matcher.regex.MatchString(reqPath) {
				matcher.handler.ServeHTTP(w, r)
				return
			}
		}

		// 3. 404 Fallback
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		resp := map[string]string{
			"code":    "NOT_FOUND",
			"message": fmt.Sprintf("route %s not found", r.URL.Path),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Apply global middleware to the entire router
	return applyGlobal(routerHandler, cfg, logger), nil
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

// applyGlobal wraps the entire router with gateway-level middleware.
// Order (outermost first): Recovery → RequestID → AccessLog → CORS
func applyGlobal(router http.Handler, cfg *config.Config, logger *slog.Logger) http.Handler {
	h := router
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

// cleanPath normalizes path by stripping trailing slashes.
func cleanPath(path string) string {
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	return path
}
