package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/zicofarry/clay-app/backend/services/search-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/search-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/search-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// ── Dependencies ─────────────────────────────────────────────────────
	esClient, err := elasticsearch.NewDefaultClient()
	if err != nil {
		logger.Error("failed to create elasticsearch client", "error", err)
		os.Exit(1)
	}
	searchRepo := repository.NewSearchRepository(esClient)
	searchSvc := service.NewSearchService(searchRepo, logger)
	searchHandler := handler.NewSearchHandler(searchSvc)

	// ── Router ───────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response.Health(w, "1.0.0")
	})

	// Search endpoints
	mux.HandleFunc("GET /search/merchants", searchHandler.SearchMerchants)
	mux.HandleFunc("GET /search/menu-items", searchHandler.SearchMenuItems)
	mux.HandleFunc("GET /search/trending", searchHandler.GetTrending)
	mux.HandleFunc("GET /search/suggest", searchHandler.SearchSuggest)
	mux.HandleFunc("GET /search/popular", searchHandler.GetPopular)

	// Internal endpoints
	mux.HandleFunc("POST /internal/index/merchants", searchHandler.IndexMerchant)
	mux.HandleFunc("DELETE /internal/index/merchants/{merchantId}", searchHandler.DeleteMerchantIndex)
	mux.HandleFunc("POST /internal/index/menu-items", searchHandler.IndexMenuItem)
	mux.HandleFunc("DELETE /internal/index/menu-items/{itemId}", searchHandler.DeleteMenuItemIndex)
	mux.HandleFunc("POST /internal/search/reindex", searchHandler.TriggerReindex)

	// ── Middleware Stack ──────────────────────────────────────────────────
	var h http.Handler = mux
	h = middleware.Logger(logger)(h)
	h = middleware.Recovery(logger)(h)
	h = middleware.RequestID(h)
	h = middleware.CORS(middleware.DefaultCORSConfig())(h)

	// ── Start Server ─────────────────────────────────────────────────────
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("starting clay-search-service", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, h); err != nil {
		logger.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}
