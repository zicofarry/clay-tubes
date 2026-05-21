package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/services/push-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/push-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/push-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisURL,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Error("failed to connect to redis", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("connected to redis", slog.String("url", redisURL))

	// TODO: Replace with real FCM/APNs SDK connections
	pushRepo := repository.NewPushRepository(logger, redisClient)
	pushSvc := service.NewPushService(pushRepo, logger)
	pushHandler := handler.NewPushHandler(pushSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Delivery
	mux.HandleFunc("POST /internal/push/send", pushHandler.SendPush)
	mux.HandleFunc("POST /internal/push/send-batch", pushHandler.SendBatchPush)

	// Topic
	mux.HandleFunc("POST /internal/push/topics/{topicName}/subscribe", pushHandler.SubscribeTopic)
	mux.HandleFunc("POST /internal/push/topics/{topicName}/unsubscribe", pushHandler.UnsubscribeTopic)
	mux.HandleFunc("POST /internal/push/topics/{topicName}/send", pushHandler.SendTopicPush)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8014"
	}

	logger.Info("starting clay-push-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
