package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/broker"
	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/cache"
	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	// TODO: Replace with real PostgreSQL + Redis + Kafka connections
	paymentRepo := repository.NewPaymentRepository(nil, nil)

	// Redis (in-memory for local dev; replace with real go-redis client)
	redisClient := cache.NewInMemoryRedis()
	rateLimiter := cache.NewRateLimiter(redisClient, logger)

	// Kafka producer (log-only for local dev; replace with real producer)
	logProducer := broker.NewLogProducer(logger)
	paymentProducer := broker.NewPaymentProducer(logProducer, logger)

	paymentSvc := service.NewPaymentService(paymentRepo, logger, paymentProducer, rateLimiter)
	paymentHandler := handler.NewPaymentHandler(paymentSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Payment Methods
	mux.HandleFunc("GET /payment-methods", paymentHandler.ListPaymentMethods)
	mux.HandleFunc("POST /payment-methods", paymentHandler.AddPaymentMethod)
	mux.HandleFunc("DELETE /payment-methods/{methodId}", paymentHandler.DeletePaymentMethod)
	mux.HandleFunc("POST /payment-methods/{methodId}/set-default", paymentHandler.SetDefaultPaymentMethod)

	// COD Verification
	mux.HandleFunc("POST /cod/verify/initiate", paymentHandler.InitiateCodVerification)
	mux.HandleFunc("GET /cod/verify/{verificationId}/status", paymentHandler.GetCodVerificationStatus)
	mux.HandleFunc("POST /cod/verify/{verificationId}/otp", paymentHandler.SubmitCodOTP)
	mux.HandleFunc("POST /cod/verify/{verificationId}/respond", paymentHandler.RespondCodVerification)

	// Transactions
	mux.HandleFunc("GET /transactions", paymentHandler.GetTransactionHistory)
	mux.HandleFunc("GET /transactions/{transactionId}", paymentHandler.GetTransactionDetail)

	// Internal: Charge / Refund
	mux.HandleFunc("POST /internal/charges", paymentHandler.CreateCharge)
	mux.HandleFunc("POST /internal/refunds", paymentHandler.CreateRefund)
	mux.HandleFunc("GET /internal/transactions/{transactionId}/status", paymentHandler.GetTransactionStatus)

	// Internal: Hold / Capture / Release
	mux.HandleFunc("POST /internal/payments/hold", paymentHandler.HoldPayment)
	mux.HandleFunc("POST /internal/payments/capture", paymentHandler.CapturePayment)
	mux.HandleFunc("POST /internal/payments/release", paymentHandler.ReleasePayment)

	// Settlement
	mux.HandleFunc("POST /internal/settlements/create", paymentHandler.CreateSettlement)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("starting clay-payment-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
