package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	// TODO: Wire real MongoDB and Redis clients here.
	trackingSvc := service.NewTrackingService(nil, nil, logger)
	trackingHandler := handler.NewTrackingHandler(trackingSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Tracking endpoints
	mux.HandleFunc("GET /tracking/orders/{orderId}/position", trackingHandler.GetOrderPosition)
	mux.HandleFunc("GET /tracking/orders/{orderId}/eta", trackingHandler.GetOrderETA)
	mux.HandleFunc("GET /tracking/orders/{orderId}/route", trackingHandler.GetOrderRoute)
	
	// Route endpoints
	mux.HandleFunc("GET /routes/{orderId}", trackingHandler.GetTripRoute)
	
	// Internal tracking lifecycle endpoints
	mux.HandleFunc("POST /internal/tracking/start", trackingHandler.StartTracking)
	mux.HandleFunc("POST /internal/tracking/{orderId}/stop", trackingHandler.StopTracking)
	mux.HandleFunc("PUT /internal/tracking/{orderId}/update", trackingHandler.PushLocationUpdate)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8006"
	}

	logger.Info("starting clay-tracking-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
