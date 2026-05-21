package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zicofarry/clay-app/backend/services/chat-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

type ChatHandler struct {
	svc service.ChatServiceInterface
}

func NewChatHandler(svc service.ChatServiceInterface) *ChatHandler {
	return &ChatHandler{svc: svc}
}

// ── Helpers ──────────────────────────────────────────────────────────────

func getUserID(r *http.Request) string {
	// In a real app with clay-shared/pkg/middleware, the JWT middleware
	// sets the user ID in the request context. 
	// For now, we simulate getting it from header for testing purposes if context is empty.
	val := r.Header.Get("X-User-ID")
	if val == "" {
		// fallback to a dummy user id for simplicity if no auth middleware is active
		return "dummy_user_id"
	}
	return val
}

// ── Rooms ────────────────────────────────────────────────────────────────

func (h *ChatHandler) ListMyRooms(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	status := r.URL.Query().Get("status")
	
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	rooms, meta, err := h.svc.ListMyRooms(r.Context(), userID, status, page, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": rooms,
		"meta": meta,
	})
}

func (h *ChatHandler) GetRoomByOrderID(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	orderID := r.PathValue("orderId")

	room, err := h.svc.GetRoomByOrderID(r.Context(), orderID, userID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": room,
	})
}

func (h *ChatHandler) GetRoomByID(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	roomID := r.PathValue("roomId")

	room, err := h.svc.GetRoomByID(r.Context(), roomID, userID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": room,
	})
}

// ── Messages ─────────────────────────────────────────────────────────────

func (h *ChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	roomID := r.PathValue("roomId")
	beforeID := r.URL.Query().Get("before")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	msgs, meta, err := h.svc.ListMessages(r.Context(), roomID, userID, beforeID, limit)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": msgs,
		"meta": meta,
	})
}

func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	roomID := r.PathValue("roomId")

	var req struct {
		Content  string  `json:"content"`
		Type     string  `json:"type"`
		ClientID *string `json:"client_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	// Assuming sender role is user for this example. 
	// Real app would extract role from JWT.
	senderRole := "user"

	msg, err := h.svc.SendMessage(r.Context(), roomID, userID, senderRole, req.Content, req.Type, req.ClientID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"data": msg,
	})
}

// ── Read Receipts ────────────────────────────────────────────────────────

func (h *ChatHandler) MarkMessagesAsRead(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	roomID := r.PathValue("roomId")

	var req struct {
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	count, err := h.svc.MarkMessagesAsRead(r.Context(), roomID, userID, req.MessageID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"marked_count": count,
		},
	})
}

func (h *ChatHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	roomID := r.PathValue("roomId")

	count, err := h.svc.GetUnreadCount(r.Context(), roomID, userID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"room_id":      roomID,
			"unread_count": count,
		},
	})
}

// ── Internal Endpoints ───────────────────────────────────────────────────

func (h *ChatHandler) InternalCreateRoom(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID   string  `json:"order_id"`
		OrderType string  `json:"order_type"`
		UserID    string  `json:"user_id"`
		DriverID  *string `json:"driver_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	room, err := h.svc.InternalCreateRoom(r.Context(), req.OrderID, req.OrderType, req.UserID, req.DriverID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"data": room,
	})
}

func (h *ChatHandler) InternalCloseRoom(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("roomId")

	room, err := h.svc.InternalCloseRoom(r.Context(), roomID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": room,
	})
}

func (h *ChatHandler) InternalAssignDriver(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	
	var req struct {
		DriverID string `json:"driver_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	room, err := h.svc.InternalAssignDriver(r.Context(), orderID, req.DriverID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": room,
	})
}