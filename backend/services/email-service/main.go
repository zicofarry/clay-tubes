package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/email-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/email-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/email-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6373/0"
	}
	emailRepo := repository.NewEmailRepository(redisUrl)
	emailSvc := service.NewEmailService(emailRepo, logger)
	emailHandler := handler.NewEmailHandler(emailSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Emails
	mux.HandleFunc("POST /internal/emails/send", emailHandler.SendEmail)
	mux.HandleFunc("GET /internal/emails/{emailId}/status", emailHandler.GetEmailStatus)

	// Webhooks
	mux.HandleFunc("POST /webhooks/email/delivery", emailHandler.HandleWebhook)

	// Templates
	mux.HandleFunc("GET /templates", emailHandler.GetTemplates)
	mux.HandleFunc("POST /templates", emailHandler.UpsertTemplate)

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

	logger.Info("starting clay-email-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
