package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/zicofarry/clay-app/backend/services/audit-log-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/audit-log-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/audit-log-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	
	// MongoDB connection
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27018"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(mongoURI)
	mongoClient, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		logger.Error("failed to connect to mongodb", slog.Any("error", err))
		os.Exit(1)
	}
	defer mongoClient.Disconnect(context.Background())

	if err := mongoClient.Ping(ctx, nil); err != nil {
		logger.Error("failed to ping mongodb", slog.Any("error", err))
		os.Exit(1)
	}

	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "audit_db"
	}
	db := mongoClient.Database(dbName)

	auditRepo := repository.NewAuditRepository(db)
	auditSvc := service.NewAuditService(auditRepo, logger)
	auditHandler := handler.NewAuditHandler(auditSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Query (Admin)
	mux.HandleFunc("GET /admin/logs", auditHandler.SearchLogs)
	mux.HandleFunc("GET /admin/logs/{logId}", auditHandler.GetLog)
	
	// Write (Internal)
	mux.HandleFunc("POST /internal/logs", auditHandler.CreateLog)
	mux.HandleFunc("POST /internal/logs/batch", auditHandler.CreateLogBatch)

	// Note: Export & Stats endpoints, along with by-resource/actor endpoints, 
	// can be wired here similarly by extending handler/service.

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8007"
	}

	logger.Info("starting clay-audit-log-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
