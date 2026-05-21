package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/services/wallet-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/wallet-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/wallet-service/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── DB Connection ────────────────────────────────────────────────────
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Fallback to docker-compose default
		dsn = "postgres://clay_user:clay_password@localhost:5452/wallet_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		logger.Error("failed to open db", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Error("failed to ping db", slog.Any("error", err))
		// Don't exit immediately, might be starting up in docker.
	}

	// ── Dependencies ─────────────────────────────────────────────────────
	walletRepo := repository.NewWalletRepository(db)
	walletSvc := service.NewWalletService(walletRepo, logger)
	walletHandler := handler.NewWalletHandler(walletSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Wallet
	mux.HandleFunc("GET /wallet", walletHandler.GetWallet)
	mux.HandleFunc("POST /wallet/topup", walletHandler.TopUp)
	
	// Internal Wallet Operations
	mux.HandleFunc("POST /internal/wallet/debit", walletHandler.Debit)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8022"
	}

	logger.Info("starting clay-wallet-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
