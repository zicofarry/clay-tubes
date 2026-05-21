//go:build unit

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/audit-log-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/audit-log-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func TestCreateLog_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockAuditServiceInterface(ctrl)
	
	reqPayload := service.CreateAuditLogRequest{
		ActorID:      uuid.New().String(),
		ActorType:    "user",
		Action:       "CREATE_ORDER",
		ResourceType: "order",
		ResourceID:   uuid.New().String(),
		IPAddress:    "192.168.1.1",
		UserAgent:    "Mozilla/5.0",
	}

	mockSvc.EXPECT().
		CreateLog(gomock.Any(), gomock.Any()).
		Return(&service.AuditLogDTO{
			ID:           "64fcb3e2b9a7c3d4e8f1a2b3",
			ActorID:      reqPayload.ActorID,
			Action:       "CREATE_ORDER",
			ResourceType: "order",
		}, nil)

	h := NewAuditHandler(mockSvc)

	body, _ := json.Marshal(reqPayload)
	req := httptest.NewRequest("POST", "/internal/audit/logs", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.CreateLog(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if _, ok := resp["data"]; !ok {
		t.Error("expected data field in response")
	}
}

func TestGetLog_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	logID := "64fcb3e2b9a7c3d4e8f1a2b3"
	
	mockSvc := mocks.NewMockAuditServiceInterface(ctrl)
	
	mockSvc.EXPECT().
		GetLog(gomock.Any(), logID).
		Return(&service.AuditLogDTO{
			ID:           logID,
			ActorID:      uuid.New().String(),
			Action:       "LOGIN",
			ResourceType: "auth",
			CreatedAt:    time.Now(),
		}, nil)

	h := NewAuditHandler(mockSvc)

	req := httptest.NewRequest("GET", "/admin/audit/logs/"+logID, nil)
	req.SetPathValue("logId", logID)
	w := httptest.NewRecorder()

	h.GetLog(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSearchLogs_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockAuditServiceInterface(ctrl)
	
	mockSvc.EXPECT().
		SearchLogs(gomock.Any(), gomock.Any()).
		Return(&service.AuditLogListResponse{
			Data: []service.AuditLogDTO{
				{
					ID:           "64fcb3e2b9a7c3d4e8f1a2b3",
					ActorID:      uuid.New().String(),
					Action:       "LOGIN",
				},
			},
			Total: 1,
			Page:  1,
			Limit: 20,
		}, nil)

	h := NewAuditHandler(mockSvc)

	req := httptest.NewRequest("GET", "/admin/audit/logs?action=LOGIN&page=1&limit=20", nil)
	w := httptest.NewRecorder()

	h.SearchLogs(w, req)

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

func TestCreateLog_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockAuditServiceInterface(ctrl)
	h := NewAuditHandler(mockSvc)

	req := httptest.NewRequest("POST", "/internal/audit/logs", bytes.NewBufferString(`{invalid`))
	w := httptest.NewRecorder()

	h.CreateLog(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
