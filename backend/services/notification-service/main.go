package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/notification-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/notification-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/notification-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/database"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	// Initialize PostgreSQL Connection
	dbConfig := database.DefaultPostgresConfig()
	dbConfig.Host = os.Getenv("DB_HOST")
	if dbConfig.Host == "" {
		dbConfig.Host = "localhost"
	}
	dbConfig.Port = 5435 // matching local docker-compose for notification service
	dbConfig.User = "clay_user"
	dbConfig.Password = "clay_password"
	dbConfig.DBName = "notification_db"

	db, err := database.NewPostgresDB(dbConfig)
	if err != nil {
		logger.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("connected to postgres", slog.String("host", dbConfig.Host), slog.Int("port", dbConfig.Port))

	notifRepo := repository.NewNotificationRepository(db)
	notifSvc := service.NewNotificationService(notifRepo, logger)
	notifHandler := handler.NewNotificationHandler(notifSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Device Tokens
	mux.HandleFunc("POST /device-tokens", notifHandler.RegisterDeviceToken)
	mux.HandleFunc("GET /device-tokens", notifHandler.ListDeviceTokens)
	mux.HandleFunc("DELETE /device-tokens/{tokenId}", notifHandler.DeactivateDeviceToken)

	// Preferences
	mux.HandleFunc("GET /preferences", notifHandler.GetPreferences)
	mux.HandleFunc("PUT /preferences", notifHandler.UpdatePreferences)

	// Notifications
	mux.HandleFunc("GET /notifications", notifHandler.ListNotifications)
	mux.HandleFunc("GET /notifications/{notificationId}", notifHandler.GetNotification)

	// Templates (Admin)
	mux.HandleFunc("GET /admin/templates", notifHandler.ListTemplates)
	mux.HandleFunc("POST /admin/templates", notifHandler.CreateTemplate)
	mux.HandleFunc("GET /admin/templates/{templateId}", notifHandler.GetTemplate)
	mux.HandleFunc("PUT /admin/templates/{templateId}", notifHandler.UpdateTemplate)
	mux.HandleFunc("DELETE /admin/templates/{templateId}", notifHandler.DeleteTemplate)
	mux.HandleFunc("POST /admin/templates/{templateId}/preview", notifHandler.PreviewTemplate)

	// Internal
	mux.HandleFunc("POST /internal/send", notifHandler.InternalSendNotification)
	mux.HandleFunc("POST /internal/send/batch", notifHandler.InternalSendBatch)
	mux.HandleFunc("GET /internal/device-tokens/{userId}", notifHandler.InternalGetDeviceTokens)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8005"
	}

	logger.Info("starting clay-notification-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
