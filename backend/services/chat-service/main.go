package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/zicofarry/clay-app/backend/services/chat-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/chat-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/chat-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
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
		dbName = "chat_db"
	}
	db := mongoClient.Database(dbName)

	chatRepo := repository.NewChatRepository(db)
	chatSvc := service.NewChatService(chatRepo, logger)
	chatHandler := handler.NewChatHandler(chatSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Rooms
	mux.HandleFunc("GET /rooms", chatHandler.ListMyRooms)
	mux.HandleFunc("GET /rooms/{roomId}", chatHandler.GetRoomByID)
	mux.HandleFunc("GET /rooms/by-order/{orderId}", chatHandler.GetRoomByOrderID)

	// Messages
	mux.HandleFunc("GET /rooms/{roomId}/messages", chatHandler.ListMessages)
	mux.HandleFunc("POST /rooms/{roomId}/messages", chatHandler.SendMessage)

	// Read Receipts
	mux.HandleFunc("POST /rooms/{roomId}/read", chatHandler.MarkMessagesAsRead)
	mux.HandleFunc("GET /rooms/{roomId}/unread-count", chatHandler.GetUnreadCount)

	// Internal
	mux.HandleFunc("POST /internal/rooms", chatHandler.InternalCreateRoom)
	mux.HandleFunc("POST /internal/rooms/{roomId}/close", chatHandler.InternalCloseRoom)
	mux.HandleFunc("PATCH /internal/rooms/by-order/{orderId}/assign-driver", chatHandler.InternalAssignDriver)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8008" // According to PORT_MAPPING.md for clay-chat-service
	}

	logger.Info("starting clay-chat-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
