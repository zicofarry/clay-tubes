package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/email-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/email-service/internal/repository"
)

var (
	ErrTemplateNotFound = errors.New("template not found")
	ErrRateLimitExceeded = errors.New("rate limit exceeded for recipient")
)

type EmailService interface {
	SendEmail(ctx context.Context, idempotencyKey string, req model.SendEmailRequest) (*model.SendEmailResponse, error)
	GetEmailStatus(ctx context.Context, emailId string) (*model.EmailStatusResponse, error)
	HandleWebhook(ctx context.Context, payload map[string]interface{}) error
	GetTemplates(ctx context.Context) ([]model.EmailTemplate, error)
	UpsertTemplate(ctx context.Context, req model.UpsertTemplateRequest) (*model.EmailTemplate, error)
}

type emailService struct {
	repo   repository.EmailRepository
	logger *slog.Logger
}

func NewEmailService(repo repository.EmailRepository, logger *slog.Logger) EmailService {
	return &emailService{
		repo:   repo,
		logger: logger,
	}
}

func (s *emailService) SendEmail(ctx context.Context, idempotencyKey string, req model.SendEmailRequest) (*model.SendEmailResponse, error) {
	s.logger.Info("sending email", slog.String("to", req.To), slog.String("template_id", string(req.TemplateId)))

	// 1. Check if template exists
	_, err := s.repo.GetTemplate(ctx, req.TemplateId)
	if err != nil {
		if errors.Is(err, repository.ErrTemplateNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}

	// 2. Rate Limiting logic (Mocked for now)
	// In reality, we'd check redis: email:rate:{recipient_email}

	// 3. Mock Provider Integration
	emailId := uuid.New().String()
	providerId := "sg-" + uuid.New().String()
	now := time.Now()

	status := model.EmailStatusResponse{
		EmailId:    emailId,
		Status:     "queued",
		ProviderId: providerId,
		SentAt:     &now,
	}

	// 4. Save to db
	err = s.repo.SaveEmailLog(ctx, status)
	if err != nil {
		return nil, err
	}

	return &model.SendEmailResponse{
		EmailId:  emailId,
		Status:   "queued",
		Provider: "sendgrid",
	}, nil
}

func (s *emailService) GetEmailStatus(ctx context.Context, emailId string) (*model.EmailStatusResponse, error) {
	return s.repo.GetEmailStatus(ctx, emailId)
}

func (s *emailService) HandleWebhook(ctx context.Context, payload map[string]interface{}) error {
	s.logger.Info("received webhook", slog.Any("payload", payload))

	// Mock webhook processing (e.g. from Sendgrid)
	// Extract provider_id and event
	providerId, okId := payload["sg_message_id"].(string)
	event, okEvent := payload["event"].(string)

	if !okId || !okEvent {
		return errors.New("invalid webhook payload")
	}

	// Map sendgrid event to our status
	status := "sent"
	if event == "delivered" {
		status = "delivered"
	} else if event == "bounce" {
		status = "bounced"
	} else if event == "spamreport" {
		status = "spam"
	}

	return s.repo.UpdateEmailStatus(ctx, providerId, status)
}

func (s *emailService) GetTemplates(ctx context.Context) ([]model.EmailTemplate, error) {
	return s.repo.GetTemplates(ctx)
}

func (s *emailService) UpsertTemplate(ctx context.Context, req model.UpsertTemplateRequest) (*model.EmailTemplate, error) {
	now := time.Now()
	template := model.EmailTemplate{
		TemplateId: req.TemplateId,
		Subject:    req.Subject,
		BodyHtml:   req.BodyHtml,
		UpdatedAt:  &now,
	}

	err := s.repo.UpsertTemplate(ctx, template)
	if err != nil {
		return nil, err
	}

	return &template, nil
}
