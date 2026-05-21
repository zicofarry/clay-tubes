package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/history-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/history-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/history-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	// ── Dependencies ─────────────────────────────────────────────────────

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "host=localhost user=clay_user password=clay_password dbname=history_db port=5453 sslmode=disable TimeZone=UTC"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Error("failed to connect to postgres", slog.Any("error", err))
		os.Exit(1)
	}

	// Auto Migration based on models
	err = db.AutoMigrate(
		&repository.OrderHistory{},
		&repository.ActivityFeed{},
	)
	if err != nil {
		log.Error("failed to run migrations", slog.Any("error", err))
		os.Exit(1)
	}

	historyRepo := repository.NewHistoryRepository(db)
	historySvc := service.NewHistoryService(historyRepo, log)
	historyHandler := handler.NewHistoryHandler(historySvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Order History
	mux.HandleFunc("GET /history/orders", historyHandler.ListMyOrderHistory)
	mux.HandleFunc("GET /history/orders/{orderId}", historyHandler.GetOrderHistoryDetail)
	mux.HandleFunc("GET /driver/history/orders", historyHandler.ListDriverTripHistory)

	// Activity Feed
	mux.HandleFunc("GET /feed", historyHandler.GetMyActivityFeed)
	mux.HandleFunc("GET /feed/{feedId}", historyHandler.GetActivityFeedEntry)

	// Internal Sync
	mux.HandleFunc("POST /internal/history/sync", historyHandler.InternalSyncOrderHistory)
	mux.HandleFunc("POST /internal/feed", historyHandler.InternalCreateFeedEntry)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(log)(h)
	h = middleware.Recovery(log)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8023" // According to PORT_MAPPING.md for clay-history-service
	}

	log.Info("starting clay-history-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
