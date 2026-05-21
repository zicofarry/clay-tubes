package handler

import (
	"log/slog"
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

// MerchantHandler handles HTTP requests for merchant and menu operations.
type MerchantHandler struct {
	svc    service.MerchantServiceInterface
	logger *slog.Logger
}

// NewMerchantHandler creates a new MerchantHandler.
func NewMerchantHandler(svc service.MerchantServiceInterface, logger *slog.Logger) *MerchantHandler {
	return &MerchantHandler{svc: svc, logger: logger}
}

// ── Merchant ──────────────────────────────────────────────────────────────────

// RegisterMerchant handles POST /merchants.
func (h *MerchantHandler) RegisterMerchant(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	var req model.RegisterMerchantRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	m, err := h.svc.RegisterMerchant(r.Context(), userID, req)
	if err != nil {
		if err == service.ErrMerchantAlreadyExists {
			response.Error(w, http.StatusConflict, "MERCHANT_EXISTS", err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusCreated, m)
}

// GetMyMerchant handles GET /merchants/me.
func (h *MerchantHandler) GetMyMerchant(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	m, err := h.svc.GetMyMerchant(r.Context(), userID)
	if err != nil {
		if err == service.ErrMerchantNotFound {
			response.Error(w, http.StatusNotFound, "MERCHANT_NOT_FOUND", err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusOK, m)
}

// UpdateMyMerchant handles PUT /merchants/me.
func (h *MerchantHandler) UpdateMyMerchant(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.UpdateMerchantRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	m, err := h.svc.UpdateMyMerchant(r.Context(), userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, m)
}

// GetMerchantByID handles GET /merchants/{merchantId}.
func (h *MerchantHandler) GetMerchantByID(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("merchantId")
	m, err := h.svc.GetMerchantByID(r.Context(), merchantID)
	if err != nil {
		if err == service.ErrMerchantNotFound {
			response.Error(w, http.StatusNotFound, "MERCHANT_NOT_FOUND", err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusOK, m)
}

// UpdateMerchantStatus handles PATCH /merchants/{merchantId}/status.
func (h *MerchantHandler) UpdateMerchantStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	var req model.UpdateMerchantStatusRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	m, err := h.svc.UpdateMerchantStatus(r.Context(), merchantID, userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, m)
}

// ── Operating Hours ───────────────────────────────────────────────────────────

// GetOperatingHours handles GET /merchants/{merchantId}/operating-hours.
func (h *MerchantHandler) GetOperatingHours(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("merchantId")
	hours, err := h.svc.GetOperatingHours(r.Context(), merchantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusOK, hours)
}

// UpsertOperatingHours handles PUT /merchants/{merchantId}/operating-hours.
func (h *MerchantHandler) UpsertOperatingHours(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	var req model.UpsertOperatingHoursRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	hours, err := h.svc.UpsertOperatingHours(r.Context(), merchantID, userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, hours)
}

// ── Bank Accounts ─────────────────────────────────────────────────────────────

// ListBankAccounts handles GET /merchants/{merchantId}/bank-accounts.
func (h *MerchantHandler) ListBankAccounts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	accounts, err := h.svc.ListBankAccounts(r.Context(), merchantID, userID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, accounts)
}

// AddBankAccount handles POST /merchants/{merchantId}/bank-accounts.
func (h *MerchantHandler) AddBankAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	var req model.AddBankAccountRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	ba, err := h.svc.AddBankAccount(r.Context(), merchantID, userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, ba)
}

// DeleteBankAccount handles DELETE /merchants/{merchantId}/bank-accounts/{accountId}.
func (h *MerchantHandler) DeleteBankAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	accountID := r.PathValue("accountId")
	if err := h.svc.DeleteBankAccount(r.Context(), merchantID, accountID, userID); err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": "bank account deleted"})
}

// SetPrimaryBankAccount handles PATCH /merchants/{merchantId}/bank-accounts/{accountId}/set-primary.
func (h *MerchantHandler) SetPrimaryBankAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	accountID := r.PathValue("accountId")
	ba, err := h.svc.SetPrimaryBankAccount(r.Context(), merchantID, accountID, userID)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, ba)
}

// ── Menu Categories ───────────────────────────────────────────────────────────

// ListCategories handles GET /merchants/{merchantId}/menu/categories.
func (h *MerchantHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("merchantId")
	cats, err := h.svc.ListCategories(r.Context(), merchantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusOK, cats)
}

// CreateCategory handles POST /merchants/{merchantId}/menu/categories.
func (h *MerchantHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	var req model.CreateMenuCategoryRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	cat, err := h.svc.CreateCategory(r.Context(), merchantID, userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, cat)
}

// UpdateCategory handles PUT /merchants/{merchantId}/menu/categories/{categoryId}.
func (h *MerchantHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	categoryID := r.PathValue("categoryId")
	var req model.UpdateMenuCategoryRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	cat, err := h.svc.UpdateCategory(r.Context(), merchantID, categoryID, userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, cat)
}

// DeleteCategory handles DELETE /merchants/{merchantId}/menu/categories/{categoryId}.
func (h *MerchantHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	categoryID := r.PathValue("categoryId")
	if err := h.svc.DeleteCategory(r.Context(), merchantID, categoryID, userID); err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": "category deleted"})
}

// ReorderCategories handles PATCH /merchants/{merchantId}/menu/categories/reorder.
func (h *MerchantHandler) ReorderCategories(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	var req model.ReorderCategoriesRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	cats, err := h.svc.ReorderCategories(r.Context(), merchantID, userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, cats)
}

// ── Menu Items ────────────────────────────────────────────────────────────────

// ListItems handles GET /merchants/{merchantId}/menu/items.
func (h *MerchantHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("merchantId")
	categoryID := validator.QueryString(r, "category_id", "")
	isAvailStr := r.URL.Query().Get("is_available")

	var isAvail *bool
	if isAvailStr != "" {
		b := isAvailStr == "true"
		isAvail = &b
	}

	items, err := h.svc.ListItems(r.Context(), merchantID, categoryID, isAvail)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusOK, items)
}

// CreateItem handles POST /merchants/{merchantId}/menu/items.
func (h *MerchantHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	var req model.CreateMenuItemRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	item, err := h.svc.CreateItem(r.Context(), merchantID, userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, item)
}

// GetItem handles GET /merchants/{merchantId}/menu/items/{itemId}.
func (h *MerchantHandler) GetItem(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("merchantId")
	itemID := r.PathValue("itemId")
	item, err := h.svc.GetItem(r.Context(), merchantID, itemID)
	if err != nil || item == nil {
		response.Error(w, http.StatusNotFound, "ITEM_NOT_FOUND", "menu item not found")
		return
	}
	response.Success(w, http.StatusOK, item)
}

// UpdateItem handles PUT /merchants/{merchantId}/menu/items/{itemId}.
func (h *MerchantHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	itemID := r.PathValue("itemId")
	var req model.UpdateMenuItemRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	item, err := h.svc.UpdateItem(r.Context(), merchantID, itemID, userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, item)
}

// DeleteItem handles DELETE /merchants/{merchantId}/menu/items/{itemId}.
func (h *MerchantHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	itemID := r.PathValue("itemId")
	if err := h.svc.DeleteItem(r.Context(), merchantID, itemID, userID); err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": "item deleted"})
}

// ToggleAvailability handles PATCH /merchants/{merchantId}/menu/items/{itemId}/availability.
func (h *MerchantHandler) ToggleAvailability(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	merchantID := r.PathValue("merchantId")
	itemID := r.PathValue("itemId")
	var req model.ToggleAvailabilityRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	item, err := h.svc.ToggleAvailability(r.Context(), merchantID, itemID, userID, req)
	if err != nil {
		h.handleError(w, err)
		return
	}
	response.Success(w, http.StatusOK, item)
}

// ── Internal Handlers ─────────────────────────────────────────────────────────

// InternalGetMerchant handles GET /internal/merchants/{merchantId}.
func (h *MerchantHandler) InternalGetMerchant(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("merchantId")
	m, err := h.svc.GetMerchantByID(r.Context(), merchantID)
	if err != nil || m == nil {
		response.Error(w, http.StatusNotFound, "MERCHANT_NOT_FOUND", "merchant not found")
		return
	}
	response.Success(w, http.StatusOK, m)
}

// InternalIsOpen handles GET /internal/merchants/{merchantId}/is-open.
func (h *MerchantHandler) InternalIsOpen(w http.ResponseWriter, r *http.Request) {
	merchantID := r.PathValue("merchantId")
	result, err := h.svc.IsOpen(r.Context(), merchantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusOK, result)
}

// InternalBatchGetItems handles POST /internal/menu-items/batch.
func (h *MerchantHandler) InternalBatchGetItems(w http.ResponseWriter, r *http.Request) {
	var req model.BatchGetMenuItemsRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	items, err := h.svc.BatchGetItems(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusOK, items)
}

// ── Error helper ──────────────────────────────────────────────────────────────

func (h *MerchantHandler) handleError(w http.ResponseWriter, err error) {
	switch err {
	case service.ErrMerchantNotFound, service.ErrItemNotFound, service.ErrCategoryNotFound:
		response.Error(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case service.ErrForbidden:
		response.Error(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	case service.ErrMerchantAlreadyExists:
		response.Error(w, http.StatusConflict, "CONFLICT", err.Error())
	default:
		h.logger.Error("handler error", slog.Any("error", err))
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}
