//go:build unit

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/chat-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/chat-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func TestGetRoomByID_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := uuid.New().String()
	roomID := "507f1f77bcf86cd799439011"
	
	mockSvc := mocks.NewMockChatServiceInterface(ctrl)
	
	mockSvc.EXPECT().
		GetRoomByID(gomock.Any(), roomID, userID).
		Return(&service.ChatRoomWithPreviewDTO{
			ChatRoomDTO: service.ChatRoomDTO{
				ID:      roomID,
				OrderID: uuid.New().String(),
				UserID:  userID,
			},
		}, nil)

	h := NewChatHandler(mockSvc)

	req := httptest.NewRequest("GET", "/chat/room/"+roomID, nil)
	req.Header.Set("X-User-ID", userID)
	req.SetPathValue("roomId", roomID)
	w := httptest.NewRecorder()

	h.GetRoomByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestSendMessage_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := uuid.New().String()
	roomID := "507f1f77bcf86cd799439011"
	
	mockSvc := mocks.NewMockChatServiceInterface(ctrl)
	
	payload := map[string]string{
		"content": "Hello Driver!",
		"type":    "text",
	}
	body, _ := json.Marshal(payload)

	mockSvc.EXPECT().
		SendMessage(gomock.Any(), roomID, userID, "user", "Hello Driver!", "text", gomock.Any()).
		Return(&service.MessageDTO{
			ID:       "507f191e810c19729de860ea",
			RoomID:   roomID,
			SenderID: userID,
			Content:  "Hello Driver!",
			Type:     "text",
		}, nil)

	h := NewChatHandler(mockSvc)

	req := httptest.NewRequest("POST", "/chat/room/"+roomID+"/message", bytes.NewBuffer(body))
	req.Header.Set("X-User-ID", userID)
	req.SetPathValue("roomId", roomID)
	w := httptest.NewRecorder()

	h.SendMessage(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var resp response.SuccessResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestSendMessage_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockChatServiceInterface(ctrl)
	h := NewChatHandler(mockSvc)

	req := httptest.NewRequest("POST", "/chat/room/123/message", bytes.NewBufferString(`{invalid`))
	w := httptest.NewRecorder()

	h.SendMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
