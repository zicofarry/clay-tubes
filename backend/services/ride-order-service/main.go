package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/ride-order-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/ride-order-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/ride-order-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	// TODO: Replace with real PostgreSQL + Redis connections (see clay-shared/pkg/database).
	repo := repository.NewRideOrderRepository(nil, nil)
	svc := service.NewRideOrderService(repo, logger)
	h := handler.NewRideOrderHandler(svc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Fare
	mux.HandleFunc("POST /orders/estimate", h.EstimateFare)

	// Order — user-facing
	mux.HandleFunc("POST /orders", h.CreateOrder)
	mux.HandleFunc("GET /orders/active", h.GetActiveOrder)
	mux.HandleFunc("GET /orders/history", h.GetOrderHistory)
	mux.HandleFunc("GET /orders/{orderId}", h.GetOrder)
	mux.HandleFunc("POST /orders/{orderId}/cancel", h.CancelOrder)
	mux.HandleFunc("POST /orders/{orderId}/rate", h.SubmitRating)
	mux.HandleFunc("GET /orders/{orderId}/fare-breakdown", h.GetFareBreakdown)

	// Driver
	mux.HandleFunc("POST /driver/orders/{orderId}/accept", h.DriverAcceptOrder)
	mux.HandleFunc("POST /driver/orders/{orderId}/reject", h.DriverRejectOrder)
	mux.HandleFunc("PUT /driver/orders/{orderId}/status", h.DriverUpdateOrderStatus)

	// Internal
	mux.HandleFunc("POST /internal/orders", h.InternalCreateOrder)
	mux.HandleFunc("GET /internal/orders/{orderId}", h.InternalGetOrder)
	mux.HandleFunc("PUT /internal/orders/{orderId}/status", h.InternalUpdateStatus)
	mux.HandleFunc("PUT /internal/orders/{orderId}/assign-driver", h.InternalAssignDriver)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var handler http.Handler = mux
	handler = middleware.Logger(logger)(handler)
	handler = middleware.Recovery(logger)(handler)
	handler = middleware.RequestID(handler)
	handler = middleware.CORS(middleware.DefaultCORSConfig())(handler)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "3003"
	}

	logger.Info("starting clay-ride-order-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
