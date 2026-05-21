package service

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/audit-log-service/internal/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── Service Error ────────────────────────────────────────────────────────────

type ServiceError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *ServiceError) Error() string {
	return e.Message
}

var (
	ErrLogNotFound   = &ServiceError{http.StatusNotFound, "LOG_NOT_FOUND", "audit log entry not found"}
	ErrInvalidFilter = &ServiceError{http.StatusBadRequest, "INVALID_FILTER", "invalid filter parameters"}
	ErrJobNotFound   = &ServiceError{http.StatusNotFound, "JOB_NOT_FOUND", "export job not found"}
)

// ── Request/Response DTOs ────────────────────────────────────────────────────

type CreateAuditLogRequest struct {
	ActorID      string                 `json:"actor_id"`
	ActorType    string                 `json:"actor_type"`
	Action       string                 `json:"action"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	Changes      map[string]interface{} `json:"changes"`
	IPAddress    string                 `json:"ip_address"`
	UserAgent    string                 `json:"user_agent"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
}

type AuditLogDTO struct {
	ID           string                 `json:"id"`
	ActorID      string                 `json:"actor_id"`
	ActorType    string                 `json:"actor_type"`
	Action       string                 `json:"action"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	Changes      map[string]interface{} `json:"changes"`
	IPAddress    string                 `json:"ip_address"`
	UserAgent    string                 `json:"user_agent"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
}

type AuditLogListResponse struct {
	Data  []AuditLogDTO `json:"data"`
	Total int64         `json:"total"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
}

type SearchLogsParams struct {
	ActorID      string
	ActorType    string
	Action       string
	ResourceType string
	ResourceID   string
	IPAddress    string
	From         time.Time
	To           time.Time
	Query        string
	Page         int
	Limit        int
}

// ── Interface ────────────────────────────────────────────────────────────────

// AuditServiceInterface defines the contract for the audit service layer.
//go:generate mockgen -source=audit_service.go -destination=../../mocks/mock_audit_service.go -package=mocks
type AuditServiceInterface interface {
	CreateLog(ctx context.Context, req *CreateAuditLogRequest) (*AuditLogDTO, error)
	CreateLogBatch(ctx context.Context, reqs []CreateAuditLogRequest) (int, int, error)
	GetLog(ctx context.Context, logID string) (*AuditLogDTO, error)
	SearchLogs(ctx context.Context, params SearchLogsParams) (*AuditLogListResponse, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

type AuditService struct {
	repo   repository.AuditRepositoryInterface
	logger *slog.Logger
}

func NewAuditService(repo repository.AuditRepositoryInterface, logger *slog.Logger) *AuditService {
	return &AuditService{repo: repo, logger: logger}
}

func (s *AuditService) CreateLog(ctx context.Context, req *CreateAuditLogRequest) (*AuditLogDTO, error) {
	// Generate a deterministic or random EventID for deduplication
	eventID := uuid.New().String()

	// Convert Changes back to OldValue / NewValue if needed, or just store as NewValue for simplicity
	// In a real app, logic would map `changes` -> `old_value` and `new_value`.
	// For now, we assume `changes` is stored in NewValue or we structure it appropriately.

	log := &repository.AuditLog{
		EventID:      eventID,
		Service:      "unknown", // could extract from metadata
		Action:       req.Action,
		ActorID:      req.ActorID,
		ActorType:    req.ActorType,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		NewValue:     req.Changes,
		IPAddress:    req.IPAddress,
		Metadata:     req.Metadata,
		CreatedAt:    req.CreatedAt,
	}
	
	if log.Metadata == nil {
		log.Metadata = make(map[string]interface{})
	}
	if req.UserAgent != "" {
		log.Metadata["user_agent"] = req.UserAgent
	}

	if err := s.repo.Insert(ctx, log); err != nil {
		return nil, err
	}

	return s.toDTO(log), nil
}

func (s *AuditService) CreateLogBatch(ctx context.Context, reqs []CreateAuditLogRequest) (int, int, error) {
	var docs []interface{}
	for _, req := range reqs {
		docs = append(docs, &repository.AuditLog{
			EventID:      uuid.New().String(),
			Action:       req.Action,
			ActorID:      req.ActorID,
			ActorType:    req.ActorType,
			ResourceType: req.ResourceType,
			ResourceID:   req.ResourceID,
			NewValue:     req.Changes,
			IPAddress:    req.IPAddress,
			CreatedAt:    req.CreatedAt,
		})
	}
	
	inserted, err := s.repo.InsertBatch(ctx, docs)
	if err != nil {
		return inserted, len(reqs) - inserted, err
	}
	
	return inserted, len(reqs) - inserted, nil
}

func (s *AuditService) GetLog(ctx context.Context, logID string) (*AuditLogDTO, error) {
	oid, err := primitive.ObjectIDFromHex(logID)
	if err != nil {
		return nil, ErrLogNotFound
	}

	log, err := s.repo.FindByID(ctx, oid)
	if err != nil {
		return nil, ErrLogNotFound
	}

	return s.toDTO(log), nil
}

func (s *AuditService) SearchLogs(ctx context.Context, params SearchLogsParams) (*AuditLogListResponse, error) {
	filter := bson.M{}

	if params.ActorID != "" {
		filter["actor_id"] = params.ActorID
	}
	if params.ActorType != "" {
		filter["actor_type"] = params.ActorType
	}
	if params.Action != "" {
		filter["action"] = params.Action
	}
	if params.ResourceType != "" {
		filter["resource_type"] = params.ResourceType
	}
	if params.ResourceID != "" {
		filter["resource_id"] = params.ResourceID
	}
	if params.IPAddress != "" {
		filter["ip_address"] = params.IPAddress
	}

	if !params.From.IsZero() || !params.To.IsZero() {
		dateFilter := bson.M{}
		if !params.From.IsZero() {
			dateFilter["$gte"] = params.From
		}
		if !params.To.IsZero() {
			dateFilter["$lte"] = params.To
		}
		filter["created_at"] = dateFilter
	}

	// Calculate skip
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 {
		params.Limit = 50
	}
	skip := int64((params.Page - 1) * params.Limit)

	logs, total, err := s.repo.Search(ctx, filter, skip, int64(params.Limit))
	if err != nil {
		return nil, err
	}

	dtos := make([]AuditLogDTO, 0, len(logs))
	for _, l := range logs {
		dtos = append(dtos, *s.toDTO(l))
	}

	return &AuditLogListResponse{
		Data:  dtos,
		Total: total,
		Page:  params.Page,
		Limit: params.Limit,
	}, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func (s *AuditService) toDTO(log *repository.AuditLog) *AuditLogDTO {
	userAgent, _ := log.Metadata["user_agent"].(string)

	return &AuditLogDTO{
		ID:           log.ID.Hex(),
		ActorID:      log.ActorID,
		ActorType:    log.ActorType,
		Action:       log.Action,
		ResourceType: log.ResourceType,
		ResourceID:   log.ResourceID,
		Changes:      log.NewValue, // Simplified mapping for `changes`
		IPAddress:    log.IPAddress,
		UserAgent:    userAgent,
		Metadata:     log.Metadata,
		CreatedAt:    log.CreatedAt,
	}
}
