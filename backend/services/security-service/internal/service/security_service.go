// Package service implements the business logic for the Security Service.
package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/zicofarry/clay-app/backend/services/security-service/internal/repository"
)

// ── Service Error ─────────────────────────────────────────────────────────────

// ServiceError represents a business-logic error with HTTP status mapping.
type ServiceError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *ServiceError) Error() string { return e.Message }

var (
	ErrFlagNotFound    = &ServiceError{http.StatusNotFound, "FLAG_NOT_FOUND", "fraud flag not found"}
	ErrFlagAlreadyResolved = &ServiceError{http.StatusConflict, "FLAG_ALREADY_RESOLVED", "fraud flag is already resolved"}
	ErrBlockNotFound   = &ServiceError{http.StatusNotFound, "BLOCK_NOT_FOUND", "IP block record not found"}
	ErrIPAlreadyBlocked = &ServiceError{http.StatusConflict, "IP_ALREADY_BLOCKED", "IP address is already blocked"}
	ErrValidation      = &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "request body validation failed"}
	ErrForbidden       = &ServiceError{http.StatusForbidden, "FORBIDDEN", "you are not allowed to access this resource"}
)

// ── Auto-flag thresholds ──────────────────────────────────────────────────────

const (
	failedLoginThreshold    = 5
	failedLoginWindowMin    = 15
)

// ── DTOs ──────────────────────────────────────────────────────────────────────

type RecordLoginAttemptRequest struct {
	UserID        string `json:"user_id"`
	IPAddress     string `json:"ip_address"`
	UserAgent     string `json:"user_agent,omitempty"`
	Success       bool   `json:"success"`
	FailureReason string `json:"failure_reason,omitempty"`
}

type RecordLoginAttemptResponse struct {
	Recorded    bool `json:"recorded"`
	AutoFlagged bool `json:"auto_flagged"`
}

type LoginAttemptResponse struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	IPAddress     string    `json:"ip_address"`
	UserAgent     string    `json:"user_agent,omitempty"`
	Success       bool      `json:"success"`
	FailureReason string    `json:"failure_reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type LoginAttemptListResponse struct {
	Data  []LoginAttemptResponse `json:"data"`
	Total int                    `json:"total"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
}

type LoginAttemptQuery struct {
	UserID    string
	IPAddress string
	Success   *bool
	From      string
	To        string
	Page      int
	Limit     int
}

type CreateFraudFlagRequest struct {
	UserID      string `json:"user_id"`
	FlagType    string `json:"flag_type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Source      string `json:"source,omitempty"`
}

type ResolveFraudFlagRequest struct {
	ResolutionNote string `json:"resolution_note"`
}

type FraudFlagResponse struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	FlagType       string     `json:"flag_type"`
	Severity       string     `json:"severity"`
	Description    string     `json:"description,omitempty"`
	Source         string     `json:"source,omitempty"`
	Resolved       bool       `json:"resolved"`
	ResolvedBy     string     `json:"resolved_by,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	ResolutionNote string     `json:"resolution_note,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type FraudFlagListResponse struct {
	Data  []FraudFlagResponse `json:"data"`
	Total int                 `json:"total"`
	Page  int                 `json:"page"`
	Limit int                 `json:"limit"`
}

type FraudFlagQuery struct {
	UserID   string
	Severity string
	FlagType string
	Resolved *bool
	Page     int
	Limit    int
}

type BlockIPRequest struct {
	IPAddress string     `json:"ip_address"`
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type IPBlacklistResponse struct {
	ID        string     `json:"id"`
	IPAddress string     `json:"ip_address"`
	Reason    string     `json:"reason,omitempty"`
	BlockedBy string     `json:"blocked_by,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
}

type IPBlacklistListResponse struct {
	Data  []IPBlacklistResponse `json:"data"`
	Total int                   `json:"total"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
}

type IPBlacklistQuery struct {
	Query      string
	ActiveOnly bool
	Page       int
	Limit      int
}

type ValidateIPRequest struct {
	IPAddress string `json:"ip_address"`
}

type ValidateIPResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type ValidateUserRequest struct {
	UserID string `json:"user_id"`
}

type ValidateUserResponse struct {
	Allowed     bool   `json:"allowed"`
	ActiveFlags int    `json:"active_flags"`
	MaxSeverity string `json:"max_severity,omitempty"`
}

type UserFraudSummaryResponse struct {
	UserID              string                 `json:"user_id"`
	RiskScore           int                    `json:"risk_score"`
	ActiveFlags         int                    `json:"active_flags"`
	TotalFlags          int                    `json:"total_flags"`
	RecentFailedLogins  int                    `json:"recent_failed_logins"`
	Flags               []FraudFlagResponse    `json:"flags"`
	RecentLoginAttempts []LoginAttemptResponse `json:"recent_login_attempts"`
}

// ── Interface ─────────────────────────────────────────────────────────────────

// SecurityServiceInterface defines the service contract.
//
//go:generate mockgen -source=security_service.go -destination=../../mocks/mock_security_service.go -package=mocks
type SecurityServiceInterface interface {
	// Login attempts
	RecordLoginAttempt(ctx context.Context, req *RecordLoginAttemptRequest) (*RecordLoginAttemptResponse, error)
	ListMyLoginAttempts(ctx context.Context, userID string, q LoginAttemptQuery) (*LoginAttemptListResponse, error)
	AdminListLoginAttempts(ctx context.Context, q LoginAttemptQuery) (*LoginAttemptListResponse, error)

	// Fraud flags
	CreateFraudFlag(ctx context.Context, adminID string, req *CreateFraudFlagRequest) (*FraudFlagResponse, error)
	GetFraudFlag(ctx context.Context, flagID string) (*FraudFlagResponse, error)
	ListFraudFlags(ctx context.Context, q FraudFlagQuery) (*FraudFlagListResponse, error)
	ResolveFraudFlag(ctx context.Context, flagID, adminID string, req *ResolveFraudFlagRequest) (*FraudFlagResponse, error)
	GetUserFraudSummary(ctx context.Context, userID string) (*UserFraudSummaryResponse, error)

	// IP blacklist
	BlockIP(ctx context.Context, adminID string, req *BlockIPRequest) (*IPBlacklistResponse, error)
	UnblockIP(ctx context.Context, blockID string) error
	ListBlockedIPs(ctx context.Context, q IPBlacklistQuery) (*IPBlacklistListResponse, error)

	// Validation (internal)
	ValidateIP(ctx context.Context, req *ValidateIPRequest) (*ValidateIPResponse, error)
	ValidateUser(ctx context.Context, req *ValidateUserRequest) (*ValidateUserResponse, error)
}

// ── Implementation ────────────────────────────────────────────────────────────

// SecurityService implements SecurityServiceInterface.
type SecurityService struct {
	repo   repository.SecurityRepositoryInterface
	logger *slog.Logger
}

// NewSecurityService creates a new SecurityService.
func NewSecurityService(repo repository.SecurityRepositoryInterface, logger *slog.Logger) *SecurityService {
	return &SecurityService{repo: repo, logger: logger}
}

// ── Login Attempts ────────────────────────────────────────────────────────────

func (s *SecurityService) RecordLoginAttempt(ctx context.Context, req *RecordLoginAttemptRequest) (*RecordLoginAttemptResponse, error) {
	if req == nil || req.UserID == "" || req.IPAddress == "" {
		return nil, ErrValidation
	}

	a := &repository.LoginAttempt{
		UserID:        req.UserID,
		IPAddress:     req.IPAddress,
		UserAgent:     sql.NullString{String: req.UserAgent, Valid: req.UserAgent != ""},
		Success:       req.Success,
		FailureReason: sql.NullString{String: req.FailureReason, Valid: req.FailureReason != ""},
	}
	if _, err := s.repo.InsertLoginAttempt(ctx, a); err != nil {
		return nil, err
	}

	autoFlagged := false
	if !req.Success {
		autoFlagged = s.checkAndAutoFlag(ctx, req.UserID)
	}

	return &RecordLoginAttemptResponse{Recorded: true, AutoFlagged: autoFlagged}, nil
}

// checkAndAutoFlag checks thresholds and creates a fraud flag if triggered.
// Returns true if a new flag was created.
func (s *SecurityService) checkAndAutoFlag(ctx context.Context, userID string) bool {
	since := time.Now().Add(-failedLoginWindowMin * time.Minute)
	count, err := s.repo.CountFailedLoginsInWindow(ctx, userID, since)
	if err != nil || count < failedLoginThreshold {
		return false
	}

	flag := &repository.FraudFlag{
		UserID:      userID,
		FlagType:    "suspicious_login",
		Severity:    "medium",
		Description: sql.NullString{String: "5+ failed login attempts in 15 minutes", Valid: true},
		Source:      sql.NullString{String: "auto_rule", Valid: true},
	}
	if _, err := s.repo.InsertFraudFlag(ctx, flag); err != nil {
		s.logger.Warn("auto-flag insert failed", slog.String("user_id", userID), slog.Any("err", err))
		return false
	}
	s.logger.Info("auto-flagged user for suspicious_login", slog.String("user_id", userID))
	return true
}

func (s *SecurityService) ListMyLoginAttempts(ctx context.Context, userID string, q LoginAttemptQuery) (*LoginAttemptListResponse, error) {
	if userID == "" {
		return nil, ErrForbidden
	}
	q.UserID = userID
	return s.listLoginAttempts(ctx, q)
}

func (s *SecurityService) AdminListLoginAttempts(ctx context.Context, q LoginAttemptQuery) (*LoginAttemptListResponse, error) {
	return s.listLoginAttempts(ctx, q)
}

func (s *SecurityService) listLoginAttempts(ctx context.Context, q LoginAttemptQuery) (*LoginAttemptListResponse, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 20
	}

	f := repository.LoginAttemptFilter{
		UserID:    q.UserID,
		IPAddress: q.IPAddress,
		Success:   q.Success,
		Limit:     q.Limit,
		Offset:    (q.Page - 1) * q.Limit,
	}
	if q.From != "" {
		if t, err := time.Parse("2006-01-02", q.From); err == nil {
			f.From = &t
		}
	}
	if q.To != "" {
		if t, err := time.Parse("2006-01-02", q.To); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			f.To = &t
		}
	}

	rows, total, err := s.repo.ListLoginAttempts(ctx, f)
	if err != nil {
		return nil, err
	}
	out := &LoginAttemptListResponse{
		Data:  make([]LoginAttemptResponse, 0, len(rows)),
		Total: total,
		Page:  q.Page,
		Limit: q.Limit,
	}
	for _, a := range rows {
		out.Data = append(out.Data, toLoginAttemptResponse(a))
	}
	return out, nil
}

// ── Fraud Flags ───────────────────────────────────────────────────────────────

func (s *SecurityService) CreateFraudFlag(ctx context.Context, adminID string, req *CreateFraudFlagRequest) (*FraudFlagResponse, error) {
	if req == nil || req.UserID == "" || req.FlagType == "" || req.Severity == "" || req.Description == "" {
		return nil, ErrValidation
	}
	if !validSeverity(req.Severity) {
		return nil, &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "severity must be low, medium, high, or critical"}
	}

	f := &repository.FraudFlag{
		UserID:      req.UserID,
		FlagType:    req.FlagType,
		Severity:    req.Severity,
		Description: sql.NullString{String: req.Description, Valid: true},
		Source:      sql.NullString{String: req.Source, Valid: req.Source != ""},
	}
	created, err := s.repo.InsertFraudFlag(ctx, f)
	if err != nil {
		return nil, err
	}

	s.logger.Info("fraud flag created",
		slog.String("flag_id", created.ID),
		slog.String("user_id", req.UserID),
		slog.String("type", req.FlagType),
		slog.String("admin_id", adminID),
	)

	resp := toFraudFlagResponse(created)
	return &resp, nil
}

func (s *SecurityService) GetFraudFlag(ctx context.Context, flagID string) (*FraudFlagResponse, error) {
	f, err := s.repo.GetFraudFlagByID(ctx, flagID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFlagNotFound
		}
		return nil, err
	}
	resp := toFraudFlagResponse(f)
	return &resp, nil
}

func (s *SecurityService) ListFraudFlags(ctx context.Context, q FraudFlagQuery) (*FraudFlagListResponse, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 20
	}

	f := repository.FraudFlagFilter{
		UserID:   q.UserID,
		Severity: q.Severity,
		FlagType: q.FlagType,
		Resolved: q.Resolved,
		Limit:    q.Limit,
		Offset:   (q.Page - 1) * q.Limit,
	}

	rows, total, err := s.repo.ListFraudFlags(ctx, f)
	if err != nil {
		return nil, err
	}
	out := &FraudFlagListResponse{
		Data:  make([]FraudFlagResponse, 0, len(rows)),
		Total: total,
		Page:  q.Page,
		Limit: q.Limit,
	}
	for _, ff := range rows {
		out.Data = append(out.Data, toFraudFlagResponse(ff))
	}
	return out, nil
}

func (s *SecurityService) ResolveFraudFlag(ctx context.Context, flagID, adminID string, req *ResolveFraudFlagRequest) (*FraudFlagResponse, error) {
	if req == nil || req.ResolutionNote == "" {
		return nil, ErrValidation
	}

	existing, err := s.repo.GetFraudFlagByID(ctx, flagID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFlagNotFound
		}
		return nil, err
	}
	if existing.Resolved {
		return nil, ErrFlagAlreadyResolved
	}

	resolved, err := s.repo.ResolveFraudFlag(ctx, flagID, adminID, req.ResolutionNote, time.Now().UTC())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFlagAlreadyResolved
		}
		return nil, err
	}

	s.logger.Info("fraud flag resolved",
		slog.String("flag_id", flagID),
		slog.String("admin_id", adminID),
	)

	resp := toFraudFlagResponse(resolved)
	return &resp, nil
}

func (s *SecurityService) GetUserFraudSummary(ctx context.Context, userID string) (*UserFraudSummaryResponse, error) {
	activeFlags, err := s.repo.CountActiveFlagsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	allFlagRows, total, err := s.repo.ListFraudFlags(ctx, repository.FraudFlagFilter{
		UserID: userID,
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		return nil, err
	}

	since24h := time.Now().Add(-24 * time.Hour)
	recentFailed, err := s.repo.CountFailedLoginsInWindow(ctx, userID, since24h)
	if err != nil {
		return nil, err
	}

	recentAttemptRows, _, err := s.repo.ListLoginAttempts(ctx, repository.LoginAttemptFilter{
		UserID: userID,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		return nil, err
	}

	maxSev, _ := s.repo.GetMaxSeverityForUser(ctx, userID)

	riskScore := computeRiskScore(activeFlags, maxSev, recentFailed)

	summary := &UserFraudSummaryResponse{
		UserID:             userID,
		RiskScore:          riskScore,
		ActiveFlags:        activeFlags,
		TotalFlags:         total,
		RecentFailedLogins: recentFailed,
		Flags:              make([]FraudFlagResponse, 0, len(allFlagRows)),
		RecentLoginAttempts: make([]LoginAttemptResponse, 0, len(recentAttemptRows)),
	}
	for _, f := range allFlagRows {
		summary.Flags = append(summary.Flags, toFraudFlagResponse(f))
	}
	for _, a := range recentAttemptRows {
		summary.RecentLoginAttempts = append(summary.RecentLoginAttempts, toLoginAttemptResponse(a))
	}
	return summary, nil
}

// ── IP Blacklist ──────────────────────────────────────────────────────────────

func (s *SecurityService) BlockIP(ctx context.Context, adminID string, req *BlockIPRequest) (*IPBlacklistResponse, error) {
	if req == nil || req.IPAddress == "" || req.Reason == "" {
		return nil, ErrValidation
	}

	// Check if already blocked
	if existing, err := s.repo.GetActiveIPBlock(ctx, req.IPAddress); err == nil && existing != nil {
		return nil, ErrIPAlreadyBlocked
	}

	e := &repository.IPBlacklistEntry{
		IPAddress: req.IPAddress,
		Reason:    sql.NullString{String: req.Reason, Valid: true},
		BlockedBy: sql.NullString{String: adminID, Valid: adminID != ""},
	}
	if req.ExpiresAt != nil {
		e.ExpiresAt = sql.NullTime{Time: *req.ExpiresAt, Valid: true}
	}

	created, err := s.repo.InsertIPBlock(ctx, e)
	if err != nil {
		return nil, err
	}

	s.logger.Info("IP blocked",
		slog.String("ip", req.IPAddress),
		slog.String("admin_id", adminID),
	)

	resp := toIPBlacklistResponse(created)
	return &resp, nil
}

func (s *SecurityService) UnblockIP(ctx context.Context, blockID string) error {
	if err := s.repo.DeactivateIPBlock(ctx, blockID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrBlockNotFound
		}
		return err
	}
	s.logger.Info("IP unblocked", slog.String("block_id", blockID))
	return nil
}

func (s *SecurityService) ListBlockedIPs(ctx context.Context, q IPBlacklistQuery) (*IPBlacklistListResponse, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 20
	}

	f := repository.IPBlacklistFilter{
		Query:      q.Query,
		ActiveOnly: q.ActiveOnly,
		Limit:      q.Limit,
		Offset:     (q.Page - 1) * q.Limit,
	}

	rows, total, err := s.repo.ListBlockedIPs(ctx, f)
	if err != nil {
		return nil, err
	}
	out := &IPBlacklistListResponse{
		Data:  make([]IPBlacklistResponse, 0, len(rows)),
		Total: total,
		Page:  q.Page,
		Limit: q.Limit,
	}
	for _, e := range rows {
		out.Data = append(out.Data, toIPBlacklistResponse(e))
	}
	return out, nil
}

// ── Validation ────────────────────────────────────────────────────────────────

func (s *SecurityService) ValidateIP(ctx context.Context, req *ValidateIPRequest) (*ValidateIPResponse, error) {
	if req == nil || req.IPAddress == "" {
		return nil, ErrValidation
	}

	block, err := s.repo.GetActiveIPBlock(ctx, req.IPAddress)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &ValidateIPResponse{Allowed: true}, nil
		}
		return nil, err
	}
	return &ValidateIPResponse{Allowed: false, Reason: block.Reason.String}, nil
}

func (s *SecurityService) ValidateUser(ctx context.Context, req *ValidateUserRequest) (*ValidateUserResponse, error) {
	if req == nil || req.UserID == "" {
		return nil, ErrValidation
	}

	activeFlags, err := s.repo.CountActiveFlagsForUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	maxSev, err := s.repo.GetMaxSeverityForUser(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	allowed := maxSev != "critical"
	return &ValidateUserResponse{
		Allowed:     allowed,
		ActiveFlags: activeFlags,
		MaxSeverity: maxSev,
	}, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func validSeverity(s string) bool {
	return s == "low" || s == "medium" || s == "high" || s == "critical"
}

func computeRiskScore(activeFlags int, maxSev string, recentFailed int) int {
	sevWeight := map[string]int{"low": 5, "medium": 15, "high": 25, "critical": 40}
	score := activeFlags*10 + sevWeight[maxSev] + recentFailed*2
	if score > 100 {
		return 100
	}
	return score
}

// ── Mappers ───────────────────────────────────────────────────────────────────

func toLoginAttemptResponse(a *repository.LoginAttempt) LoginAttemptResponse {
	r := LoginAttemptResponse{
		ID:        a.ID,
		UserID:    a.UserID,
		IPAddress: a.IPAddress,
		Success:   a.Success,
		CreatedAt: a.CreatedAt,
	}
	if a.UserAgent.Valid {
		r.UserAgent = a.UserAgent.String
	}
	if a.FailureReason.Valid {
		r.FailureReason = a.FailureReason.String
	}
	return r
}

func toFraudFlagResponse(f *repository.FraudFlag) FraudFlagResponse {
	r := FraudFlagResponse{
		ID:        f.ID,
		UserID:    f.UserID,
		FlagType:  f.FlagType,
		Severity:  f.Severity,
		Resolved:  f.Resolved,
		CreatedAt: f.CreatedAt,
	}
	if f.Description.Valid {
		r.Description = f.Description.String
	}
	if f.Source.Valid {
		r.Source = f.Source.String
	}
	if f.ResolvedBy.Valid {
		r.ResolvedBy = f.ResolvedBy.String
	}
	if f.ResolvedAt.Valid {
		t := f.ResolvedAt.Time
		r.ResolvedAt = &t
	}
	if f.ResolutionNote.Valid {
		r.ResolutionNote = f.ResolutionNote.String
	}
	return r
}

func toIPBlacklistResponse(e *repository.IPBlacklistEntry) IPBlacklistResponse {
	r := IPBlacklistResponse{
		ID:        e.ID,
		IPAddress: e.IPAddress,
		IsActive:  e.IsActive,
		CreatedAt: e.CreatedAt,
	}
	if e.Reason.Valid {
		r.Reason = e.Reason.String
	}
	if e.BlockedBy.Valid {
		r.BlockedBy = e.BlockedBy.String
	}
	if e.ExpiresAt.Valid {
		t := e.ExpiresAt.Time
		r.ExpiresAt = &t
	}
	return r
}
