package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/security-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/security-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/security-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	// TODO: Replace with real PostgreSQL + Redis connections (see clay-shared/pkg/database).
	repo := repository.NewSecurityRepository(nil, nil)
	svc := service.NewSecurityService(repo, logger)
	h := handler.NewSecurityHandler(svc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Login Attempts
	mux.HandleFunc("GET /login-attempts", h.ListMyLoginAttempts)
	mux.HandleFunc("GET /admin/login-attempts", h.AdminListLoginAttempts)

	// Fraud Flags
	mux.HandleFunc("GET /admin/fraud-flags", h.ListFraudFlags)
	mux.HandleFunc("POST /admin/fraud-flags", h.CreateFraudFlag)
	mux.HandleFunc("GET /admin/fraud-flags/{flagId}", h.GetFraudFlag)
	mux.HandleFunc("POST /admin/fraud-flags/{flagId}/resolve", h.ResolveFraudFlag)
	mux.HandleFunc("GET /admin/users/{userId}/fraud-summary", h.GetUserFraudSummary)

	// IP Blacklist
	mux.HandleFunc("GET /admin/ip-blacklist", h.ListBlockedIPs)
	mux.HandleFunc("POST /admin/ip-blacklist", h.BlockIP)
	mux.HandleFunc("DELETE /admin/ip-blacklist/{blockId}", h.UnblockIP)

	// Internal / Validation
	mux.HandleFunc("POST /internal/validate/ip", h.ValidateIP)
	mux.HandleFunc("POST /internal/validate/user", h.ValidateUser)
	mux.HandleFunc("POST /internal/login-attempts", h.RecordLoginAttempt)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var serverHandler http.Handler = mux
	serverHandler = middleware.Logger(logger)(serverHandler)
	serverHandler = middleware.Recovery(logger)(serverHandler)
	serverHandler = middleware.RequestID(serverHandler)
	serverHandler = middleware.CORS(middleware.DefaultCORSConfig())(serverHandler)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8005"
	}

	logger.Info("starting clay-security-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, serverHandler); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
