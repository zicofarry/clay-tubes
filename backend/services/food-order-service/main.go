package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/database"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/kafka"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"

	_ "github.com/lib/pq"
)

func main() {
	// ── Setup Logger ─────────────────────────────────────────────────────
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Load Configuration (with defaults for Docker) ─────────────────────
	port := getEnv("PORT", "8017")

	// PostgreSQL Config
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	pgCfg := database.PostgresConfig{
		Host:     getEnv("DB_HOST", "clay_food_order_postgres"),
		Port:     dbPort,
		User:     getEnv("DB_USER", "clay"),
		Password: getEnv("DB_PASSWORD", "clay"),
		DBName:   getEnv("DB_NAME", "clay_food_order"),
		SSLMode:  getEnv("DB_SSL_MODE", "disable"),
	}

	// MongoDB Config
	mongoCfg := database.MongoConfig{
		URI:      getEnv("MONGO_URI", "mongodb://clay_food_order_mongo:27017"),
		Database: getEnv("MONGO_DATABASE", "clay_food_order"),
	}

	// Redis Config
	redisPort, _ := strconv.Atoi(getEnv("REDIS_PORT", "6379"))
	redisCfg := database.RedisConfig{
		Host:     getEnv("REDIS_HOST", "clay_food_order_redis"),
		Port:     redisPort,
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       0,
	}

	// Kafka Config
	kafkaBrokers := strings.Split(getEnv("KAFKA_BROKERS", "kafka:9092"), ",")

	// ── Initialize Dependencies ──────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. PostgreSQL
	db, err := database.NewPostgresDB(pgCfg)
	if err != nil {
		logger.Error("failed to connect to postgres", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	// 2. MongoDB
	mongoClient, mongoDB, err := database.NewMongoClient(mongoCfg)
	if err != nil {
		logger.Error("failed to connect to mongodb", slog.Any("error", err))
		os.Exit(1)
	}
	defer mongoClient.Disconnect(context.Background())

	// 3. Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr(),
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("failed to connect to redis", slog.Any("error", err))
		os.Exit(1)
	}
	defer rdb.Close()

	// 4. Kafka Producer
	var producer kafka.Producer
	if getEnv("KAFKA_ENABLED", "false") == "true" {
		producer = kafka.NewKafkaProducer(kafkaBrokers)
	} else {
		logger.Info("using no-op kafka producer")
		producer = kafka.NewNoopProducer()
	}
	defer producer.Close()

	// ── Wire Application Layers ──────────────────────────────────────────
	repo := repository.NewFoodOrderRepository(db, mongoDB, rdb)
	svc := service.NewFoodOrderService(repo, producer, logger)
	h := handler.NewFoodOrderHandler(svc, logger)

	// ── Setup Router ─────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// User Endpoints
	mux.HandleFunc("POST /orders/estimate", h.EstimateFare)
	mux.HandleFunc("POST /orders", h.CreateOrder)
	mux.HandleFunc("GET /orders/active", h.GetActiveOrder)
	mux.HandleFunc("GET /orders/history", h.GetOrderHistory)
	mux.HandleFunc("GET /orders/{orderId}", h.GetOrder)
	mux.HandleFunc("POST /orders/{orderId}/cancel", h.CancelOrder)
	mux.HandleFunc("POST /orders/{orderId}/rate", h.SubmitRating)
	mux.HandleFunc("GET /orders/{orderId}/fare-breakdown", h.GetFareBreakdown)

	// Merchant Endpoints
	mux.HandleFunc("GET /merchant/orders", h.MerchantListOrders)
	mux.HandleFunc("GET /merchant/orders/{orderId}", h.MerchantGetOrder)
	mux.HandleFunc("POST /merchant/orders/{orderId}/confirm", h.MerchantConfirmOrder)
	mux.HandleFunc("POST /merchant/orders/{orderId}/reject", h.MerchantRejectOrder)
	mux.HandleFunc("PUT /merchant/orders/{orderId}/status", h.MerchantUpdateStatus)

	// Driver Endpoints
	mux.HandleFunc("POST /driver/orders/{orderId}/pickup", h.DriverPickup)
	mux.HandleFunc("POST /driver/orders/{orderId}/deliver", h.DriverDeliver)

	// Internal (service-to-service)
	mux.HandleFunc("GET /internal/orders/{orderId}", h.InternalGetOrder)
	mux.HandleFunc("POST /internal/orders/{orderId}/assign-driver", h.InternalAssignDriver)
	mux.HandleFunc("GET /internal/users/{userId}/active-order", h.InternalGetUserActiveOrder)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var finalHandler http.Handler = mux
	finalHandler = middleware.Logger(logger)(finalHandler)
	finalHandler = middleware.Recovery(logger)(finalHandler)
	finalHandler = middleware.RequestID(finalHandler)
	finalHandler = middleware.CORS(middleware.DefaultCORSConfig())(finalHandler)

	// ── Graceful Shutdown ────────────────────────────────────────────────
	server := &http.Server{
		Addr:    ":" + port,
		Handler: finalHandler,
	}

	go func() {
		logger.Info("starting clay-food-order-service", slog.String("port", port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down gracefully...")
	ctxShut, cancelShut := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShut()

	if err := server.Shutdown(ctxShut); err != nil {
		logger.Error("shutdown failed", slog.Any("error", err))
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
