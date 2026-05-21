package functional

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/email-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/email-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/email-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/email-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
)

func setupTestServer() *httptest.Server {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6373/0"
	}
	emailRepo := repository.NewEmailRepository(redisUrl)
	emailSvc := service.NewEmailService(emailRepo, logger)
	emailHandler := handler.NewEmailHandler(emailSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/emails/send", emailHandler.SendEmail)
	mux.HandleFunc("GET /internal/emails/{emailId}/status", emailHandler.GetEmailStatus)
	mux.HandleFunc("POST /webhooks/email/delivery", emailHandler.HandleWebhook)
	mux.HandleFunc("GET /templates", emailHandler.GetTemplates)
	mux.HandleFunc("POST /templates", emailHandler.UpsertTemplate)

	var h http.Handler = mux
	h = middleware.RequestID(h)

	return httptest.NewServer(h)
}

func TestEmailServiceIntegration(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	client := &http.Client{}

	// 1. Create Template
	tmplReq := model.UpsertTemplateRequest{
		TemplateId: model.WelcomeTemplate,
		Subject:    "Welcome to Clay!",
		BodyHtml:   "<h1>Welcome</h1>",
	}
	bodyTmpl, _ := json.Marshal(tmplReq)
	req1, _ := http.NewRequest("POST", server.URL+"/templates", bytes.NewBuffer(bodyTmpl))
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}
	if resp1.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %v", resp1.StatusCode)
	}

	// 2. Send Email
	sendReq := model.SendEmailRequest{
		To:         "newuser@clay.id",
		TemplateId: model.WelcomeTemplate,
		Variables:  map[string]interface{}{},
	}
	bodySend, _ := json.Marshal(sendReq)
	req2, _ := http.NewRequest("POST", server.URL+"/internal/emails/send", bytes.NewBuffer(bodySend))
	req2.Header.Set("Idempotency-Key", "idemp-001")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("failed to send email: %v", err)
	}
	if resp2.StatusCode != http.StatusAccepted {
		t.Errorf("expected 202 Accepted, got %v", resp2.StatusCode)
	}

	var sendRes model.SendEmailResponse
	json.NewDecoder(resp2.Body).Decode(&sendRes)

	// 3. Get Status
	req3, _ := http.NewRequest("GET", server.URL+"/internal/emails/"+sendRes.EmailId+"/status", nil)
	resp3, err := client.Do(req3)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %v", resp3.StatusCode)
	}

	var statusRes struct {
		Data model.EmailStatusResponse `json:"data"`
	}
	json.NewDecoder(resp3.Body).Decode(&statusRes)

	if statusRes.Data.Status != "queued" {
		t.Errorf("expected status queued, got %v", statusRes.Data.Status)
	}

	// 4. Webhook
	webhookPayload := map[string]interface{}{
		"sg_message_id": statusRes.Data.ProviderId,
		"event":         "delivered",
	}
	bodyWebhook, _ := json.Marshal(webhookPayload)
	req4, _ := http.NewRequest("POST", server.URL+"/webhooks/email/delivery", bytes.NewBuffer(bodyWebhook))
	resp4, err := client.Do(req4)
	if err != nil {
		t.Fatalf("failed to send webhook: %v", err)
	}
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %v", resp4.StatusCode)
	}

	// 5. Check Status Again
	req5, _ := http.NewRequest("GET", server.URL+"/internal/emails/"+sendRes.EmailId+"/status", nil)
	resp5, _ := client.Do(req5)
	var finalStatusRes struct {
		Data model.EmailStatusResponse `json:"data"`
	}
	json.NewDecoder(resp5.Body).Decode(&finalStatusRes)

	if finalStatusRes.Data.Status != "delivered" {
		t.Errorf("expected status delivered, got %v", finalStatusRes.Data.Status)
	}
}
