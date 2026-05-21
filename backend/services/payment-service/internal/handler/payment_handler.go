// Package handler implements HTTP handlers for the Payment Service.
// Each method maps 1:1 to an OpenAPI endpoint.
package handler

import (
	"net/http"
	"strconv"

	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

// PaymentHandler holds references to the payment service layer.
type PaymentHandler struct {
	svc service.PaymentServiceInterface
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(svc service.PaymentServiceInterface) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

// ── Payment Methods ──────────────────────────────────────────────────────────

func (h *PaymentHandler) ListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	result, err := h.svc.ListPaymentMethods(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *PaymentHandler) AddPaymentMethod(w http.ResponseWriter, r *http.Request) {
	var req service.AddPaymentMethodRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	userID := r.Header.Get("X-User-ID")
	result, err := h.svc.AddPaymentMethod(r.Context(), userID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, result)
}

func (h *PaymentHandler) DeletePaymentMethod(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	methodID := r.PathValue("methodId")
	if err := h.svc.DeletePaymentMethod(r.Context(), userID, methodID); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *PaymentHandler) SetDefaultPaymentMethod(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	methodID := r.PathValue("methodId")
	if err := h.svc.SetDefaultPaymentMethod(r.Context(), userID, methodID); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]bool{"success": true})
}

// ── COD Verification ─────────────────────────────────────────────────────────

func (h *PaymentHandler) InitiateCodVerification(w http.ResponseWriter, r *http.Request) {
	var req service.CodVerifyInitiateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	userID := r.Header.Get("X-User-ID")
	result, err := h.svc.InitiateCodVerification(r.Context(), userID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, result)
}

func (h *PaymentHandler) GetCodVerificationStatus(w http.ResponseWriter, r *http.Request) {
	verificationID := r.PathValue("verificationId")
	result, err := h.svc.GetCodVerificationStatus(r.Context(), verificationID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *PaymentHandler) SubmitCodOTP(w http.ResponseWriter, r *http.Request) {
	var req service.CodVerifyOTPRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	verificationID := r.PathValue("verificationId")
	result, err := h.svc.SubmitCodOTP(r.Context(), verificationID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *PaymentHandler) RespondCodVerification(w http.ResponseWriter, r *http.Request) {
	var req service.CodVerifyRespondRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	verificationID := r.PathValue("verificationId")
	result, err := h.svc.RespondCodVerification(r.Context(), verificationID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Transactions ─────────────────────────────────────────────────────────────

func (h *PaymentHandler) GetTransactionHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	txType := r.URL.Query().Get("type")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 50 {
		limit = 20
	}

	result, err := h.svc.GetTransactionHistory(r.Context(), userID, txType, page, limit)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *PaymentHandler) GetTransactionDetail(w http.ResponseWriter, r *http.Request) {
	transactionID := r.PathValue("transactionId")
	result, err := h.svc.GetTransactionDetail(r.Context(), transactionID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Internal: Charge / Refund ────────────────────────────────────────────────

func (h *PaymentHandler) CreateCharge(w http.ResponseWriter, r *http.Request) {
	var req service.ChargeRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.CreateCharge(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	statusCode := http.StatusOK
	if result.Status == "pending" {
		statusCode = http.StatusAccepted
	}
	response.Success(w, statusCode, result)
}

func (h *PaymentHandler) CreateRefund(w http.ResponseWriter, r *http.Request) {
	var req service.RefundRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.CreateRefund(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *PaymentHandler) GetTransactionStatus(w http.ResponseWriter, r *http.Request) {
	transactionID := r.PathValue("transactionId")
	result, err := h.svc.GetTransactionStatus(r.Context(), transactionID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Internal: Hold / Capture / Release ───────────────────────────────────────

func (h *PaymentHandler) HoldPayment(w http.ResponseWriter, r *http.Request) {
	var req service.HoldRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.HoldPayment(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *PaymentHandler) CapturePayment(w http.ResponseWriter, r *http.Request) {
	var req service.CaptureRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.CapturePayment(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *PaymentHandler) ReleasePayment(w http.ResponseWriter, r *http.Request) {
	var req service.ReleaseRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	if err := h.svc.ReleasePayment(r.Context(), &req); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]bool{"success": true})
}

// ── Settlement ───────────────────────────────────────────────────────────────

func (h *PaymentHandler) CreateSettlement(w http.ResponseWriter, r *http.Request) {
	var req service.CreateSettlementRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.CreateSettlement(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, result)
}

// ── Error Mapping ────────────────────────────────────────────────────────────

func handleServiceError(w http.ResponseWriter, err error) {
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
		return
	}
	response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
}
