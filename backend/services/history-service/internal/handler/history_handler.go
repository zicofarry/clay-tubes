package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zicofarry/clay-app/backend/services/history-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

type HistoryHandler struct {
	svc service.HistoryServiceInterface
}

func NewHistoryHandler(svc service.HistoryServiceInterface) *HistoryHandler {
	return &HistoryHandler{svc: svc}
}

// ── Helpers ──────────────────────────────────────────────────────────────

func getUserID(r *http.Request) string {
	val := r.Header.Get("X-User-ID")
	if val == "" {
		return "dummy-user-id" // fallback for local test
	}
	return val
}

// ── Orders ───────────────────────────────────────────────────────────────

func (h *HistoryHandler) ListMyOrderHistory(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	orderType := r.URL.Query().Get("order_type")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	histories, meta, err := h.svc.ListMyOrderHistory(r.Context(), userID, orderType, status, page, limit)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": histories,
		"meta": meta,
	})
}

func (h *HistoryHandler) GetOrderHistoryDetail(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	orderID := r.PathValue("orderId")

	history, err := h.svc.GetOrderHistoryDetail(r.Context(), orderID, userID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": history,
	})
}

func (h *HistoryHandler) ListDriverTripHistory(w http.ResponseWriter, r *http.Request) {
	driverID := getUserID(r)
	orderType := r.URL.Query().Get("order_type")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	histories, meta, err := h.svc.ListDriverTripHistory(r.Context(), driverID, orderType, status, page, limit)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": histories,
		"meta": meta,
	})
}

// ── Activity Feed ────────────────────────────────────────────────────────

func (h *HistoryHandler) GetMyActivityFeed(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	eventType := r.URL.Query().Get("event_type")
	beforeID := r.URL.Query().Get("before")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	feeds, err := h.svc.GetMyActivityFeed(r.Context(), userID, eventType, beforeID, limit)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": feeds,
	})
}

func (h *HistoryHandler) GetActivityFeedEntry(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	feedID := r.PathValue("feedId")

	feed, err := h.svc.GetActivityFeedEntry(r.Context(), feedID, userID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": feed,
	})
}

// ── Internal Sync ────────────────────────────────────────────────────────

func (h *HistoryHandler) InternalSyncOrderHistory(w http.ResponseWriter, r *http.Request) {
	var req service.InternalCreateOrderHistoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	history, err := h.svc.InternalSyncOrderHistory(r.Context(), req)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"data": history,
	})
}

func (h *HistoryHandler) InternalCreateFeedEntry(w http.ResponseWriter, r *http.Request) {
	var req service.InternalCreateFeedEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	feed, err := h.svc.InternalCreateFeedEntry(r.Context(), req)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"data": feed,
	})
}
