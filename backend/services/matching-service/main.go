package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/redis/go-redis/v9"

	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/geo"
	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6380"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Use real geo-service HTTP client; fall back to noop if GEO_SERVICE_URL not set.
	geoServiceURL := os.Getenv("GEO_SERVICE_URL")
	var geoClient geo.Client
	if geoServiceURL == "" {
		logger.Warn("GEO_SERVICE_URL not set, using noop geo client")
		geoClient = geo.NewNoopClient()
	} else {
		geoClient = geo.NewHTTPClient(geoServiceURL)
	}

	repo := repository.NewMatchingRepository(rdb)
	svc := service.NewMatchingService(repo, geoClient, logger)
	h := handler.NewMatchingHandler(svc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Dispatcher (Driver-facing)
	mux.HandleFunc("POST /dispatcher/go-online", h.GoOnline)
	mux.HandleFunc("POST /dispatcher/go-offline", h.GoOffline)
	mux.HandleFunc("PUT /dispatcher/location", h.UpdateLocation)
	mux.HandleFunc("POST /dispatcher/heartbeat", h.Heartbeat)
	mux.HandleFunc("POST /dispatcher/respond", h.Respond)
	mux.HandleFunc("PUT /dispatcher/mode", h.SetMode)
	mux.HandleFunc("GET /dispatcher/status", h.GetFullStatus)
	mux.HandleFunc("GET /dispatcher/earnings/today", h.GetTodayEarnings)

	// Internal (Service-to-Service)
	mux.HandleFunc("POST /internal/dispatcher/dispatch", h.StartDispatch)
	mux.HandleFunc("POST /internal/dispatcher/cancel", h.CancelDispatch)
	mux.HandleFunc("GET /internal/dispatcher/nearby-drivers", h.NearbyActiveDrivers)
	mux.HandleFunc("GET /internal/dispatcher/order/{orderId}/status", h.GetSession)
	mux.HandleFunc("GET /internal/dispatcher/zone/{zoneId}/stats", h.GetZoneStats)
	mux.HandleFunc("PUT /internal/drivers/{driverId}/free", h.FreeDriver)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var httpHandler http.Handler = mux
	httpHandler = middleware.Logger(logger)(httpHandler)
	httpHandler = middleware.Recovery(logger)(httpHandler)
	httpHandler = middleware.RequestID(httpHandler)
	httpHandler = middleware.CORS(middleware.DefaultCORSConfig())(httpHandler)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8010"
	}

	logger.Info("starting clay-matching-service",
		slog.String("port", port),
		slog.String("redis", redisAddr),
	)
	if err := http.ListenAndServe(":"+port, httpHandler); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
