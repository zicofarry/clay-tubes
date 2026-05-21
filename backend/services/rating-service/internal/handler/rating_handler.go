package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zicofarry/clay-app/backend/services/rating-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

type RatingHandler struct {
	svc service.RatingServiceInterface
}

func NewRatingHandler(svc service.RatingServiceInterface) *RatingHandler {
	return &RatingHandler{svc: svc}
}

func getUserID(r *http.Request) string {
	val := r.Header.Get("X-User-ID")
	if val == "" {
		return "dummy-user-id"
	}
	return val
}

// ── Rating ────────────────────────────────────────────────────────────────

func (h *RatingHandler) SubmitRating(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	var req service.SubmitRatingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	res, err := h.svc.SubmitRating(r.Context(), userID, req)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusCreated, res)
}

func (h *RatingHandler) GetRatings(w http.ResponseWriter, r *http.Request) {
	subjectType := r.PathValue("subjectType")
	subjectID := r.PathValue("subjectId")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	res, err := h.svc.GetRatings(r.Context(), subjectType, subjectID, page, limit)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, res)
}

func (h *RatingHandler) GetOrderRatings(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	res, err := h.svc.GetOrderRatings(r.Context(), orderID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, res)
}

func (h *RatingHandler) GetMyGivenRatings(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	res, err := h.svc.GetMyGivenRatings(r.Context(), userID, page, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, res)
}

func (h *RatingHandler) GetMyReceivedRatings(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	res, err := h.svc.GetMyReceivedRatings(r.Context(), userID, page, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, res)
}

// ── Internal ──────────────────────────────────────────────────────────────

func (h *RatingHandler) GetDriverScore(w http.ResponseWriter, r *http.Request) {
	driverID := r.PathValue("driverId")
	res, err := h.svc.GetDriverScore(r.Context(), driverID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, res)
}

func (h *RatingHandler) GetAverageRating(w http.ResponseWriter, r *http.Request) {
	subjectType := r.PathValue("subjectType")
	subjectID := r.PathValue("subjectId")
	res, err := h.svc.GetAverageRating(r.Context(), subjectType, subjectID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, res)
}

func (h *RatingHandler) BatchGetAverageRatings(w http.ResponseWriter, r *http.Request) {
	var req service.BatchAverageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	res, err := h.svc.BatchGetAverageRatings(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, res)
}
