package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zicofarry/clay-app/backend/services/promotion-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

type PromotionHandler struct {
	svc service.PromotionServiceInterface
}

func NewPromotionHandler(svc service.PromotionServiceInterface) *PromotionHandler {
	return &PromotionHandler{svc: svc}
}

func getUserID(r *http.Request) string {
	val := r.Header.Get("X-User-ID")
	if val == "" {
		return "dummy-user-id"
	}
	return val
}

// ── Promo ─────────────────────────────────────────────────────────────────

func (h *PromotionHandler) ValidatePromo(w http.ResponseWriter, r *http.Request) {
	var req service.ValidatePromoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	res, err := h.svc.ValidatePromo(r.Context(), req)
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

// ── Voucher ───────────────────────────────────────────────────────────────

func (h *PromotionHandler) ListMyVouchers(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	serviceType := r.URL.Query().Get("service_type")
	status := r.URL.Query().Get("status")

	vouchers, err := h.svc.ListMyVouchers(r.Context(), userID, serviceType, status)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"vouchers": vouchers,
		"total":    len(vouchers),
	})
}

func (h *PromotionHandler) ClaimVoucher(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	voucher, err := h.svc.ClaimVoucher(r.Context(), userID, req.Code)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusCreated, voucher)
}

// ── Admin ─────────────────────────────────────────────────────────────────

func (h *PromotionHandler) AdminListPromos(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}

	promos, count, err := h.svc.ListPromos(r.Context(), status, page, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"promos": promos,
		"total":  count,
		"page":   page,
		"limit":  limit,
	})
}

func (h *PromotionHandler) CreatePromo(w http.ResponseWriter, r *http.Request) {
	var req service.PromoDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	promo, err := h.svc.CreatePromo(r.Context(), req)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusCreated, promo)
}

func (h *PromotionHandler) UpdatePromo(w http.ResponseWriter, r *http.Request) {
	promoID := r.PathValue("promoId")
	// Simplified update request parsing for demonstration
	var req struct {
		IsActive *bool `json:"is_active"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	promo, err := h.svc.UpdatePromo(r.Context(), promoID, req.IsActive, nil, nil, nil)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, promo)
}

func (h *PromotionHandler) DeactivatePromo(w http.ResponseWriter, r *http.Request) {
	promoID := r.PathValue("promoId")
	err := h.svc.DeactivatePromo(r.Context(), promoID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]bool{"success": true})
}

// ── Internal ──────────────────────────────────────────────────────────────

func (h *PromotionHandler) ApplyPromo(w http.ResponseWriter, r *http.Request) {
	var req service.ApplyPromoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	res, err := h.svc.ApplyPromo(r.Context(), req)
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

func (h *PromotionHandler) ReleasePromo(w http.ResponseWriter, r *http.Request) {
	var req service.ReleasePromoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	err := h.svc.ReleasePromo(r.Context(), req.PromoCode, req.OrderID, req.UserID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]bool{"success": true})
}
