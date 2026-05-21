package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/database"
	sharedKafka "github.com/zicofarry/clay-app/backend/pkg/pkg/kafka"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── PostgreSQL ────────────────────────────────────────────────────────
	pgCfg := database.DefaultPostgresConfig()
	pgCfg.Host = envOrDefault("DB_HOST", "localhost")
	pgCfg.Port = 5441
	pgCfg.DBName = envOrDefault("DB_NAME", "clay_merchant")
	pgCfg.User = envOrDefault("DB_USER", "clay")
	pgCfg.Password = envOrDefault("DB_PASSWORD", "clay")

	db, err := database.NewPostgresDB(pgCfg)
	if err != nil {
		logger.Error("postgres connection failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	// ── MongoDB ───────────────────────────────────────────────────────────
	mongoCfg := database.MongoConfig{
		URI:      envOrDefault("MONGO_URI", "mongodb://localhost:27020"),
		Database: envOrDefault("MONGO_DB", "clay_merchant"),
	}
	mongoClient, mongoDB, err := database.NewMongoClient(mongoCfg)
	if err != nil {
		logger.Error("mongodb connection failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		_ = mongoClient.Disconnect(context.Background())
	}()

	// ── Kafka ─────────────────────────────────────────────────────────────
	kafkaBrokers := strings.Split(envOrDefault("KAFKA_BROKERS", "localhost:9092"), ",")

	var kafkaProducer sharedKafka.Producer
	if os.Getenv("KAFKA_DISABLED") == "true" {
		kafkaProducer = sharedKafka.NewNoopProducer()
	} else {
		kafkaProducer = sharedKafka.NewKafkaProducer(kafkaBrokers)
	}
	defer kafkaProducer.Close()

	// ── Dependencies ──────────────────────────────────────────────────────
	merchantRepo := repository.NewMerchantRepository(db)
	menuRepo := repository.NewMenuRepository(mongoDB)
	svc := service.NewMerchantService(merchantRepo, menuRepo, kafkaProducer, logger)
	h := handler.NewMerchantHandler(svc, logger)

	// ── Router ────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Merchant profile
	mux.HandleFunc("POST /merchants", h.RegisterMerchant)
	mux.HandleFunc("GET /merchants/me", h.GetMyMerchant)
	mux.HandleFunc("PUT /merchants/me", h.UpdateMyMerchant)
	mux.HandleFunc("GET /merchants/{merchantId}", h.GetMerchantByID)
	mux.HandleFunc("PATCH /merchants/{merchantId}/status", h.UpdateMerchantStatus)

	// Operating hours
	mux.HandleFunc("GET /merchants/{merchantId}/operating-hours", h.GetOperatingHours)
	mux.HandleFunc("PUT /merchants/{merchantId}/operating-hours", h.UpsertOperatingHours)

	// Bank accounts
	mux.HandleFunc("GET /merchants/{merchantId}/bank-accounts", h.ListBankAccounts)
	mux.HandleFunc("POST /merchants/{merchantId}/bank-accounts", h.AddBankAccount)
	mux.HandleFunc("DELETE /merchants/{merchantId}/bank-accounts/{accountId}", h.DeleteBankAccount)
	mux.HandleFunc("PATCH /merchants/{merchantId}/bank-accounts/{accountId}/set-primary", h.SetPrimaryBankAccount)

	// Menu categories
	mux.HandleFunc("GET /merchants/{merchantId}/menu/categories", h.ListCategories)
	mux.HandleFunc("POST /merchants/{merchantId}/menu/categories", h.CreateCategory)
	mux.HandleFunc("PATCH /merchants/{merchantId}/menu/categories/reorder", h.ReorderCategories)
	mux.HandleFunc("PUT /merchants/{merchantId}/menu/categories/{categoryId}", h.UpdateCategory)
	mux.HandleFunc("DELETE /merchants/{merchantId}/menu/categories/{categoryId}", h.DeleteCategory)

	// Menu items
	mux.HandleFunc("GET /merchants/{merchantId}/menu/items", h.ListItems)
	mux.HandleFunc("POST /merchants/{merchantId}/menu/items", h.CreateItem)
	mux.HandleFunc("GET /merchants/{merchantId}/menu/items/{itemId}", h.GetItem)
	mux.HandleFunc("PUT /merchants/{merchantId}/menu/items/{itemId}", h.UpdateItem)
	mux.HandleFunc("DELETE /merchants/{merchantId}/menu/items/{itemId}", h.DeleteItem)
	mux.HandleFunc("PATCH /merchants/{merchantId}/menu/items/{itemId}/availability", h.ToggleAvailability)

	// Internal (service-to-service)
	mux.HandleFunc("GET /internal/merchants/{merchantId}", h.InternalGetMerchant)
	mux.HandleFunc("GET /internal/merchants/{merchantId}/is-open", h.InternalIsOpen)
	mux.HandleFunc("POST /internal/menu-items/batch", h.InternalBatchGetItems)

	// ── Middleware stack ──────────────────────────────────────────────────
	var httpHandler http.Handler = mux
	httpHandler = middleware.AuthContext(false)(httpHandler)
	httpHandler = middleware.Logger(logger)(httpHandler)
	httpHandler = middleware.Recovery(logger)(httpHandler)
	httpHandler = middleware.RequestID(httpHandler)
	httpHandler = middleware.CORS(middleware.DefaultCORSConfig())(httpHandler)

	// ── HTTP server ───────────────────────────────────────────────────────
	port := envOrDefault("PORT", "8011")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      httpHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("clay-merchant-service starting", slog.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-quit
	logger.Info("shutting down merchant-service...")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
	logger.Info("merchant-service stopped")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
