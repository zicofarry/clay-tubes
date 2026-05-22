//go:build functional

package functional

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

var gatewayURL string

type dummyLogger struct{}

func (dummyLogger) Printf(ctx context.Context, format string, v ...interface{}) {}

func TestMain(m *testing.M) {
	// Silence go-redis internal connection warnings
	redis.SetLogger(dummyLogger{})

	// 1. Verify Redis dependency is actually running before compiling/running tests
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6370",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		cancel()
		rdb.Close()
		fmt.Println("FAIL: Redis dependency (Docker) is not running on port 6370. Please run 'docker compose up -d' first.")
		os.Exit(1)
	}
	cancel()
	rdb.Close()

	port := envOrDefault("GATEWAY_TEST_PORT", "18080")
	gatewayURL = fmt.Sprintf("http://localhost:%s", port)

	// 2. Build the gateway binary first to avoid go run zombie processes
	var binaryPath string
	if os.PathSeparator == '/' {
		binaryPath = "./gateway.test"
	} else {
		binaryPath = `.\gateway.test.exe`
	}

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./main.go")
	buildCmd.Dir = "../../"
	if err := buildCmd.Run(); err != nil {
		fmt.Printf("failed to build gateway binary: %v\n", err)
		os.Exit(1)
	}

	// 3. Start gateway binary directly in background for testing
	cmd := exec.Command(binaryPath)
	cmd.Dir = "../../"
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%s", port),
		"REDIS_ADDR=localhost:6370",
		"JWT_SECRET=test-secret",
		"CORS_ORIGINS=*",
	)

	logFile, err := os.Create("gateway_test_run.log")
	if err != nil {
		fmt.Printf("failed to create log file: %v\n", err)
		os.Remove("../../" + binaryPath)
		os.Exit(1)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		fmt.Printf("failed to start gateway: %v\n", err)
		_ = logFile.Close()
		os.Remove("../../" + binaryPath)
		os.Exit(1)
	}

	// Wait for gateway to be ready
	ready := false
	for i := 0; i < 15; i++ {
		resp, err := http.Get(gatewayURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !ready {
		fmt.Println("gateway did not become ready in time")
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		_ = os.Remove("../../" + binaryPath)
		os.Exit(1)
	}

	code := m.Run()

	_ = cmd.Process.Kill()
	_ = logFile.Close()
	_ = os.Remove("../../" + binaryPath) // Clean up test binary
	os.Exit(code)
}

// ─── Health Check ────────────────────────────────────────────────────────────

func TestHealthEndpoint(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
	if body["service"] != "clay-gateway" {
		t.Errorf("expected service 'clay-gateway', got %q", body["service"])
	}
}

// ─── CORS ────────────────────────────────────────────────────────────────────

func TestCORSPreflight(t *testing.T) {
	req, _ := http.NewRequest(http.MethodOptions, gatewayURL+"/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("CORS preflight request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status 204 for OPTIONS, got %d", resp.StatusCode)
	}

	acaoHeader := resp.Header.Get("Access-Control-Allow-Origin")
	if acaoHeader == "" {
		t.Error("missing Access-Control-Allow-Origin header")
	}
}

// ─── 404 Not Found ──────────────────────────────────────────────────────────

func TestNotFoundRoute(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/this-route-does-not-exist")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode 404 response: %v", err)
	}

	if body["code"] != "NOT_FOUND" {
		t.Errorf("expected code 'NOT_FOUND', got %q", body["code"])
	}
}

// ─── Request ID Header ──────────────────────────────────────────────────────

func TestRequestIDHeader(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	reqID := resp.Header.Get("X-Request-ID")
	if reqID == "" {
		t.Error("missing X-Request-ID header in response")
	}
}

// ─── Rate Limiter (Requires Redis) ──────────────────────────────────────────

func TestRateLimiting(t *testing.T) {
	// Hits /auth/password/forgot (which is public but has a rate_limit of 5 per minute)
	// The first 5 requests should bypass the rate limiter but return 502 Bad Gateway
	// (since the upstream clay-auth-service is not running).
	// The 6th request MUST be blocked by the rate limiter and return 429 Too Many Requests.
	// This test requires the Redis container to be active.
	for i := 0; i < 5; i++ {
		resp, err := http.Post(gatewayURL+"/auth/password/forgot", "application/json", nil)
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("request %d was rate limited prematurely", i+1)
		}
	}

	// 6th request
	resp, err := http.Post(gatewayURL+"/auth/password/forgot", "application/json", nil)
	if err != nil {
		t.Fatalf("6th request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests for 6th request, got %d (is Redis running?)", resp.StatusCode)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
