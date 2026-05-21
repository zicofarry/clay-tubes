// Package proxy implements the reverse-proxy core of the Clay API Gateway.
// Each registered route is forwarded to its upstream service with:
//   - Optional path prefix stripping
//   - Header forwarding (X-User-ID, X-User-Role, X-Request-ID)
//   - Upstream error handling with clean JSON error responses
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// transport is a shared HTTP transport tuned for internal service calls.
var transport = &http.Transport{
	MaxIdleConns:          200,
	MaxIdleConnsPerHost:   20,
	IdleConnTimeout:       90 * time.Second,
	ResponseHeaderTimeout: 30 * time.Second,
}

// New creates a reverse-proxy handler that forwards requests to upstreamAddr,
// optionally stripping stripPrefix from the request path before forwarding.
//
// upstreamAddr: "clay-auth-service:8080"  (scheme optional, defaults to http)
// stripPrefix:  "/ride"  → /ride/orders/{id} becomes /orders/{id} upstream
func New(upstreamAddr, stripPrefix string) (http.Handler, error) {
	rawURL := upstreamAddr
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "http://" + rawURL
	}

	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream address %q: %w", upstreamAddr, err)
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Transport = transport

	// Custom Director: adjust path + set X-Upstream-Target for logging
	originalDirector := rp.Director
	rp.Director = func(req *http.Request) {
		originalDirector(req)

		// Strip prefix if configured
		if stripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, stripPrefix)
		}

		// Tag for access log
		req.Header.Set("X-Upstream-Target", upstreamAddr)
		// Remove hop-by-hop headers that shouldn't be forwarded
		req.Header.Del("Connection")
		req.Header.Del("Upgrade")
	}

	// Error handler: turn upstream connection errors into clean JSON
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if isTimeout(err) {
			w.WriteHeader(http.StatusGatewayTimeout)
			fmt.Fprintf(w, `{"code":"UPSTREAM_TIMEOUT","message":"upstream service timed out"}`)
			return
		}

		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, `{"code":"UPSTREAM_UNAVAILABLE","message":"upstream service is unavailable"}`)
	}

	return rp, nil
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded")
}
