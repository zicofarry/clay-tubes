package handler

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/email-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/email-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/email-service/mocks"
	"go.uber.org/mock/gomock"
)

func TestEmailHandler_SendEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockEmailRepository(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := service.NewEmailService(repo, logger)
	emailHandler := NewEmailHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/emails/send", emailHandler.SendEmail)

	t.Run("Success Send Email", func(t *testing.T) {
		sendReq := model.SendEmailRequest{
			To:         "test@example.com",
			TemplateId: model.OTPLoginTemplate,
			Variables:  map[string]interface{}{},
		}
		
		template := &model.EmailTemplate{
			TemplateId: model.OTPLoginTemplate,
			Subject:    "Test",
			BodyHtml:   "Test",
		}
		
		// Expect GetTemplate and SaveEmailLog
		repo.EXPECT().GetTemplate(gomock.Any(), sendReq.TemplateId).Return(template, nil).Times(1)
		repo.EXPECT().SaveEmailLog(gomock.Any(), gomock.Any()).Return(nil).Times(1)

		bodySend, _ := json.Marshal(sendReq)
		reqSend, _ := http.NewRequest("POST", "/internal/emails/send", bytes.NewBuffer(bodySend))
		reqSend.Header.Set("Idempotency-Key", "123")

		rrSend := httptest.NewRecorder()
		mux.ServeHTTP(rrSend, reqSend)

		if rrSend.Code != http.StatusAccepted {
			t.Errorf("expected status 202 Accepted, got %v: %s", rrSend.Code, rrSend.Body.String())
		}
	})
}
