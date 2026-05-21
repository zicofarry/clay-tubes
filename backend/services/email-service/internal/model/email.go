package model

import "time"

type EmailTemplateId string

const (
	OTPLoginTemplate          EmailTemplateId = "otp_login"
	EmailVerificationTemplate EmailTemplateId = "email_verification"
	PasswordResetTemplate     EmailTemplateId = "password_reset"
	OrderReceiptTemplate      EmailTemplateId = "order_receipt"
	RideReceiptTemplate       EmailTemplateId = "ride_receipt"
	DeliveryReceiptTemplate   EmailTemplateId = "delivery_receipt"
	WelcomeTemplate           EmailTemplateId = "welcome"
	DriverPayoutTemplate      EmailTemplateId = "driver_payout"
)

type SendEmailRequest struct {
	To         string                 `json:"to"`
	ToName     *string                `json:"to_name,omitempty"`
	TemplateId EmailTemplateId        `json:"template_id"`
	Variables  map[string]interface{} `json:"variables"`
	ReplyTo    *string                `json:"reply_to,omitempty"`
}

type SendEmailResponse struct {
	EmailId  string `json:"email_id"`
	Status   string `json:"status"` // queued
	Provider string `json:"provider"`
}

type EmailStatusResponse struct {
	EmailId     string     `json:"email_id"`
	Status      string     `json:"status"` // queued, sent, delivered, bounced, spam, failed
	ProviderId  string     `json:"provider_id"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

type EmailTemplate struct {
	TemplateId EmailTemplateId `json:"template_id"`
	Subject    string          `json:"subject"`
	BodyHtml   string          `json:"body_html"`
	UpdatedAt  *time.Time      `json:"updated_at,omitempty"`
}

type TemplateListResponse struct {
	Templates []EmailTemplate `json:"templates"`
}

type UpsertTemplateRequest struct {
	TemplateId EmailTemplateId `json:"template_id"`
	Subject    string          `json:"subject"`
	BodyHtml   string          `json:"body_html"`
}

type SuccessResponse struct {
	Success bool `json:"success"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
