package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/auth-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/auth-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/auth-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	// TODO: Replace with real PostgreSQL + Redis connections
	authRepo := repository.NewAuthRepository(nil, nil)
	authSvc := service.NewAuthService(authRepo, logger)
	authHandler := handler.NewAuthHandler(authSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Registration
	mux.HandleFunc("POST /auth/register", authHandler.Register)

	// OTP
	mux.HandleFunc("POST /auth/request-otp", authHandler.RequestOTP)
	mux.HandleFunc("POST /auth/verify-otp", authHandler.VerifyOTP)

	// Login
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.HandleFunc("POST /auth/login/otp", authHandler.LoginWithOTP)

	// Token
	mux.HandleFunc("POST /auth/refresh-token", authHandler.RefreshToken)

	// Logout
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.HandleFunc("POST /auth/logout-all", authHandler.LogoutAll)

	// Sessions
	mux.HandleFunc("GET /auth/sessions", authHandler.ListSessions)
	mux.HandleFunc("DELETE /auth/sessions/{sessionId}", authHandler.RevokeSession)

	// Password
	mux.HandleFunc("POST /auth/password/forgot", authHandler.ForgotPassword)
	mux.HandleFunc("POST /auth/password/reset", authHandler.ResetPassword)
	mux.HandleFunc("PUT /auth/password/change", authHandler.ChangePassword)

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

	logger.Info("starting clay-auth-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
