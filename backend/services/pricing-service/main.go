package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/pricing-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/pricing-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/pricing-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	// TODO: Replace with real PostgreSQL + Redis connections
	pricingRepo := repository.NewPricingRepository(nil)
	pricingSvc := service.NewPricingService(pricingRepo, logger)
	pricingHandler := handler.NewPricingHandler(pricingSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Estimate
	mux.HandleFunc("POST /estimate/ride", pricingHandler.EstimateRideFare)
	mux.HandleFunc("POST /estimate/delivery", pricingHandler.EstimateDeliveryFare)
	mux.HandleFunc("POST /estimate/food", pricingHandler.EstimateFoodFare)

	// Surge
	mux.HandleFunc("GET /surge", pricingHandler.GetSurge)

	// Fare
	mux.HandleFunc("POST /fare/calculate", pricingHandler.CalculateFinalFare)

	// Internal
	mux.HandleFunc("GET /internal/fare-rules", pricingHandler.GetFareRules)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8012"
	}

	logger.Info("starting clay-pricing-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
