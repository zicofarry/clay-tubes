//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/push-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/push-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/push-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func TestSendPush_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockPushServiceInterface(ctrl)
	mockSvc.EXPECT().
		SendPush(gomock.Any(), gomock.Any()).
		Return(&service.SendPushResponse{
			MessageID: "msg-123",
			Provider:  "fcm",
		}, nil)

	h := NewPushHandler(mockSvc)

	body := `{"token":"device-token","platform":"android","payload":{"title":"Test","body":"Body"}}`
	req := httptest.NewRequest("POST", "/internal/push/send", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.SendPush(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp response.SuccessResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestSendPush_MissingToken(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockPushServiceInterface(ctrl)
	mockSvc.EXPECT().
		SendPush(gomock.Any(), gomock.Any()).
		Return(nil, service.ErrMissingToken)

	h := NewPushHandler(mockSvc)

	body := `{"token":"","platform":"android","payload":{"title":"Test","body":"Body"}}`
	req := httptest.NewRequest("POST", "/internal/push/send", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.SendPush(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSendPush_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockPushServiceInterface(ctrl)
	// No EXPECT — service should never be called for invalid JSON

	h := NewPushHandler(mockSvc)

	req := httptest.NewRequest("POST", "/internal/push/send", strings.NewReader(`{invalid`))
	w := httptest.NewRecorder()

	h.SendPush(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSendBatchPush_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockPushServiceInterface(ctrl)
	mockSvc.EXPECT().
		SendBatchPush(gomock.Any(), gomock.Any()).
		Return(&service.SendBatchPushResponse{
			Total:        2,
			SuccessCount: 2,
			FailureCount: 0,
			Results: []repository.BatchResult{
				{Token: "t1", Status: "success"},
				{Token: "t2", Status: "success"},
			},
		}, nil)

	h := NewPushHandler(mockSvc)

	body := `{"tokens":[{"token":"t1","platform":"android"},{"token":"t2","platform":"android"}],"payload":{"title":"Batch","body":"Body"}}`
	req := httptest.NewRequest("POST", "/internal/push/send-batch", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.SendBatchPush(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSendBatchPush_EmptyTokens(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockPushServiceInterface(ctrl)
	mockSvc.EXPECT().
		SendBatchPush(gomock.Any(), gomock.Any()).
		Return(nil, service.ErrMissingTokens)

	h := NewPushHandler(mockSvc)

	body := `{"tokens":[],"payload":{"title":"Batch","body":"Body"}}`
	req := httptest.NewRequest("POST", "/internal/push/send-batch", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.SendBatchPush(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSubscribeTopic_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockPushServiceInterface(ctrl)
	mockSvc.EXPECT().
		SubscribeTopic(gomock.Any(), "promo_all", gomock.Any()).
		Return(&service.TopicSubscribeResponse{
			SuccessCount: 2,
			FailureCount: 0,
		}, nil)

	h := NewPushHandler(mockSvc)

	body := `{"tokens":["t1","t2"]}`
	req := httptest.NewRequest("POST", "/internal/push/topics/promo_all/subscribe", strings.NewReader(body))
	req.SetPathValue("topicName", "promo_all")
	w := httptest.NewRecorder()

	h.SubscribeTopic(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSendTopicPush_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockPushServiceInterface(ctrl)
	mockSvc.EXPECT().
		SendTopicPush(gomock.Any(), "driver_bandung", gomock.Any()).
		Return(&service.SendTopicPushResponse{
			MessageID: "topic-msg-123",
		}, nil)

	h := NewPushHandler(mockSvc)

	body := `{"payload":{"title":"Topic","body":"Body"}}`
	req := httptest.NewRequest("POST", "/internal/push/topics/driver_bandung/send", strings.NewReader(body))
	req.SetPathValue("topicName", "driver_bandung")
	w := httptest.NewRecorder()

	h.SendTopicPush(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSendTopicPush_MissingPayload(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockPushServiceInterface(ctrl)
	mockSvc.EXPECT().
		SendTopicPush(gomock.Any(), "driver_bandung", gomock.Any()).
		Return(nil, service.ErrMissingPayload)

	h := NewPushHandler(mockSvc)

	body := `{"payload":{"title":"","body":""}}`
	req := httptest.NewRequest("POST", "/internal/push/topics/driver_bandung/send", strings.NewReader(body))
	req.SetPathValue("topicName", "driver_bandung")
	w := httptest.NewRecorder()

	h.SendTopicPush(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
