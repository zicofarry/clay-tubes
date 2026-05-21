// Package service implements the business logic for the Payment Service.
package service

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/broker"
	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/cache"
	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/repository"
)

// ── Service Error ────────────────────────────────────────────────────────────

type ServiceError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *ServiceError) Error() string { return e.Message }

var (
	ErrInsufficientBalance = &ServiceError{http.StatusPaymentRequired, "INSUFFICIENT_BALANCE", "insufficient balance"}
	ErrInvalidPaymentMethod = &ServiceError{http.StatusUnprocessableEntity, "INVALID_PAYMENT_METHOD", "invalid payment method or amount"}
	ErrPaymentMethodNotFound = &ServiceError{http.StatusNotFound, "PAYMENT_METHOD_NOT_FOUND", "payment method not found"}
	ErrTransactionNotFound  = &ServiceError{http.StatusNotFound, "TRANSACTION_NOT_FOUND", "transaction not found"}
	ErrHoldNotFound         = &ServiceError{http.StatusNotFound, "HOLD_NOT_FOUND", "hold not found or already captured/released"}
	ErrCaptureExceedsHold   = &ServiceError{http.StatusUnprocessableEntity, "CAPTURE_EXCEEDS_HOLD", "capture amount exceeds hold amount"}
	ErrSettlementDuplicate  = &ServiceError{http.StatusConflict, "SETTLEMENT_DUPLICATE", "settlement already exists for this order"}
	ErrCodVerificationNotFound = &ServiceError{http.StatusNotFound, "COD_VERIFICATION_NOT_FOUND", "verification not found or expired"}
	ErrCodVerificationExpired  = &ServiceError{http.StatusGone, "COD_VERIFICATION_EXPIRED", "verification expired"}
	ErrCodOTPInvalid        = &ServiceError{http.StatusBadRequest, "COD_OTP_INVALID", "invalid OTP code"}
	ErrCodOTPMaxAttempts    = &ServiceError{http.StatusTooManyRequests, "COD_OTP_MAX_ATTEMPTS", "max OTP attempts exceeded — re-initiate required"}
	ErrCodNotRecipient      = &ServiceError{http.StatusForbidden, "COD_NOT_RECIPIENT", "only the recipient can respond"}
	ErrRateLimited          = &ServiceError{http.StatusTooManyRequests, "RATE_LIMITED", "too many verification attempts"}
)

// ── Request/Response DTOs ────────────────────────────────────────────────────

type AddPaymentMethodRequest struct {
	Type         string  `json:"type"`
	CardToken    *string `json:"card_token,omitempty"`
	SetAsDefault bool    `json:"set_as_default"`
}

type PaymentMethodResponse struct {
	MethodID    string    `json:"method_id"`
	Type        string    `json:"type"`
	DisplayName string    `json:"display_name"`
	LastFour    *string   `json:"last_four,omitempty"`
	ExpiryMonth *int      `json:"expiry_month,omitempty"`
	ExpiryYear  *int      `json:"expiry_year,omitempty"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
}

type PaymentMethodsListResponse struct {
	Methods         []PaymentMethodResponse `json:"methods"`
	DefaultMethodID *string                 `json:"default_method_id"`
}

type ChargeRequest struct {
	OrderID         string `json:"order_id"`
	UserID          string `json:"user_id"`
	Amount          int    `json:"amount"`
	PaymentMethodID string `json:"payment_method_id"`
	Description     string `json:"description"`
}

type ChargeResponse struct {
	TransactionID      string  `json:"transaction_id"`
	Status             string  `json:"status"`
	GatewayRedirectURL *string `json:"gateway_redirect_url,omitempty"`
}

type RefundRequest struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
	Amount  int    `json:"amount"`
	Reason  string `json:"reason"`
}

type RefundResponse struct {
	RefundID            string     `json:"refund_id"`
	Status              string     `json:"status"`
	EstimatedCompletion *time.Time `json:"estimated_completion,omitempty"`
}

type TransactionHistoryResponse struct {
	Transactions []repository.Transaction `json:"transactions"`
	Total        int                      `json:"total"`
	Page         int                      `json:"page"`
	Limit        int                      `json:"limit"`
}

type TransactionStatusResponse struct {
	TransactionID    string  `json:"transaction_id"`
	Status           string  `json:"status"`
	GatewayReference *string `json:"gateway_reference,omitempty"`
}

type HoldRequest struct {
	OrderID           string  `json:"order_id"`
	UserID            string  `json:"user_id"`
	Amount            int     `json:"amount"`
	PaymentMethodType string  `json:"payment_method_type"`
	PaymentMethodID   *string `json:"payment_method_id,omitempty"`
}

type HoldResponse struct {
	HoldID    string    `json:"hold_id"`
	OrderID   string    `json:"order_id"`
	Amount    int       `json:"amount"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type CaptureRequest struct {
	HoldID        string `json:"hold_id"`
	CaptureAmount int    `json:"capture_amount"`
}

type CaptureResponse struct {
	TransactionID  string `json:"transaction_id"`
	HoldID         string `json:"hold_id"`
	CapturedAmount int    `json:"captured_amount"`
	ReleasedAmount int    `json:"released_amount"`
	Status         string `json:"status"`
}

type ReleaseRequest struct {
	HoldID string `json:"hold_id"`
	Reason string `json:"reason"`
}

type CreateSettlementRequest struct {
	OrderID        string  `json:"order_id"`
	DriverID       string  `json:"driver_id"`
	GrossFare      int     `json:"gross_fare"`
	ServiceType    string  `json:"service_type"`
	PlatformFeePct float64 `json:"platform_fee_pct"`
}

type SettlementResponse struct {
	SettlementID string    `json:"settlement_id"`
	OrderID      string    `json:"order_id"`
	DriverID     string    `json:"driver_id"`
	GrossFare    int       `json:"gross_fare"`
	PlatformFee  int       `json:"platform_fee"`
	DriverPayout int       `json:"driver_payout"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type CodVerifyInitiateRequest struct {
	RecipientPhone string `json:"recipient_phone"`
	OrderType      string `json:"order_type"`
	OrderSummary   string `json:"order_summary"`
}

type CodVerifyInitiateResponse struct {
	VerificationID          string    `json:"verification_id"`
	VerificationType        string    `json:"verification_type"`
	RecipientMaskedPhone    string    `json:"recipient_masked_phone"`
	RecipientHasClayAccount bool      `json:"recipient_has_clay_account"`
	ExpiresAt               time.Time `json:"expires_at"`
}

type CodVerifyOTPRequest struct {
	OTPCode string `json:"otp_code"`
}

type CodVerifyRespondRequest struct {
	Action string `json:"action"` // accept, reject
}

type CodVerifyStatusResponse struct {
	VerificationID       string  `json:"verification_id"`
	Status               string  `json:"status"`
	CodUnlocked          bool    `json:"cod_unlocked"`
	CodToken             *string `json:"cod_token,omitempty"`
	OTPAttemptsRemaining *int    `json:"otp_attempts_remaining,omitempty"`
}

// ── Interface ────────────────────────────────────────────────────────────────

//go:generate mockgen -source=payment_service.go -destination=../../mocks/mock_payment_service.go -package=mocks
type PaymentServiceInterface interface {
	// Payment Methods
	ListPaymentMethods(ctx context.Context, userID string) (*PaymentMethodsListResponse, error)
	AddPaymentMethod(ctx context.Context, userID string, req *AddPaymentMethodRequest) (*PaymentMethodResponse, error)
	DeletePaymentMethod(ctx context.Context, userID, methodID string) error
	SetDefaultPaymentMethod(ctx context.Context, userID, methodID string) error

	// COD Verification
	InitiateCodVerification(ctx context.Context, userID string, req *CodVerifyInitiateRequest) (*CodVerifyInitiateResponse, error)
	GetCodVerificationStatus(ctx context.Context, verificationID string) (*CodVerifyStatusResponse, error)
	SubmitCodOTP(ctx context.Context, verificationID string, req *CodVerifyOTPRequest) (*CodVerifyStatusResponse, error)
	RespondCodVerification(ctx context.Context, verificationID string, req *CodVerifyRespondRequest) (*CodVerifyStatusResponse, error)

	// Transactions
	GetTransactionHistory(ctx context.Context, userID string, txType string, page, limit int) (*TransactionHistoryResponse, error)
	GetTransactionDetail(ctx context.Context, transactionID string) (*repository.Transaction, error)

	// Internal
	CreateCharge(ctx context.Context, req *ChargeRequest) (*ChargeResponse, error)
	CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)
	GetTransactionStatus(ctx context.Context, transactionID string) (*TransactionStatusResponse, error)

	// Hold / Capture / Release
	HoldPayment(ctx context.Context, req *HoldRequest) (*HoldResponse, error)
	CapturePayment(ctx context.Context, req *CaptureRequest) (*CaptureResponse, error)
	ReleasePayment(ctx context.Context, req *ReleaseRequest) error

	// Settlement
	CreateSettlement(ctx context.Context, req *CreateSettlementRequest) (*SettlementResponse, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

type PaymentService struct {
	repo        repository.PaymentRepositoryInterface
	logger      *slog.Logger
	producer    *broker.PaymentProducer
	rateLimiter *cache.RateLimiter
}

func NewPaymentService(repo repository.PaymentRepositoryInterface, logger *slog.Logger, producer *broker.PaymentProducer, rateLimiter *cache.RateLimiter) *PaymentService {
	return &PaymentService{repo: repo, logger: logger, producer: producer, rateLimiter: rateLimiter}
}

func (s *PaymentService) ListPaymentMethods(ctx context.Context, userID string) (*PaymentMethodsListResponse, error) {
	methods, err := s.repo.ListPaymentMethods(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp := &PaymentMethodsListResponse{Methods: make([]PaymentMethodResponse, 0, len(methods))}
	for _, m := range methods {
		pmr := PaymentMethodResponse{
			MethodID: m.ID, Type: m.Type, DisplayName: m.DisplayName,
			LastFour: m.LastFour, ExpiryMonth: m.ExpiryMonth, ExpiryYear: m.ExpiryYear,
			IsDefault: m.IsDefault, CreatedAt: m.CreatedAt,
		}
		resp.Methods = append(resp.Methods, pmr)
		if m.IsDefault {
			id := m.ID
			resp.DefaultMethodID = &id
		}
	}
	return resp, nil
}

func (s *PaymentService) AddPaymentMethod(ctx context.Context, userID string, req *AddPaymentMethodRequest) (*PaymentMethodResponse, error) {
	pm := &repository.PaymentMethod{
		UserID:      userID,
		Type:        req.Type,
		DisplayName: req.Type, // TODO: Generate display name from card token
		IsDefault:   req.SetAsDefault,
		CardToken:   req.CardToken,
	}

	created, err := s.repo.CreatePaymentMethod(ctx, pm)
	if err != nil {
		return nil, err
	}

	s.logger.Info("payment method added", slog.String("user_id", userID), slog.String("type", req.Type))

	return &PaymentMethodResponse{
		MethodID: created.ID, Type: created.Type, DisplayName: created.DisplayName,
		LastFour: created.LastFour, ExpiryMonth: created.ExpiryMonth, ExpiryYear: created.ExpiryYear,
		IsDefault: created.IsDefault, CreatedAt: created.CreatedAt,
	}, nil
}

func (s *PaymentService) DeletePaymentMethod(ctx context.Context, userID, methodID string) error {
	if err := s.repo.DeletePaymentMethod(ctx, userID, methodID); err != nil {
		return ErrPaymentMethodNotFound
	}
	s.logger.Info("payment method deleted", slog.String("user_id", userID), slog.String("method_id", methodID))
	return nil
}

func (s *PaymentService) SetDefaultPaymentMethod(ctx context.Context, userID, methodID string) error {
	if err := s.repo.SetDefaultPaymentMethod(ctx, userID, methodID); err != nil {
		return err
	}
	s.logger.Info("default payment method set", slog.String("user_id", userID), slog.String("method_id", methodID))
	return nil
}

func (s *PaymentService) InitiateCodVerification(ctx context.Context, userID string, req *CodVerifyInitiateRequest) (*CodVerifyInitiateResponse, error) {
	// Check rate limit (max 3 per hour)
	if s.rateLimiter != nil {
		allowed, _, err := s.rateLimiter.Allow(ctx, "cod_verify", userID, 3, 1*time.Hour)
		if err != nil {
			s.logger.Error("rate limit check failed", slog.Any("error", err))
		}
		if !allowed {
			return nil, ErrRateLimited
		}
	}
	// TODO: Call User Service to check if recipient has Clay account

	recipientHasClay := false // placeholder
	vType := "whatsapp_otp"
	if recipientHasClay {
		vType = "push_confirmation"
	}

	cv := &repository.CodVerification{
		UserID:               userID,
		RecipientPhone:       req.RecipientPhone,
		OrderType:            req.OrderType,
		OrderSummary:         req.OrderSummary,
		VerificationType:     vType,
		RecipientHasClayAcct: recipientHasClay,
		Status:               "pending",
		OTPAttemptsRemaining: 3,
		ExpiresAt:            time.Now().Add(10 * time.Minute),
	}

	created, err := s.repo.CreateCodVerification(ctx, cv)
	if err != nil {
		return nil, err
	}

	masked := maskPhone(req.RecipientPhone)
	s.logger.Info("COD verification initiated", slog.String("verification_id", created.ID), slog.String("type", vType))

	return &CodVerifyInitiateResponse{
		VerificationID:          created.ID,
		VerificationType:        vType,
		RecipientMaskedPhone:    masked,
		RecipientHasClayAccount: recipientHasClay,
		ExpiresAt:               created.ExpiresAt,
	}, nil
}

func (s *PaymentService) GetCodVerificationStatus(ctx context.Context, verificationID string) (*CodVerifyStatusResponse, error) {
	cv, err := s.repo.FindCodVerificationByID(ctx, verificationID)
	if err != nil {
		return nil, ErrCodVerificationNotFound
	}
	unlocked := cv.Status == "accepted" || cv.Status == "verified"
	resp := &CodVerifyStatusResponse{
		VerificationID: cv.ID, Status: cv.Status,
		CodUnlocked: unlocked, CodToken: cv.CodToken,
	}
	if cv.VerificationType == "whatsapp_otp" {
		resp.OTPAttemptsRemaining = &cv.OTPAttemptsRemaining
	}
	return resp, nil
}

func (s *PaymentService) SubmitCodOTP(ctx context.Context, verificationID string, req *CodVerifyOTPRequest) (*CodVerifyStatusResponse, error) {
	cv, err := s.repo.FindCodVerificationByID(ctx, verificationID)
	if err != nil {
		return nil, ErrCodVerificationNotFound
	}
	if time.Now().After(cv.ExpiresAt) {
		return nil, ErrCodVerificationExpired
	}
	if cv.OTPAttemptsRemaining <= 0 {
		return nil, ErrCodOTPMaxAttempts
	}

	// TODO: Compare OTP hash
	validOTP := req.OTPCode == "123456" // placeholder
	if !validOTP {
		cv.OTPAttemptsRemaining--
		if cv.OTPAttemptsRemaining <= 0 {
			cv.Status = "failed"
		}
		s.repo.UpdateCodVerification(ctx, cv)
		return nil, ErrCodOTPInvalid
	}

	cv.Status = "verified"
	token := "cod_tok_" + verificationID[:12]
	cv.CodToken = &token
	s.repo.UpdateCodVerification(ctx, cv)

	return &CodVerifyStatusResponse{
		VerificationID: cv.ID, Status: "verified",
		CodUnlocked: true, CodToken: &token,
	}, nil
}

func (s *PaymentService) RespondCodVerification(ctx context.Context, verificationID string, req *CodVerifyRespondRequest) (*CodVerifyStatusResponse, error) {
	cv, err := s.repo.FindCodVerificationByID(ctx, verificationID)
	if err != nil {
		return nil, ErrCodVerificationNotFound
	}
	if time.Now().After(cv.ExpiresAt) {
		return nil, ErrCodVerificationExpired
	}

	cv.Status = req.Action // "accepted" or "rejected"
	if req.Action == "accept" {
		cv.Status = "accepted"
		token := "cod_tok_" + verificationID[:12]
		cv.CodToken = &token
	} else {
		cv.Status = "rejected"
	}
	s.repo.UpdateCodVerification(ctx, cv)

	unlocked := cv.Status == "accepted"
	return &CodVerifyStatusResponse{
		VerificationID: cv.ID, Status: cv.Status,
		CodUnlocked: unlocked, CodToken: cv.CodToken,
	}, nil
}

func (s *PaymentService) GetTransactionHistory(ctx context.Context, userID string, txType string, page, limit int) (*TransactionHistoryResponse, error) {
	transactions, total, err := s.repo.ListTransactions(ctx, userID, txType, page, limit)
	if err != nil {
		return nil, err
	}
	return &TransactionHistoryResponse{
		Transactions: transactions, Total: total, Page: page, Limit: limit,
	}, nil
}

func (s *PaymentService) GetTransactionDetail(ctx context.Context, transactionID string) (*repository.Transaction, error) {
	tx, err := s.repo.FindTransactionByID(ctx, transactionID)
	if err != nil {
		return nil, ErrTransactionNotFound
	}
	return tx, nil
}

func (s *PaymentService) CreateCharge(ctx context.Context, req *ChargeRequest) (*ChargeResponse, error) {
	pm, err := s.repo.FindPaymentMethodByID(ctx, req.PaymentMethodID)
	if err != nil {
		return nil, ErrInvalidPaymentMethod
	}

	status := "completed" // wallet payments are instant
	if pm.Type != "clay_wallet" {
		status = "pending" // gateway payments are async
	}

	tx := &repository.Transaction{
		UserID:            req.UserID,
		OrderID:           &req.OrderID,
		Type:              "charge",
		Status:            status,
		Amount:            req.Amount,
		PaymentMethodType: pm.Type,
		Description:       req.Description,
	}

	created, err := s.repo.CreateTransaction(ctx, tx)
	if err != nil {
		return nil, err
	}

	// Publish payment.charged Kafka event
	if s.producer != nil {
		s.producer.PublishChargeEvent(ctx, &broker.ChargeEvent{
			TransactionID: created.ID, OrderID: req.OrderID, UserID: req.UserID,
			Amount: req.Amount, PaymentMethod: pm.Type, Status: status,
		})
	}
	s.logger.Info("charge created", slog.String("transaction_id", created.ID), slog.String("status", status))

	return &ChargeResponse{TransactionID: created.ID, Status: status}, nil
}

func (s *PaymentService) CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	origTx, err := s.repo.FindTransactionByOrderID(ctx, req.OrderID)
	if err != nil {
		return nil, ErrTransactionNotFound
	}

	status := "processed"
	var estimated *time.Time
	if origTx.PaymentMethodType != "clay_wallet" {
		status = "pending"
		est := time.Now().Add(3 * 24 * time.Hour) // T+3
		estimated = &est
	}

	refund := &repository.Refund{
		TransactionID: origTx.ID, OrderID: req.OrderID, UserID: req.UserID,
		Amount: req.Amount, Reason: req.Reason, Status: status,
		EstimatedCompletion: estimated,
	}

	created, err := s.repo.CreateRefund(ctx, refund)
	if err != nil {
		return nil, err
	}

	if status == "processed" {
		s.repo.UpdateTransactionStatus(ctx, origTx.ID, "refunded")
	}

	// Publish payment.refunded Kafka event
	if s.producer != nil {
		s.producer.PublishRefundEvent(ctx, &broker.RefundEvent{
			RefundID: created.ID, TransactionID: origTx.ID, OrderID: req.OrderID,
			UserID: req.UserID, Amount: req.Amount, Reason: req.Reason, Status: status,
		})
	}
	s.logger.Info("refund created", slog.String("refund_id", created.ID), slog.String("status", status))

	return &RefundResponse{RefundID: created.ID, Status: status, EstimatedCompletion: estimated}, nil
}

func (s *PaymentService) GetTransactionStatus(ctx context.Context, transactionID string) (*TransactionStatusResponse, error) {
	tx, err := s.repo.FindTransactionByID(ctx, transactionID)
	if err != nil {
		return nil, ErrTransactionNotFound
	}
	return &TransactionStatusResponse{
		TransactionID: tx.ID, Status: tx.Status, GatewayReference: tx.GatewayReference,
	}, nil
}

func (s *PaymentService) HoldPayment(ctx context.Context, req *HoldRequest) (*HoldResponse, error) {
	// TODO: Check balance / authorize with gateway
	hold := &repository.Hold{
		OrderID: req.OrderID, UserID: req.UserID, Amount: req.Amount,
		PaymentMethodType: req.PaymentMethodType, PaymentMethodID: req.PaymentMethodID,
		Status: "held", ExpiresAt: time.Now().Add(2 * time.Hour),
	}

	created, err := s.repo.CreateHold(ctx, hold)
	if err != nil {
		return nil, err
	}

	// Publish payment.held Kafka event
	if s.producer != nil {
		s.producer.PublishHoldEvent(ctx, &broker.HoldEvent{
			HoldID: created.ID, OrderID: req.OrderID, UserID: req.UserID,
			Amount: req.Amount, Status: "held",
		})
	}
	s.logger.Info("hold placed", slog.String("hold_id", created.ID), slog.Int("amount", req.Amount))

	return &HoldResponse{
		HoldID: created.ID, OrderID: created.OrderID, Amount: created.Amount,
		Status: "held", ExpiresAt: created.ExpiresAt, CreatedAt: created.CreatedAt,
	}, nil
}

func (s *PaymentService) CapturePayment(ctx context.Context, req *CaptureRequest) (*CaptureResponse, error) {
	hold, err := s.repo.FindHoldByID(ctx, req.HoldID)
	if err != nil || hold.Status != "held" {
		return nil, ErrHoldNotFound
	}
	if req.CaptureAmount > hold.Amount {
		return nil, ErrCaptureExceedsHold
	}

	released := hold.Amount - req.CaptureAmount

	tx := &repository.Transaction{
		UserID: hold.UserID, OrderID: &hold.OrderID,
		Type: "charge", Status: "completed", Amount: req.CaptureAmount,
		PaymentMethodType: hold.PaymentMethodType, Description: "Captured from hold",
	}

	created, err := s.repo.CreateTransaction(ctx, tx)
	if err != nil {
		return nil, err
	}

	s.repo.UpdateHoldStatus(ctx, req.HoldID, "captured")

	// Publish payment.captured Kafka event
	if s.producer != nil {
		s.producer.PublishCaptureEvent(ctx, &broker.HoldEvent{
			HoldID: req.HoldID, OrderID: hold.OrderID, UserID: hold.UserID,
			Amount: req.CaptureAmount, Status: "captured", TransactionID: created.ID,
		})
	}
	s.logger.Info("payment captured", slog.String("hold_id", req.HoldID), slog.Int("amount", req.CaptureAmount))

	return &CaptureResponse{
		TransactionID: created.ID, HoldID: req.HoldID,
		CapturedAmount: req.CaptureAmount, ReleasedAmount: released, Status: "captured",
	}, nil
}

func (s *PaymentService) ReleasePayment(ctx context.Context, req *ReleaseRequest) error {
	hold, err := s.repo.FindHoldByID(ctx, req.HoldID)
	if err != nil || hold.Status != "held" {
		return ErrHoldNotFound
	}

	if err := s.repo.UpdateHoldStatus(ctx, req.HoldID, "released"); err != nil {
		return err
	}

	// Publish payment.released Kafka event
	if s.producer != nil {
		s.producer.PublishReleaseEvent(ctx, &broker.HoldEvent{
			HoldID: req.HoldID, OrderID: hold.OrderID, UserID: hold.UserID,
			Amount: hold.Amount, Status: "released",
		})
	}
	s.logger.Info("hold released", slog.String("hold_id", req.HoldID), slog.String("reason", req.Reason))
	return nil
}

func (s *PaymentService) CreateSettlement(ctx context.Context, req *CreateSettlementRequest) (*SettlementResponse, error) {
	exists, err := s.repo.SettlementExistsByOrderID(ctx, req.OrderID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrSettlementDuplicate
	}

	feePct := req.PlatformFeePct
	if feePct == 0 {
		feePct = 0.20 // default 20%
	}
	platformFee := int(float64(req.GrossFare) * feePct)
	driverPayout := req.GrossFare - platformFee

	settlement := &repository.Settlement{
		OrderID: req.OrderID, DriverID: req.DriverID,
		GrossFare: req.GrossFare, PlatformFee: platformFee, DriverPayout: driverPayout,
		ServiceType: req.ServiceType, Status: "settled",
	}

	created, err := s.repo.CreateSettlement(ctx, settlement)
	if err != nil {
		return nil, err
	}

	// TODO: Credit driver wallet via Wallet Service
	// Publish settlement.created Kafka event
	if s.producer != nil {
		s.producer.PublishSettlementEvent(ctx, &broker.SettlementEvent{
			SettlementID: created.ID, OrderID: req.OrderID, DriverID: req.DriverID,
			GrossFare: req.GrossFare, PlatformFee: platformFee, DriverPayout: driverPayout,
			ServiceType: req.ServiceType,
		})
	}
	s.logger.Info("settlement created", slog.String("settlement_id", created.ID), slog.Int("driver_payout", driverPayout))

	return &SettlementResponse{
		SettlementID: created.ID, OrderID: created.OrderID, DriverID: created.DriverID,
		GrossFare: created.GrossFare, PlatformFee: created.PlatformFee,
		DriverPayout: created.DriverPayout, Status: created.Status, CreatedAt: created.CreatedAt,
	}, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func maskPhone(phone string) string {
	if len(phone) < 8 {
		return phone
	}
	return phone[:5] + "****" + phone[len(phone)-4:]
}
