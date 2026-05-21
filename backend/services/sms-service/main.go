package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/services/sms-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/sms-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/sms-service/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Setup Redis ──────────────────────────────────────────────────────
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:9021/0"
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Error("failed to parse redis url", slog.Any("error", err))
		os.Exit(1)
	}
	rdb := redis.NewClient(opts)
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Error("failed to ping redis", slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("connected to redis")

	// ── Dependencies ─────────────────────────────────────────────────────
	smsRepo := repository.NewSMSRepository(rdb)
	smsSvc := service.NewSMSService(smsRepo, logger)
	smsHandler := handler.NewSMSHandler(smsSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// SMS OTP Endpoints
	mux.HandleFunc("POST /internal/sms/otp/send", smsHandler.SendOTP)
	mux.HandleFunc("POST /internal/sms/otp/verify", smsHandler.VerifyOTP)

	// SMS Endpoints
	mux.HandleFunc("POST /internal/sms/send", smsHandler.SendSMS)
	mux.HandleFunc("POST /webhooks/sms/delivery", smsHandler.ProcessWebhook)
	mux.HandleFunc("GET /internal/sms/status/{smsId}", smsHandler.GetSMSStatus)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8021"
	}

	logger.Info("starting clay-sms-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
