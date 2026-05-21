// Package repository implements the data access layer for the Notification Service.
package repository

import (
	"context"
	"database/sql"
	"time"
)

// ── Models ───────────────────────────────────────────────────────────────────

type DeviceToken struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Token      string    `json:"token"`
	Platform   string    `json:"platform"`
	AppVersion string    `json:"app_version"`
	IsActive   bool      `json:"is_active"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type NotificationPreference struct {
	UserID          string    `json:"user_id"`
	PushEnabled     bool      `json:"push_enabled"`
	EmailEnabled    bool      `json:"email_enabled"`
	SMSEnabled      bool      `json:"sms_enabled"`
	QuietHoursStart *string   `json:"quiet_hours_start"`
	QuietHoursEnd   *string   `json:"quiet_hours_end"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type NotificationLog struct {
	ID           string     `json:"id"`
	RecipientID  string     `json:"recipient_id"`
	EventType    string     `json:"event_type"`
	Channel      string     `json:"channel"`
	Payload      string     `json:"payload"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message"`
	SentAt       *time.Time `json:"sent_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type NotificationTemplate struct {
	ID            string    `json:"id"`
	EventType     string    `json:"event_type"`
	Channel       string    `json:"channel"`
	TitleTemplate *string   `json:"title_template"`
	BodyTemplate  string    `json:"body_template"`
	IsActive      bool      `json:"is_active"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ── Interface ────────────────────────────────────────────────────────────────

//go:generate mockgen -source=notification_repository.go -destination=../../mocks/repomock/mock_notification_repository.go -package=repomock
type NotificationRepositoryInterface interface {
	CreateDeviceToken(ctx context.Context, token *DeviceToken) (*DeviceToken, error)
	FindDeviceTokensByUserID(ctx context.Context, userID string, activeOnly bool) ([]DeviceToken, error)
	DeactivateDeviceToken(ctx context.Context, tokenID, userID string) error
	GetPreferences(ctx context.Context, userID string) (*NotificationPreference, error)
	UpsertPreferences(ctx context.Context, pref *NotificationPreference) (*NotificationPreference, error)
	CreateNotificationLog(ctx context.Context, log *NotificationLog) (*NotificationLog, error)
	FindNotificationsByUserID(ctx context.Context, userID string, page, limit int) ([]NotificationLog, int, error)
	FindNotificationByID(ctx context.Context, id, userID string) (*NotificationLog, error)
	CreateTemplate(ctx context.Context, tmpl *NotificationTemplate) (*NotificationTemplate, error)
	FindTemplateByID(ctx context.Context, id string) (*NotificationTemplate, error)
	FindTemplateByEventAndChannel(ctx context.Context, eventType, channel string) (*NotificationTemplate, error)
	ListTemplates(ctx context.Context) ([]NotificationTemplate, error)
	UpdateTemplate(ctx context.Context, tmpl *NotificationTemplate) (*NotificationTemplate, error)
	DeactivateTemplate(ctx context.Context, id string) error
}

// ── Implementation ───────────────────────────────────────────────────────────

type NotificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) CreateDeviceToken(ctx context.Context, token *DeviceToken) (*DeviceToken, error) {
	query := `INSERT INTO device_tokens (user_id, token, platform, app_version, is_active)
		VALUES ($1, $2, $3, $4, true)
		ON CONFLICT (token) DO UPDATE SET user_id=$1, platform=$3, app_version=$4, is_active=true, updated_at=NOW()
		RETURNING id, is_active, updated_at`
	err := r.db.QueryRowContext(ctx, query, token.UserID, token.Token, token.Platform, token.AppVersion).Scan(&token.ID, &token.IsActive, &token.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (r *NotificationRepository) FindDeviceTokensByUserID(ctx context.Context, userID string, activeOnly bool) ([]DeviceToken, error) {
	query := `SELECT id, user_id, token, platform, app_version, is_active, updated_at FROM device_tokens WHERE user_id = $1`
	if activeOnly {
		query += ` AND is_active = true`
	}
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tokens []DeviceToken
	for rows.Next() {
		var t DeviceToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Token, &t.Platform, &t.AppVersion, &t.IsActive, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, nil
}

func (r *NotificationRepository) DeactivateDeviceToken(ctx context.Context, tokenID, userID string) error {
	query := `UPDATE device_tokens SET is_active = false, updated_at = NOW() WHERE id = $1 AND user_id = $2`
	result, err := r.db.ExecContext(ctx, query, tokenID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *NotificationRepository) GetPreferences(ctx context.Context, userID string) (*NotificationPreference, error) {
	query := `SELECT user_id, push_enabled, email_enabled, sms_enabled, quiet_hours_start, quiet_hours_end, updated_at
		FROM user_notif_prefs WHERE user_id = $1`
	pref := &NotificationPreference{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&pref.UserID, &pref.PushEnabled, &pref.EmailEnabled, &pref.SMSEnabled, &pref.QuietHoursStart, &pref.QuietHoursEnd, &pref.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return pref, nil
}

func (r *NotificationRepository) UpsertPreferences(ctx context.Context, pref *NotificationPreference) (*NotificationPreference, error) {
	query := `INSERT INTO user_notif_prefs (user_id, push_enabled, email_enabled, sms_enabled, quiet_hours_start, quiet_hours_end)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET push_enabled=$2, email_enabled=$3, sms_enabled=$4, quiet_hours_start=$5, quiet_hours_end=$6, updated_at=NOW()
		RETURNING updated_at`
	err := r.db.QueryRowContext(ctx, query, pref.UserID, pref.PushEnabled, pref.EmailEnabled, pref.SMSEnabled, pref.QuietHoursStart, pref.QuietHoursEnd).Scan(&pref.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return pref, nil
}

func (r *NotificationRepository) CreateNotificationLog(ctx context.Context, log *NotificationLog) (*NotificationLog, error) {
	query := `INSERT INTO notification_logs (recipient_id, event_type, channel, payload, status)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`
	err := r.db.QueryRowContext(ctx, query, log.RecipientID, log.EventType, log.Channel, log.Payload, log.Status).Scan(&log.ID, &log.CreatedAt)
	if err != nil {
		return nil, err
	}
	return log, nil
}

func (r *NotificationRepository) FindNotificationsByUserID(ctx context.Context, userID string, page, limit int) ([]NotificationLog, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE recipient_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * limit
	query := `SELECT id, recipient_id, event_type, channel, payload, status, error_message, sent_at, created_at
		FROM notification_logs WHERE recipient_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var logs []NotificationLog
	for rows.Next() {
		var l NotificationLog
		if err := rows.Scan(&l.ID, &l.RecipientID, &l.EventType, &l.Channel, &l.Payload, &l.Status, &l.ErrorMessage, &l.SentAt, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, nil
}

func (r *NotificationRepository) FindNotificationByID(ctx context.Context, id, userID string) (*NotificationLog, error) {
	query := `SELECT id, recipient_id, event_type, channel, payload, status, error_message, sent_at, created_at
		FROM notification_logs WHERE id = $1 AND recipient_id = $2`
	l := &NotificationLog{}
	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(&l.ID, &l.RecipientID, &l.EventType, &l.Channel, &l.Payload, &l.Status, &l.ErrorMessage, &l.SentAt, &l.CreatedAt)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (r *NotificationRepository) CreateTemplate(ctx context.Context, tmpl *NotificationTemplate) (*NotificationTemplate, error) {
	query := `INSERT INTO notification_templates (event_type, channel, title_template, body_template, is_active)
		VALUES ($1, $2, $3, $4, true) RETURNING id, is_active, updated_at`
	err := r.db.QueryRowContext(ctx, query, tmpl.EventType, tmpl.Channel, tmpl.TitleTemplate, tmpl.BodyTemplate).Scan(&tmpl.ID, &tmpl.IsActive, &tmpl.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

func (r *NotificationRepository) FindTemplateByID(ctx context.Context, id string) (*NotificationTemplate, error) {
	query := `SELECT id, event_type, channel, title_template, body_template, is_active, updated_at FROM notification_templates WHERE id = $1`
	tmpl := &NotificationTemplate{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(&tmpl.ID, &tmpl.EventType, &tmpl.Channel, &tmpl.TitleTemplate, &tmpl.BodyTemplate, &tmpl.IsActive, &tmpl.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

func (r *NotificationRepository) FindTemplateByEventAndChannel(ctx context.Context, eventType, channel string) (*NotificationTemplate, error) {
	query := `SELECT id, event_type, channel, title_template, body_template, is_active, updated_at
		FROM notification_templates WHERE event_type = $1 AND channel = $2 AND is_active = true`
	tmpl := &NotificationTemplate{}
	err := r.db.QueryRowContext(ctx, query, eventType, channel).Scan(&tmpl.ID, &tmpl.EventType, &tmpl.Channel, &tmpl.TitleTemplate, &tmpl.BodyTemplate, &tmpl.IsActive, &tmpl.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

func (r *NotificationRepository) ListTemplates(ctx context.Context) ([]NotificationTemplate, error) {
	query := `SELECT id, event_type, channel, title_template, body_template, is_active, updated_at FROM notification_templates ORDER BY event_type, channel`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var templates []NotificationTemplate
	for rows.Next() {
		var t NotificationTemplate
		if err := rows.Scan(&t.ID, &t.EventType, &t.Channel, &t.TitleTemplate, &t.BodyTemplate, &t.IsActive, &t.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, nil
}

func (r *NotificationRepository) UpdateTemplate(ctx context.Context, tmpl *NotificationTemplate) (*NotificationTemplate, error) {
	query := `UPDATE notification_templates SET title_template=$1, body_template=$2, is_active=$3, updated_at=NOW() WHERE id=$4 RETURNING updated_at`
	err := r.db.QueryRowContext(ctx, query, tmpl.TitleTemplate, tmpl.BodyTemplate, tmpl.IsActive, tmpl.ID).Scan(&tmpl.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

func (r *NotificationRepository) DeactivateTemplate(ctx context.Context, id string) error {
	query := `UPDATE notification_templates SET is_active = false, updated_at = NOW() WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
