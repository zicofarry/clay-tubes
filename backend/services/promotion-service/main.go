package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/promotion-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/promotion-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/promotion-service/internal/service"
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
		dsn = "host=localhost user=clay_user password=clay_password dbname=promotion_db port=5443 sslmode=disable TimeZone=UTC"
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
		&repository.PromoCode{},
		&repository.PromoTarget{},
		&repository.UserPromo{},
		&repository.PromoUsage{},
	)
	if err != nil {
		log.Error("failed to run migrations", slog.Any("error", err))
		os.Exit(1)
	}

	repo := repository.NewPromotionRepository(db)
	svc := service.NewPromotionService(repo, log)
	h := handler.NewPromotionHandler(svc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Promo
	mux.HandleFunc("POST /promos/validate", h.ValidatePromo)

	// Voucher
	mux.HandleFunc("GET /vouchers", h.ListMyVouchers)
	mux.HandleFunc("POST /vouchers/claim", h.ClaimVoucher)

	// Admin
	mux.HandleFunc("GET /admin/promos", h.AdminListPromos)
	mux.HandleFunc("POST /admin/promos", h.CreatePromo)
	mux.HandleFunc("PUT /admin/promos/{promoId}", h.UpdatePromo)
	mux.HandleFunc("DELETE /admin/promos/{promoId}", h.DeactivatePromo)

	// Internal
	mux.HandleFunc("POST /internal/promos/apply", h.ApplyPromo)
	mux.HandleFunc("POST /internal/promos/release", h.ReleasePromo)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var m http.Handler = mux
	m = middleware.Logger(log)(m)
	m = middleware.Recovery(log)(m)
	m = middleware.RequestID(m)
	m = middleware.CORS(middleware.DefaultCORSConfig())(m)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8013" // According to PORT_MAPPING.md for clay-promotion-service
	}

	log.Info("starting clay-promotion-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, m); err != nil {
		log.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
