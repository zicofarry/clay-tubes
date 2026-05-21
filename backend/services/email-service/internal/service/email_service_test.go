package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/email-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/email-service/mocks"
	"go.uber.org/mock/gomock"
)

func TestEmailService_SendEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEmailRepository(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewEmailService(repo, logger)
	ctx := context.Background()

	t.Run("Success SendEmail", func(t *testing.T) {
		req := model.SendEmailRequest{
			To:         "test@example.com",
			TemplateId: model.OTPLoginTemplate,
			Variables:  map[string]interface{}{"otp_code": "123456"},
		}

		template := &model.EmailTemplate{
			TemplateId: model.OTPLoginTemplate,
			Subject:    "Your OTP Code",
			BodyHtml:   "<p>{{.otp_code}}</p>",
		}

		// Expect GetTemplate and SaveEmailLog to be called
		repo.EXPECT().GetTemplate(ctx, req.TemplateId).Return(template, nil).Times(1)
		repo.EXPECT().SaveEmailLog(ctx, gomock.Any()).Return(nil).Times(1)

		res, err := svc.SendEmail(ctx, "idempotency-123", req)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if res.Status != "queued" {
			t.Errorf("expected status queued, got %s", res.Status)
		}
	})
}

func TestEmailService_UpsertTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEmailRepository(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewEmailService(repo, logger)
	ctx := context.Background()

	t.Run("Success UpsertTemplate", func(t *testing.T) {
		req := model.UpsertTemplateRequest{
			TemplateId: model.OTPLoginTemplate,
			Subject:    "Your OTP Code",
			BodyHtml:   "<p>{{.otp_code}}</p>",
		}

		repo.EXPECT().UpsertTemplate(ctx, gomock.Any()).Return(nil).Times(1)

		res, err := svc.UpsertTemplate(ctx, req)
		if err != nil {
			t.Fatalf("failed to upsert template: %v", err)
		}

		if res.TemplateId != model.OTPLoginTemplate {
			t.Errorf("expected template id %s, got %s", model.OTPLoginTemplate, res.TemplateId)
		}
	})
}

func TestEmailService_GetEmailStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEmailRepository(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewEmailService(repo, logger)
	ctx := context.Background()

	t.Run("Success GetEmailStatus", func(t *testing.T) {
		now := time.Now()
		expectedStatus := &model.EmailStatusResponse{
			EmailId: "email-123",
			Status:  "queued",
			SentAt:  &now,
		}

		repo.EXPECT().GetEmailStatus(ctx, "email-123").Return(expectedStatus, nil).Times(1)

		status, err := svc.GetEmailStatus(ctx, "email-123")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if status.Status != "queued" {
			t.Errorf("expected status queued, got %s", status.Status)
		}
	})
}

func TestEmailService_HandleWebhook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEmailRepository(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewEmailService(repo, logger)
	ctx := context.Background()

	t.Run("Success HandleWebhook", func(t *testing.T) {
		payload := map[string]interface{}{
			"sg_message_id": "sg-123",
			"event":         "delivered",
		}

		// Expectations
		repo.EXPECT().UpdateEmailStatus(ctx, "sg-123", "delivered").Return(nil).Times(1)

		err := svc.HandleWebhook(ctx, payload)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})
}
