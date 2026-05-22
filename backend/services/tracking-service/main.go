package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-tubes/backend/pkg/database"
	"github.com/zicofarry/clay-tubes/backend/pkg/middleware"
	"github.com/zicofarry/clay-tubes/backend/pkg/response"
	"github.com/zicofarry/clay-tubes/backend/services/tracking-service/internal/handler"
	"github.com/zicofarry/clay-tubes/backend/services/tracking-service/internal/repository"
	"github.com/zicofarry/clay-tubes/backend/services/tracking-service/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	mongoCfg := database.DefaultMongoConfig("clay_tracking")
	if uri := os.Getenv("MONGO_URI"); uri != "" {
		mongoCfg.URI = uri
	}
	if dbName := os.Getenv("MONGO_DB"); dbName != "" {
		mongoCfg.Database = dbName
	}
	mongoClient, mongoDb, err := database.NewMongoClient(mongoCfg)
	if err != nil {
		logger.Error("failed to connect to mongodb", slog.Any("error", err))
		os.Exit(1)
	}
	_ = mongoClient // Ignore unused variable
	// Note: We don't defer Disconnect here so the app can continue running
	// Normally this is handled by graceful shutdown

	trackingRepo := repository.NewTrackingRepository(mongoDb)
	trackingSvc := service.NewTrackingService(trackingRepo, nil, logger)
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
