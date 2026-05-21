package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/zicofarry/clay-app/backend/services/user-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/user-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/user-service/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/database"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	_ "github.com/lib/pq"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	// Initialize PostgreSQL
	pgConfig := database.DefaultPostgresConfig()
	pgConfig.Host = os.Getenv("DB_HOST") // Will be set by docker-compose
	if pgConfig.Host == "" {
		pgConfig.Host = "localhost"
	}
	db, err := database.NewPostgresDB(pgConfig)
	if err != nil {
		logger.Error("failed to connect to postgres", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	// Initialize Redis
	rdbConfig := database.DefaultRedisConfig()
	rdbConfig.Host = os.Getenv("REDIS_HOST")
	if rdbConfig.Host == "" {
		rdbConfig.Host = "localhost"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     rdbConfig.Addr(),
		Password: rdbConfig.Password,
		DB:       rdbConfig.DB,
	})

	userRepo := repository.NewUserRepository(db, rdb)
	userSvc := service.NewUserService(userRepo, logger)
	userHandler := handler.NewUserHandler(userSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Profile
	mux.HandleFunc("GET /users/me", userHandler.GetMyProfile)
	mux.HandleFunc("POST /users/me", userHandler.CreateProfile)
	mux.HandleFunc("PUT /users/me", userHandler.UpdateProfile)
	mux.HandleFunc("GET /users/{userId}", userHandler.GetProfileByUserId)
	mux.HandleFunc("PUT /users/me/avatar", userHandler.UploadAvatar)
	mux.HandleFunc("POST /users/me/referral/apply", userHandler.ApplyReferralCode)

	// Address
	mux.HandleFunc("GET /addresses", userHandler.ListAddresses)
	mux.HandleFunc("POST /addresses", userHandler.CreateAddress)
	mux.HandleFunc("PUT /addresses/{addressId}", userHandler.UpdateAddress)
	mux.HandleFunc("DELETE /addresses/{addressId}", userHandler.DeleteAddress)
	mux.HandleFunc("PUT /addresses/{addressId}/default", userHandler.SetDefaultAddress)

	// Driver Profile
	mux.HandleFunc("GET /drivers/me", userHandler.GetDriverProfile)
	mux.HandleFunc("PUT /drivers/me", userHandler.UpdateDriverProfile)
	mux.HandleFunc("POST /drivers/register", userHandler.CreateDriverProfile)
	mux.HandleFunc("GET /drivers/{driverId}", userHandler.GetDriverProfileById)
	mux.HandleFunc("PUT /drivers/{driverId}/status", userHandler.ToggleDriverOnline)

	// Driver Documents
	mux.HandleFunc("GET /drivers/me/documents", userHandler.ListDriverDocuments)
	mux.HandleFunc("POST /drivers/me/documents", userHandler.UploadDocument)
	mux.HandleFunc("GET /drivers/me/documents/{documentId}", userHandler.GetDocument)
	mux.HandleFunc("DELETE /drivers/me/documents/{documentId}", userHandler.DeleteDocument)
	
	// Admin Documents (Internal)
	mux.HandleFunc("GET /drivers/{driverId}/verification", userHandler.GetDriverVerificationStatus)
	mux.HandleFunc("PUT /admin/documents/{documentId}/verify", userHandler.VerifyDocument)

	// Settings
	mux.HandleFunc("GET /settings", userHandler.GetSettings)
	mux.HandleFunc("PUT /settings", userHandler.UpdateSettings)

	// Internal
	mux.HandleFunc("POST /internal/users/lookup-by-phone", userHandler.LookupUserByPhone)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "3002"
	}

	logger.Info("starting clay-user-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
