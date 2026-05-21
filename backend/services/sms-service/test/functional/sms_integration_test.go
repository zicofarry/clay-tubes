//go:build functional

package functional

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/services/sms-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/sms-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/sms-service/internal/service"
)

func TestFunctional_SendOTP(t *testing.T) {
	// 1. Setup Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:9021",
	})
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("failed to ping redis: %v", err)
	}

	// Clean up before test
	rdb.FlushDB(context.Background())

	// 2. Setup Dependencies
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	repo := repository.NewSMSRepository(rdb)
	svc := service.NewSMSService(repo, logger)
	smsHandler := handler.NewSMSHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/sms/otp/send", smsHandler.SendOTP)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	reqBody := service.SendOTPRequest{
		Phone:   "+628123456789",
		Purpose: "login",
	}
	body, _ := json.Marshal(reqBody)

	resp, err := http.Post(ts.URL+"/internal/sms/otp/send", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.StatusCode)
	}

	// Verify Redis state
	otp, err := repo.GetOTP(context.Background(), reqBody.Phone, reqBody.Purpose)
	if err != nil {
		t.Fatalf("failed to get otp from redis: %v", err)
	}
	if otp == "" {
		t.Errorf("expected otp to be set in redis")
	}

	// Verify rate limit was incremented
	count, err := repo.GetRateLimit(context.Background(), reqBody.Phone)
	if err != nil {
		t.Fatalf("failed to get rate limit: %v", err)
	}
	if count != 1 {
		t.Errorf("expected rate limit count 1, got %d", count)
	}
}
