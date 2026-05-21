package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/rating-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/rating-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/rating-service/internal/service"
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
		dsn = "host=localhost user=clay_user password=clay_password dbname=rating_db port=5445 sslmode=disable TimeZone=UTC"
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
		&repository.Rating{},
		&repository.DriverScoreAggregate{},
		&repository.MerchantScoreAggregate{},
	)
	if err != nil {
		log.Error("failed to run migrations", slog.Any("error", err))
		os.Exit(1)
	}

	repo := repository.NewRatingRepository(db)
	svc := service.NewRatingService(repo, log)
	h := handler.NewRatingHandler(svc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Rating
	mux.HandleFunc("POST /ratings", h.SubmitRating)
	mux.HandleFunc("GET /ratings/{subjectType}/{subjectId}", h.GetRatings)
	mux.HandleFunc("GET /ratings/orders/{orderId}", h.GetOrderRatings)
	mux.HandleFunc("GET /ratings/me/given", h.GetMyGivenRatings)
	mux.HandleFunc("GET /ratings/me/received", h.GetMyReceivedRatings)

	// Internal
	mux.HandleFunc("GET /internal/ratings/driver/{driverId}/score", h.GetDriverScore)
	mux.HandleFunc("GET /internal/ratings/{subjectType}/{subjectId}/average", h.GetAverageRating)
	mux.HandleFunc("POST /internal/ratings/batch-average", h.BatchGetAverageRatings)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var m http.Handler = mux
	m = middleware.Logger(log)(m)
	m = middleware.Recovery(log)(m)
	m = middleware.RequestID(m)
	m = middleware.CORS(middleware.DefaultCORSConfig())(m)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8015" // According to PORT_MAPPING.md for clay-rating-service
	}

	log.Info("starting clay-rating-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, m); err != nil {
		log.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
