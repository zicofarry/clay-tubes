package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/services/gateway/config"
	"github.com/zicofarry/clay-app/backend/services/gateway/middleware"
	"github.com/zicofarry/clay-app/backend/services/gateway/router"
)

func main() {
	// ── Logger ────────────────────────────────────────────────────────────────
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	// ── Routes ────────────────────────────────────────────────────────────────
	routesPath := envOrDefault("ROUTES_FILE", "config/routes.yaml")
	routes, err := config.LoadRoutes(routesPath)
	if err != nil {
		logger.Error("failed to load routes", slog.String("path", routesPath), slog.Any("error", err))
		os.Exit(1)
	}
	logger.Info("routes loaded", slog.Int("count", len(routes)))

	// ── Redis (rate limiting) ─────────────────────────────────────────────────
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		// Non-fatal: gateway can start without Redis, rate limiting will be skipped
		logger.Warn("redis unavailable, rate limiting disabled", slog.Any("error", err))
	} else {
		logger.Info("redis connected", slog.String("addr", cfg.RedisAddr))
	}

	rateLimiter := middleware.NewRateLimiter(rdb)

	// ── Router ────────────────────────────────────────────────────────────────
	handler, err := router.Build(routes, cfg, rateLimiter, logger)
	if err != nil {
		logger.Error("failed to build router", slog.Any("error", err))
		os.Exit(1)
	}

	// ── HTTP Server ───────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("gateway starting", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-quit
	logger.Info("shutting down gateway...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.Any("error", err))
		os.Exit(1)
	}

	_ = rdb.Close()
	logger.Info("gateway stopped")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
