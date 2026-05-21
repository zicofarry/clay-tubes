// Package service implements the business logic for the Notification Service.
package service

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/notification-service/internal/repository"
)

// ── Service Error ────────────────────────────────────────────────────────────

type ServiceError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *ServiceError) Error() string { return e.Message }

var (
	ErrTokenNotFound        = &ServiceError{http.StatusNotFound, "TOKEN_NOT_FOUND", "device token not found"}
	ErrTemplateNotFound     = &ServiceError{http.StatusNotFound, "TEMPLATE_NOT_FOUND", "notification template not found"}
	ErrTemplateConflict     = &ServiceError{http.StatusConflict, "TEMPLATE_CONFLICT", "template for this event_type and channel already exists"}
	ErrNotificationNotFound = &ServiceError{http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "notification not found"}
	ErrChannelDisabled      = &ServiceError{http.StatusUnprocessableEntity, "CHANNEL_DISABLED", "user has disabled this notification channel"}
)

// ── Request/Response DTOs ────────────────────────────────────────────────────

type RegisterDeviceTokenRequest struct {
	Token      string `json:"token"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version"`
}

type UpdatePreferenceRequest struct {
	PushEnabled     *bool   `json:"push_enabled"`
	EmailEnabled    *bool   `json:"email_enabled"`
	SMSEnabled      *bool   `json:"sms_enabled"`
	QuietHoursStart *string `json:"quiet_hours_start"`
	QuietHoursEnd   *string `json:"quiet_hours_end"`
}

type CreateTemplateRequest struct {
	EventType     string  `json:"event_type"`
	Channel       string  `json:"channel"`
	TitleTemplate *string `json:"title_template"`
	BodyTemplate  string  `json:"body_template"`
}

type UpdateTemplateRequest struct {
	TitleTemplate *string `json:"title_template"`
	BodyTemplate  *string `json:"body_template"`
	IsActive      *bool   `json:"is_active"`
}

type InternalSendRequest struct {
	RecipientID   string            `json:"recipient_id"`
	EventType     string            `json:"event_type"`
	Channel       string            `json:"channel"`
	Priority      string            `json:"priority"`
	Payload       map[string]interface{} `json:"payload"`
	OverrideTitle *string           `json:"override_title"`
	OverrideBody  *string           `json:"override_body"`
}

type InternalSendResponse struct {
	NotificationID string `json:"notification_id"`
	RecipientID    string `json:"recipient_id"`
	Channel        string `json:"channel"`
	DeliveryStatus string `json:"delivery_status"`
	Reason         *string `json:"reason"`
}

type PreviewTemplateRequest struct {
	SampleData map[string]interface{} `json:"sample_data"`
}

type PreviewTemplateResponse struct {
	RenderedTitle string `json:"rendered_title"`
	RenderedBody  string `json:"rendered_body"`
}

type InternalSendBatchRequest struct {
	Notifications []InternalSendRequest `json:"notifications"`
}

type InternalSendBatchResponse struct {
	Total   int                    `json:"total"`
	Sent    int                    `json:"sent"`
	Failed  int                    `json:"failed"`
	Results []InternalSendResponse `json:"results"`
}

// ── Interface ────────────────────────────────────────────────────────────────

//go:generate mockgen -source=notification_service.go -destination=../../mocks/mock_notification_service.go -package=mocks
type NotificationServiceInterface interface {
	RegisterDeviceToken(ctx context.Context, userID string, req *RegisterDeviceTokenRequest) (*repository.DeviceToken, error)
	ListDeviceTokens(ctx context.Context, userID string, activeOnly bool) ([]repository.DeviceToken, error)
	DeactivateDeviceToken(ctx context.Context, userID, tokenID string) error
	GetPreferences(ctx context.Context, userID string) (*repository.NotificationPreference, error)
	UpdatePreferences(ctx context.Context, userID string, req *UpdatePreferenceRequest) (*repository.NotificationPreference, error)
	ListNotifications(ctx context.Context, userID string, page, limit int) ([]repository.NotificationLog, int, error)
	GetNotification(ctx context.Context, userID, notificationID string) (*repository.NotificationLog, error)
	CreateTemplate(ctx context.Context, req *CreateTemplateRequest) (*repository.NotificationTemplate, error)
	GetTemplate(ctx context.Context, templateID string) (*repository.NotificationTemplate, error)
	ListTemplates(ctx context.Context) ([]repository.NotificationTemplate, error)
	UpdateTemplate(ctx context.Context, templateID string, req *UpdateTemplateRequest) (*repository.NotificationTemplate, error)
	DeleteTemplate(ctx context.Context, templateID string) error
	PreviewTemplate(ctx context.Context, templateID string, req *PreviewTemplateRequest) (*PreviewTemplateResponse, error)
	SendNotification(ctx context.Context, req *InternalSendRequest) (*InternalSendResponse, error)
	SendBatchNotification(ctx context.Context, req *InternalSendBatchRequest) (*InternalSendBatchResponse, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

type NotificationService struct {
	repo   repository.NotificationRepositoryInterface
	logger *slog.Logger
}

func NewNotificationService(repo repository.NotificationRepositoryInterface, logger *slog.Logger) *NotificationService {
	return &NotificationService{repo: repo, logger: logger}
}

func (s *NotificationService) RegisterDeviceToken(ctx context.Context, userID string, req *RegisterDeviceTokenRequest) (*repository.DeviceToken, error) {
	token := &repository.DeviceToken{
		UserID:     userID,
		Token:      req.Token,
		Platform:   req.Platform,
		AppVersion: req.AppVersion,
	}
	created, err := s.repo.CreateDeviceToken(ctx, token)
	if err != nil {
		return nil, err
	}
	s.logger.Info("device token registered", slog.String("user_id", userID), slog.String("platform", req.Platform))
	return created, nil
}

func (s *NotificationService) ListDeviceTokens(ctx context.Context, userID string, activeOnly bool) ([]repository.DeviceToken, error) {
	return s.repo.FindDeviceTokensByUserID(ctx, userID, activeOnly)
}

func (s *NotificationService) DeactivateDeviceToken(ctx context.Context, userID, tokenID string) error {
	err := s.repo.DeactivateDeviceToken(ctx, tokenID, userID)
	if err == sql.ErrNoRows {
		return ErrTokenNotFound
	}
	if err != nil {
		return err
	}
	s.logger.Info("device token deactivated", slog.String("user_id", userID), slog.String("token_id", tokenID))
	return nil
}

func (s *NotificationService) GetPreferences(ctx context.Context, userID string) (*repository.NotificationPreference, error) {
	pref, err := s.repo.GetPreferences(ctx, userID)
	if err == sql.ErrNoRows {
		// Return defaults
		return &repository.NotificationPreference{
			UserID:       userID,
			PushEnabled:  true,
			EmailEnabled: true,
			SMSEnabled:   true,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return pref, nil
}

func (s *NotificationService) UpdatePreferences(ctx context.Context, userID string, req *UpdatePreferenceRequest) (*repository.NotificationPreference, error) {
	pref := &repository.NotificationPreference{
		UserID:       userID,
		PushEnabled:  true,
		EmailEnabled: true,
		SMSEnabled:   true,
	}

	// Get existing if available
	existing, err := s.repo.GetPreferences(ctx, userID)
	if err == nil {
		pref = existing
	}

	if req.PushEnabled != nil {
		pref.PushEnabled = *req.PushEnabled
	}
	if req.EmailEnabled != nil {
		pref.EmailEnabled = *req.EmailEnabled
	}
	if req.SMSEnabled != nil {
		pref.SMSEnabled = *req.SMSEnabled
	}
	if req.QuietHoursStart != nil {
		pref.QuietHoursStart = req.QuietHoursStart
	}
	if req.QuietHoursEnd != nil {
		pref.QuietHoursEnd = req.QuietHoursEnd
	}

	updated, err := s.repo.UpsertPreferences(ctx, pref)
	if err != nil {
		return nil, err
	}
	s.logger.Info("preferences updated", slog.String("user_id", userID))
	return updated, nil
}

func (s *NotificationService) ListNotifications(ctx context.Context, userID string, page, limit int) ([]repository.NotificationLog, int, error) {
	return s.repo.FindNotificationsByUserID(ctx, userID, page, limit)
}

func (s *NotificationService) GetNotification(ctx context.Context, userID, notificationID string) (*repository.NotificationLog, error) {
	notif, err := s.repo.FindNotificationByID(ctx, notificationID, userID)
	if err == sql.ErrNoRows {
		return nil, ErrNotificationNotFound
	}
	if err != nil {
		return nil, err
	}
	return notif, nil
}

func (s *NotificationService) CreateTemplate(ctx context.Context, req *CreateTemplateRequest) (*repository.NotificationTemplate, error) {
	// Check conflict
	_, err := s.repo.FindTemplateByEventAndChannel(ctx, req.EventType, req.Channel)
	if err == nil {
		return nil, ErrTemplateConflict
	}

	tmpl := &repository.NotificationTemplate{
		EventType:     req.EventType,
		Channel:       req.Channel,
		TitleTemplate: req.TitleTemplate,
		BodyTemplate:  req.BodyTemplate,
	}
	created, err := s.repo.CreateTemplate(ctx, tmpl)
	if err != nil {
		return nil, err
	}
	s.logger.Info("template created", slog.String("event_type", req.EventType), slog.String("channel", req.Channel))
	return created, nil
}

func (s *NotificationService) GetTemplate(ctx context.Context, templateID string) (*repository.NotificationTemplate, error) {
	tmpl, err := s.repo.FindTemplateByID(ctx, templateID)
	if err == sql.ErrNoRows {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

func (s *NotificationService) ListTemplates(ctx context.Context) ([]repository.NotificationTemplate, error) {
	return s.repo.ListTemplates(ctx)
}

func (s *NotificationService) UpdateTemplate(ctx context.Context, templateID string, req *UpdateTemplateRequest) (*repository.NotificationTemplate, error) {
	tmpl, err := s.repo.FindTemplateByID(ctx, templateID)
	if err == sql.ErrNoRows {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	if req.TitleTemplate != nil {
		tmpl.TitleTemplate = req.TitleTemplate
	}
	if req.BodyTemplate != nil {
		tmpl.BodyTemplate = *req.BodyTemplate
	}
	if req.IsActive != nil {
		tmpl.IsActive = *req.IsActive
	}
	return s.repo.UpdateTemplate(ctx, tmpl)
}

func (s *NotificationService) DeleteTemplate(ctx context.Context, templateID string) error {
	err := s.repo.DeactivateTemplate(ctx, templateID)
	if err == sql.ErrNoRows {
		return ErrTemplateNotFound
	}
	if err != nil {
		return err
	}
	s.logger.Info("template deactivated", slog.String("template_id", templateID))
	return nil
}

func (s *NotificationService) PreviewTemplate(ctx context.Context, templateID string, req *PreviewTemplateRequest) (*PreviewTemplateResponse, error) {
	tmpl, err := s.repo.FindTemplateByID(ctx, templateID)
	if err == sql.ErrNoRows {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}

	// Basic placeholder replacement (mock implementation for Handlebars)
	renderedTitle := ""
	if tmpl.TitleTemplate != nil {
		renderedTitle = *tmpl.TitleTemplate
	}
	renderedBody := tmpl.BodyTemplate

	// In real implementation, you would use a template engine like text/template or aymerick/raymond
	// Here we just do a simple mock implementation returning the template itself for demo
	// to satisfy the preview endpoint contract
	return &PreviewTemplateResponse{
		RenderedTitle: renderedTitle,
		RenderedBody:  renderedBody,
	}, nil
}

func (s *NotificationService) SendNotification(ctx context.Context, req *InternalSendRequest) (*InternalSendResponse, error) {
	// Check user preferences
	pref, _ := s.GetPreferences(ctx, req.RecipientID)
	if pref != nil {
		switch req.Channel {
		case "push":
			if !pref.PushEnabled && req.Priority != "critical" {
				return &InternalSendResponse{
					RecipientID: req.RecipientID, Channel: req.Channel,
					DeliveryStatus: "skipped", Reason: strPtr("user disabled push notifications"),
				}, nil
			}
		case "sms":
			if !pref.SMSEnabled && req.Priority != "critical" {
				return &InternalSendResponse{
					RecipientID: req.RecipientID, Channel: req.Channel,
					DeliveryStatus: "skipped", Reason: strPtr("user disabled sms notifications"),
				}, nil
			}
		case "email":
			if !pref.EmailEnabled && req.Priority != "critical" {
				return &InternalSendResponse{
					RecipientID: req.RecipientID, Channel: req.Channel,
					DeliveryStatus: "skipped", Reason: strPtr("user disabled email notifications"),
				}, nil
			}
		}
	}

	// Check template existence to ensure it's valid if no override
	if req.OverrideTitle == nil || req.OverrideBody == nil {
		_, err := s.repo.FindTemplateByEventAndChannel(ctx, req.EventType, req.Channel)
		if err != nil {
			return nil, ErrTemplateNotFound
		}
	}

	// Log the notification
	logEntry := &repository.NotificationLog{
		RecipientID:   req.RecipientID,
		EventType:     req.EventType,
		Channel:       req.Channel,
		Payload:       "{}",
		Status:        "sent",
	}

	created, err := s.repo.CreateNotificationLog(ctx, logEntry)
	if err != nil {
		return nil, err
	}

	s.logger.Info("notification sent", slog.String("recipient_id", req.RecipientID), slog.String("channel", req.Channel))

	return &InternalSendResponse{
		NotificationID: created.ID,
		RecipientID:    req.RecipientID,
		Channel:        req.Channel,
		DeliveryStatus: "sent",
	}, nil
}

func (s *NotificationService) SendBatchNotification(ctx context.Context, req *InternalSendBatchRequest) (*InternalSendBatchResponse, error) {
	var results []InternalSendResponse
	sent := 0
	failed := 0

	for _, notifReq := range req.Notifications {
		res, err := s.SendNotification(ctx, &notifReq)
		if err != nil {
			failed++
			// Fallback if err
			results = append(results, InternalSendResponse{
				RecipientID:    notifReq.RecipientID,
				Channel:        notifReq.Channel,
				DeliveryStatus: "failed",
				Reason:         strPtr(err.Error()),
			})
		} else {
			if res.DeliveryStatus == "sent" {
				sent++
			} else {
				failed++
			}
			results = append(results, *res)
		}
	}

	return &InternalSendBatchResponse{
		Total:   len(req.Notifications),
		Sent:    sent,
		Failed:  failed,
		Results: results,
	}, nil
}

func strPtr(s string) *string { return &s }
