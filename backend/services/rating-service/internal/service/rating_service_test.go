//go:build unit

package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/rating-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/rating-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func TestRatingService_GetAverageRating(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockRatingRepositoryInterface(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewRatingService(mockRepo, logger)

	ctx := context.Background()

	t.Run("Success Get Driver Rating", func(t *testing.T) {
		driverID := uuid.New()
		
		mockAgg := &repository.DriverScoreAggregate{
			DriverID:     driverID,
			AvgScore:     4.85,
			TotalRatings: 100,
			UpdatedAt:    time.Now(),
		}

		mockRepo.EXPECT().GetDriverAggregate(ctx, driverID).Return(mockAgg, nil)

		res, err := svc.GetAverageRating(ctx, "driver", driverID.String())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if res == nil || res.AverageScore != 4.85 {
			t.Errorf("expected average 4.85, got %v", res)
		}
	})
}
