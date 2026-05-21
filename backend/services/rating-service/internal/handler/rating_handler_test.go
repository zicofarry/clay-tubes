//go:build unit

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/rating-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/rating-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func TestSubmitRating_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := uuid.New().String()
	mockSvc := mocks.NewMockRatingServiceInterface(ctrl)
	
	comment := "Good service"
	reqPayload := service.SubmitRatingRequest{
		OrderID: uuid.New().String(),
		Ratings: []service.RatingEntry{
			{
				SubjectType: "driver",
				SubjectID:   uuid.New().String(),
				Score:       5,
				ReviewText:  &comment,
				Tags:        []string{"polite", "safe"},
			},
		},
	}
	
	mockSvc.EXPECT().
		SubmitRating(gomock.Any(), userID, reqPayload).
		Return(&service.RatingSubmitResponse{
			Submitted: []service.SubmittedRatingDTO{
				{
					RatingID:    uuid.New().String(),
					SubjectType: "driver",
					SubjectID:   reqPayload.Ratings[0].SubjectID,
					Score:       5,
				},
			},
		}, nil)

	h := NewRatingHandler(mockSvc)

	body, _ := json.Marshal(reqPayload)
	req := httptest.NewRequest("POST", "/rating", bytes.NewBuffer(body))
	req.Header.Set("X-User-ID", userID)
	w := httptest.NewRecorder()

	h.SubmitRating(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var resp response.SuccessResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestGetOrderRatings_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orderID := uuid.New().String()
	mockSvc := mocks.NewMockRatingServiceInterface(ctrl)
	
	mockSvc.EXPECT().
		GetOrderRatings(gomock.Any(), orderID).
		Return(&service.OrderRatingsResponse{
			OrderID: orderID,
			Ratings: []struct {
				service.RatingDTO
				SubjectType string "json:\"subject_type\""
				SubjectID   string "json:\"subject_id\""
			}{
				{
					RatingDTO: service.RatingDTO{
						RatingID: uuid.New().String(),
						OrderID:  orderID,
						Score:    5,
					},
					SubjectType: "driver",
					SubjectID:   uuid.New().String(),
				},
			},
		}, nil)

	h := NewRatingHandler(mockSvc)

	req := httptest.NewRequest("GET", "/rating/order/"+orderID, nil)
	req.SetPathValue("orderId", orderID)
	w := httptest.NewRecorder()

	h.GetOrderRatings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetDriverScore_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	driverID := uuid.New().String()
	mockSvc := mocks.NewMockRatingServiceInterface(ctrl)
	
	mockSvc.EXPECT().
		GetDriverScore(gomock.Any(), driverID).
		Return(&service.AverageRatingResponse{
			SubjectType:  "driver",
			SubjectID:    driverID,
			AverageScore: 4.85,
			TotalRatings: 120,
		}, nil)

	h := NewRatingHandler(mockSvc)

	req := httptest.NewRequest("GET", "/internal/rating/driver/"+driverID, nil)
	req.SetPathValue("driverId", driverID)
	w := httptest.NewRecorder()

	h.GetDriverScore(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSubmitRating_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockRatingServiceInterface(ctrl)
	h := NewRatingHandler(mockSvc)

	req := httptest.NewRequest("POST", "/rating", bytes.NewBufferString(`{invalid`))
	w := httptest.NewRecorder()

	h.SubmitRating(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
