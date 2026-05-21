//go:build unit

package service

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/search-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func TestSearchService_CheckHealth(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockSearchRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	svc := NewSearchService(mockRepo, logger)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRepo.EXPECT().Ping(ctx).Return(nil)
		err := svc.CheckHealth(ctx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}
