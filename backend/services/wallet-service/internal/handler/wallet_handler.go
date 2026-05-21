package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/services/wallet-service/internal/service"
)

type WalletHandler struct {
	svc service.WalletService
}

func NewWalletHandler(svc service.WalletService) *WalletHandler {
	return &WalletHandler{svc: svc}
}

func (h *WalletHandler) GetWallet(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid X-User-ID header")
		return
	}

	wallet, err := h.svc.GetBalance(r.Context(), userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get wallet")
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"wallet_id":  wallet.ID,
		"user_id":    wallet.UserID,
		"balance":    wallet.Balance,
		"is_active":  wallet.IsActive,
		"created_at": wallet.CreatedAt,
	})
}

func (h *WalletHandler) TopUp(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "Invalid X-User-ID header")
		return
	}

	var req struct {
		Amount  int64  `json:"amount"`
		Channel string `json:"channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	tx, err := h.svc.TopUp(r.Context(), userID, req.Amount, req.Channel)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to top up")
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"transaction_id": tx.ID,
		"amount":         tx.Amount,
		"channel":        req.Channel,
		"redirect_url":   "https://gateway.example.com/pay/" + tx.ID.String(),
	})
}

func (h *WalletHandler) Debit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID      uuid.UUID `json:"user_id"`
		Amount      int64     `json:"amount"`
		ReferenceID uuid.UUID `json:"reference_id"`
		Description string    `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	tx, err := h.svc.Debit(r.Context(), req.UserID, req.Amount, req.ReferenceID, req.Description)
	if err != nil {
		response.Error(w, http.StatusPaymentRequired, "INSUFFICIENT_BALANCE", err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"tx_id":          tx.ID,
		"balance_after":  tx.BalanceAfter,
		"balance_before": tx.BalanceAfter + tx.Amount,
		"amount":         tx.Amount,
	})
}
