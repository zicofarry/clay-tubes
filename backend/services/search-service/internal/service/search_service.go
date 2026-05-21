package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/zicofarry/clay-app/backend/services/search-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/search-service/internal/repository"
)

type SearchServiceInterface interface {
	CheckHealth(ctx context.Context) error
	
	// Search
	SearchMerchants(ctx context.Context, query map[string]string) (interface{}, error)
	SearchMenuItems(ctx context.Context, query map[string]string) (interface{}, error)
	GetTrending(ctx context.Context, query map[string]string) (interface{}, error)
	SearchSuggest(ctx context.Context, query map[string]string) (interface{}, error)
	GetPopular(ctx context.Context, query map[string]string) (interface{}, error)
	
	// Internal Index Management
	IndexMerchant(ctx context.Context, payload model.MerchantDocument) error
	DeleteMerchantIndex(ctx context.Context, merchantId string) error
	IndexMenuItem(ctx context.Context, payload model.MenuItemDocument) error
	DeleteMenuItemIndex(ctx context.Context, itemId string) error
	TriggerReindex(ctx context.Context, payload interface{}) error
}

type searchService struct {
	repo   repository.SearchRepositoryInterface
	logger *slog.Logger
}

func NewSearchService(repo repository.SearchRepositoryInterface, logger *slog.Logger) SearchServiceInterface {
	return &searchService{
		repo:   repo,
		logger: logger,
	}
}

func (s *searchService) CheckHealth(ctx context.Context) error {
	return s.repo.Ping(ctx)
}

func (s *searchService) SearchMerchants(ctx context.Context, query map[string]string) (interface{}, error) { return nil, nil }
func (s *searchService) SearchMenuItems(ctx context.Context, query map[string]string) (interface{}, error) { return nil, nil }
func (s *searchService) GetTrending(ctx context.Context, query map[string]string) (interface{}, error) { return nil, nil }
func (s *searchService) SearchSuggest(ctx context.Context, query map[string]string) (interface{}, error) { return nil, nil }
func (s *searchService) GetPopular(ctx context.Context, query map[string]string) (interface{}, error) { return nil, nil }

func (s *searchService) IndexMerchant(ctx context.Context, payload model.MerchantDocument) error { 
	if err := s.repo.IndexMerchant(ctx, payload); err != nil {
		s.logger.Error("failed to index merchant", "error", err, "id", payload.ID)
		return fmt.Errorf("indexing merchant failed: %w", err)
	}
	return nil 
}

func (s *searchService) DeleteMerchantIndex(ctx context.Context, merchantId string) error { return nil }

func (s *searchService) IndexMenuItem(ctx context.Context, payload model.MenuItemDocument) error { 
	if err := s.repo.IndexMenuItem(ctx, payload); err != nil {
		s.logger.Error("failed to index menu item", "error", err, "id", payload.ID)
		return fmt.Errorf("indexing menu item failed: %w", err)
	}
	return nil 
}

func (s *searchService) DeleteMenuItemIndex(ctx context.Context, itemId string) error { return nil }
func (s *searchService) TriggerReindex(ctx context.Context, payload interface{}) error { return nil }

