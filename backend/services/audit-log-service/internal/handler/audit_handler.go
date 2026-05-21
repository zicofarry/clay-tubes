package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/zicofarry/clay-app/backend/services/audit-log-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

type AuditHandler struct {
	svc service.AuditServiceInterface
}

func NewAuditHandler(svc service.AuditServiceInterface) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// ── Admin Endpoints ─────────────────────────────────────────────────────────

func (h *AuditHandler) SearchLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	
	params := service.SearchLogsParams{
		ActorID:      query.Get("actor_id"),
		ActorType:    query.Get("actor_type"),
		Action:       query.Get("action"),
		ResourceType: query.Get("resource_type"),
		ResourceID:   query.Get("resource_id"),
		IPAddress:    query.Get("ip_address"),
		Query:        query.Get("q"),
	}

	if from := query.Get("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			params.From = t
		}
	}
	if to := query.Get("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			params.To = t
		}
	}

	if page, err := strconv.Atoi(query.Get("page")); err == nil {
		params.Page = page
	}
	if limit, err := strconv.Atoi(query.Get("limit")); err == nil {
		params.Limit = limit
	}

	res, err := h.svc.SearchLogs(r.Context(), params)
	if err != nil {
		// Proper error handling can map service errors to HTTP status codes
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, res)
}

func (h *AuditHandler) GetLog(w http.ResponseWriter, r *http.Request) {
	logID := r.PathValue("logId")
	
	log, err := h.svc.GetLog(r.Context(), logID)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
		} else {
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": log,
	})
}

// ── Internal Endpoints ──────────────────────────────────────────────────────

func (h *AuditHandler) CreateLog(w http.ResponseWriter, r *http.Request) {
	var req service.CreateAuditLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	log, err := h.svc.CreateLog(r.Context(), &req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"data": log,
	})
}

func (h *AuditHandler) CreateLogBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Entries []service.CreateAuditLogRequest `json:"entries"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid request body")
		return
	}

	if len(body.Entries) == 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "entries array is empty")
		return
	}
	if len(body.Entries) > 100 {
		response.Error(w, http.StatusBadRequest, "INVALID_PAYLOAD", "max 100 entries per batch")
		return
	}

	inserted, skipped, err := h.svc.CreateLogBatch(r.Context(), body.Entries)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, map[string]interface{}{
		"inserted":           inserted,
		"duplicates_skipped": skipped,
	})
}
