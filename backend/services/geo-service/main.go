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

	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/broker"
	geocache "github.com/zicofarry/clay-app/backend/services/geo-service/internal/cache"
	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/service"
	sharedKafka "github.com/zicofarry/clay-app/backend/pkg/pkg/kafka"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	geoRepo := repository.NewGeoRepository(nil) // TODO: real PostgreSQL
	geoCache := geocache.NewInMemoryGeoCache()   // TODO: real Redis

	geoSvc := service.NewGeoService(geoRepo, geoCache, logger)
	geoHandler := handler.NewGeoHandler(geoSvc)

	// ── Kafka ─────────────────────────────────────────────────────────────
	kafkaBrokers := strings.Split(envOrDefault("KAFKA_BROKERS", "localhost:9092"), ",")

	var kafkaConsumer sharedKafka.Consumer
	if os.Getenv("KAFKA_DISABLED") == "true" {
		kafkaConsumer = broker.NewLogConsumer(logger)
	} else {
		kafkaConsumer = sharedKafka.NewKafkaConsumer(kafkaBrokers,
			envOrDefault("KAFKA_GROUP", "clay-geo-service"),
			logger,
		)
	}
	defer kafkaConsumer.Close()

	geoConsumer := broker.NewGeoConsumer(geoCache, logger)
	geoConsumer.RegisterHandlers(kafkaConsumer)

	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()
	go func() {
		if err := kafkaConsumer.Start(consumerCtx); err != nil && err != context.Canceled {
			logger.Error("kafka consumer error", slog.Any("error", err))
		}
	}()

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Location
	mux.HandleFunc("PUT /drivers/{driverId}/location", geoHandler.UpdateDriverLocation)
	mux.HandleFunc("GET /drivers/{driverId}/location", geoHandler.GetDriverLocation)
	mux.HandleFunc("GET /drivers/nearby", geoHandler.FindNearbyDrivers)

	// Maps
	mux.HandleFunc("POST /maps/estimate", geoHandler.EstimateRoute)
	mux.HandleFunc("GET /maps/polyline", geoHandler.GetPolyline)
	mux.HandleFunc("GET /maps/routing", geoHandler.GetRouting)
	mux.HandleFunc("POST /maps/snapping", geoHandler.SnapToRoad)
	mux.HandleFunc("GET /maps/traffic", geoHandler.GetTraffic)

	// Geocoding
	mux.HandleFunc("POST /maps/geocode", geoHandler.ForwardGeocode)
	mux.HandleFunc("POST /maps/reverse-geocode", geoHandler.ReverseGeocode)
	mux.HandleFunc("GET /distance", geoHandler.CalculateDistance)

	// Places
	mux.HandleFunc("GET /maps/places/autocomplete", geoHandler.PlacesAutocomplete)
	mux.HandleFunc("GET /maps/places/details", geoHandler.GetPlaceDetail)

	// Geofence
	mux.HandleFunc("POST /geofence/check", geoHandler.CheckGeofence)

	// Internal
	mux.HandleFunc("POST /internal/drivers/locations/batch", geoHandler.BatchGetDriverLocations)
	mux.HandleFunc("GET /internal/maps/eta/{driverId}/{orderId}", geoHandler.GetDriverETA)
	mux.HandleFunc("PUT /internal/maps/eta/{driverId}/{orderId}", geoHandler.UpdateDriverETA)
	mux.HandleFunc("POST /internal/drivers/{driverId}/register", geoHandler.UpdateDriverLocation)
	mux.HandleFunc("POST /internal/drivers/{driverId}/location", geoHandler.UpdateDriverLocation)
	mux.HandleFunc("DELETE /internal/drivers/{driverId}", geoHandler.GetDriverLocation)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Server with graceful shutdown ─────────────────────────────────────
	port := envOrDefault("PORT", "8009")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("starting clay-geo-service", slog.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-quit
	logger.Info("shutting down geo-service...")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
	consumerCancel()
	logger.Info("geo-service stopped")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
