package service

import (
	"context"
	"log/slog"
	"time"
	"errors"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/rating-service/internal/repository"
	"gorm.io/gorm"
)

type ServiceError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *ServiceError) Error() string {
	return e.Message
}

var (
	ErrNotFound      = &ServiceError{"NOT_FOUND", "resource not found", 404}
	ErrInvalid       = &ServiceError{"INVALID_REQUEST", "invalid request parameters", 400}
	ErrConflict      = &ServiceError{"ALREADY_RATED", "rating already submitted", 409}
	ErrWindowExpired = &ServiceError{"RATING_WINDOW_EXPIRED", "rating window expired", 422}
)

// ── DTOs ─────────────────────────────────────────────────────────────────

type RatingEntry struct {
	SubjectType string   `json:"subject_type"`
	SubjectID   string   `json:"subject_id"`
	Score       int16    `json:"score"`
	ReviewText  *string  `json:"review_text,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type SubmitRatingRequest struct {
	OrderID string        `json:"order_id"`
	Ratings []RatingEntry `json:"ratings"`
}

type SubmittedRatingDTO struct {
	RatingID    string `json:"rating_id"`
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
	Score       int16  `json:"score"`
}

type RatingSubmitResponse struct {
	Submitted []SubmittedRatingDTO `json:"submitted"`
}

type RatingDTO struct {
	RatingID   string   `json:"rating_id"`
	OrderID    string   `json:"order_id"`
	RaterID    string   `json:"rater_id"`
	RaterName  string   `json:"rater_name,omitempty"`
	Score      int16    `json:"score"`
	ReviewText *string  `json:"review_text,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

type RatingListResponse struct {
	SubjectType  string      `json:"subject_type"`
	SubjectID    string      `json:"subject_id"`
	AverageScore float64     `json:"average_score"`
	TotalRatings int         `json:"total_ratings"`
	Ratings      []RatingDTO `json:"ratings"`
	Page         int         `json:"page"`
	Limit        int         `json:"limit"`
}

type OrderRatingsResponse struct {
	OrderID string `json:"order_id"`
	Ratings []struct {
		RatingDTO
		SubjectType string `json:"subject_type"`
		SubjectID   string `json:"subject_id"`
	} `json:"ratings"`
}

type AverageRatingResponse struct {
	SubjectType  string  `json:"subject_type"`
	SubjectID    string  `json:"subject_id"`
	AverageScore float64 `json:"average_score"`
	TotalRatings int     `json:"total_ratings"`
	FromCache    bool    `json:"from_cache"`
}

type BatchAverageRequest struct {
	Subjects []struct {
		SubjectType string `json:"subject_type"`
		SubjectID   string `json:"subject_id"`
	} `json:"subjects"`
}

type BatchAverageResponse struct {
	Results map[string]AverageRatingResponse `json:"results"`
}

// ── Interface ────────────────────────────────────────────────────────────

//go:generate mockgen -source=rating_service.go -destination=../../mocks/rating_service_mock.go -package=mocks
type RatingServiceInterface interface {
	SubmitRating(ctx context.Context, userID string, req SubmitRatingRequest) (*RatingSubmitResponse, error)
	GetRatings(ctx context.Context, subjectType, subjectID string, page, limit int) (*RatingListResponse, error)
	GetOrderRatings(ctx context.Context, orderID string) (*OrderRatingsResponse, error)
	GetMyGivenRatings(ctx context.Context, userID string, page, limit int) (*RatingListResponse, error)
	GetMyReceivedRatings(ctx context.Context, userID string, page, limit int) (*RatingListResponse, error)
	
	// Internal
	GetDriverScore(ctx context.Context, driverID string) (*AverageRatingResponse, error)
	GetAverageRating(ctx context.Context, subjectType, subjectID string) (*AverageRatingResponse, error)
	BatchGetAverageRatings(ctx context.Context, req BatchAverageRequest) (*BatchAverageResponse, error)
}

type ratingService struct {
	repo   repository.RatingRepositoryInterface
	logger *slog.Logger
}

func NewRatingService(repo repository.RatingRepositoryInterface, logger *slog.Logger) RatingServiceInterface {
	return &ratingService{repo: repo, logger: logger}
}

// ── Implementation ───────────────────────────────────────────────────────

func (s *ratingService) SubmitRating(ctx context.Context, userID string, req SubmitRatingRequest) (*RatingSubmitResponse, error) {
	uID, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrInvalid
	}
	oID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, ErrInvalid
	}

	var submitted []SubmittedRatingDTO

	// Simplification: Not checking rating window constraint dynamically here
	// Assuming order is within 24h for demonstration

	err = s.repo.RunInTx(ctx, func(txRepo repository.RatingRepositoryInterface) error {
		for _, entry := range req.Ratings {
			subjectID, e := uuid.Parse(entry.SubjectID)
			if e != nil {
				return ErrInvalid
			}

			// Upsert logic for aggregates not implemented fully here
			// A real system would update the aggregate tables via a Kafka consumer or DB Trigger.
			// Here we just insert the rating.

			var revText string
			if entry.ReviewText != nil {
				revText = *entry.ReviewText
			}

			rating := &repository.Rating{
				ID:        uuid.New(),
				OrderID:   oID,
				OrderType: "ride", // default for simplicity
				RaterID:   uID,
				RateeID:   subjectID,
				RateeType: entry.SubjectType,
				Score:     entry.Score,
				Comment:   revText,
				Tags:      pq.StringArray(entry.Tags),
				CreatedAt: time.Now().UTC(),
			}

			if err := txRepo.CreateRating(ctx, rating); err != nil {
				// Handle unique constraint violation (already rated)
				return ErrConflict
			}

			submitted = append(submitted, SubmittedRatingDTO{
				RatingID:    rating.ID.String(),
				SubjectType: rating.RateeType,
				SubjectID:   rating.RateeID.String(),
				Score:       rating.Score,
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &RatingSubmitResponse{Submitted: submitted}, nil
}

func (s *ratingService) GetRatings(ctx context.Context, subjectType, subjectID string, page, limit int) (*RatingListResponse, error) {
	sID, err := uuid.Parse(subjectID)
	if err != nil {
		return nil, ErrInvalid
	}

	offset := (page - 1) * limit
	ratings, count, err := s.repo.GetRatingsBySubject(ctx, subjectType, sID, limit, offset)
	if err != nil {
		return nil, err
	}

	dtos := s.mapToDTOs(ratings)

	// Mock average calculation
	avg := 0.0
	if subjectType == "driver" {
		agg, err := s.repo.GetDriverAggregate(ctx, sID)
		if err == nil {
			avg = agg.AvgScore
		}
	} else if subjectType == "merchant" {
		agg, err := s.repo.GetMerchantAggregate(ctx, sID)
		if err == nil {
			avg = agg.AvgScore
		}
	}

	return &RatingListResponse{
		SubjectType:  subjectType,
		SubjectID:    subjectID,
		AverageScore: avg,
		TotalRatings: int(count),
		Ratings:      dtos,
		Page:         page,
		Limit:        limit,
	}, nil
}

func (s *ratingService) GetOrderRatings(ctx context.Context, orderID string) (*OrderRatingsResponse, error) {
	oID, err := uuid.Parse(orderID)
	if err != nil {
		return nil, ErrInvalid
	}

	ratings, err := s.repo.GetRatingsByOrder(ctx, oID)
	if err != nil {
		return nil, err
	}

	if len(ratings) == 0 {
		return nil, ErrNotFound
	}

	res := &OrderRatingsResponse{
		OrderID: orderID,
	}

	for _, r := range ratings {
		var revText *string
		if r.Comment != "" {
			revText = &r.Comment
		}
		res.Ratings = append(res.Ratings, struct {
			RatingDTO
			SubjectType string `json:"subject_type"`
			SubjectID   string `json:"subject_id"`
		}{
			RatingDTO: RatingDTO{
				RatingID:   r.ID.String(),
				OrderID:    r.OrderID.String(),
				RaterID:    r.RaterID.String(),
				Score:      r.Score,
				ReviewText: revText,
				Tags:       r.Tags,
				CreatedAt:  r.CreatedAt.Format(time.RFC3339),
			},
			SubjectType: r.RateeType,
			SubjectID:   r.RateeID.String(),
		})
	}

	return res, nil
}

func (s *ratingService) GetMyGivenRatings(ctx context.Context, userID string, page, limit int) (*RatingListResponse, error) {
	uID, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrInvalid
	}

	offset := (page - 1) * limit
	ratings, count, err := s.repo.GetRatingsByRater(ctx, uID, limit, offset)
	if err != nil {
		return nil, err
	}

	return &RatingListResponse{
		SubjectType:  "user",
		SubjectID:    userID,
		AverageScore: 0,
		TotalRatings: int(count),
		Ratings:      s.mapToDTOs(ratings),
		Page:         page,
		Limit:        limit,
	}, nil
}

func (s *ratingService) GetMyReceivedRatings(ctx context.Context, userID string, page, limit int) (*RatingListResponse, error) {
	// For simplicity, assuming user is a passenger
	return s.GetRatings(ctx, "passenger", userID, page, limit)
}

func (s *ratingService) GetDriverScore(ctx context.Context, driverID string) (*AverageRatingResponse, error) {
	return s.GetAverageRating(ctx, "driver", driverID)
}

func (s *ratingService) GetAverageRating(ctx context.Context, subjectType, subjectID string) (*AverageRatingResponse, error) {
	sID, err := uuid.Parse(subjectID)
	if err != nil {
		return nil, ErrInvalid
	}

	res := &AverageRatingResponse{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		FromCache:   false,
	}

	if subjectType == "driver" {
		agg, err := s.repo.GetDriverAggregate(ctx, sID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		res.AverageScore = agg.AvgScore
		res.TotalRatings = agg.TotalRatings
	} else if subjectType == "merchant" {
		agg, err := s.repo.GetMerchantAggregate(ctx, sID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		res.AverageScore = agg.AvgScore
		res.TotalRatings = agg.TotalRatings
	} else {
		return nil, ErrNotFound
	}

	return res, nil
}

func (s *ratingService) BatchGetAverageRatings(ctx context.Context, req BatchAverageRequest) (*BatchAverageResponse, error) {
	res := &BatchAverageResponse{
		Results: make(map[string]AverageRatingResponse),
	}

	for _, sub := range req.Subjects {
		avg, err := s.GetAverageRating(ctx, sub.SubjectType, sub.SubjectID)
		if err == nil {
			key := sub.SubjectType + ":" + sub.SubjectID
			res.Results[key] = *avg
		}
	}

	return res, nil
}

func (s *ratingService) mapToDTOs(ratings []repository.Rating) []RatingDTO {
	dtos := make([]RatingDTO, len(ratings))
	for i, r := range ratings {
		var revText *string
		if r.Comment != "" {
			revText = &r.Comment
		}
		dtos[i] = RatingDTO{
			RatingID:   r.ID.String(),
			OrderID:    r.OrderID.String(),
			RaterID:    r.RaterID.String(),
			Score:      r.Score,
			ReviewText: revText,
			Tags:       r.Tags,
			CreatedAt:  r.CreatedAt.Format(time.RFC3339),
		}
	}
	return dtos
}
