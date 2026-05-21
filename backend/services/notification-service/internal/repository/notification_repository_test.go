//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreateDeviceToken_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewNotificationRepository(db)

	token := &DeviceToken{
		UserID:     "user-123",
		Token:      "fcm-token-abc",
		Platform:   "android",
		AppVersion: "2.4.1",
	}

	mock.ExpectQuery(`^INSERT INTO device_tokens`).
		WithArgs(token.UserID, token.Token, token.Platform, token.AppVersion).
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_active", "updated_at"}).
			AddRow("tok-uuid-123", true, time.Now()))

	created, err := repo.CreateDeviceToken(context.Background(), token)

	if err != nil {
		t.Errorf("error was not expected while inserting device token: %s", err)
	}

	if created.ID != "tok-uuid-123" {
		t.Errorf("expected id 'tok-uuid-123', got '%s'", created.ID)
	}

	if !created.IsActive {
		t.Error("expected is_active to be true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestFindDeviceTokensByUserID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewNotificationRepository(db)

	mock.ExpectQuery(`^SELECT (.+) FROM device_tokens WHERE user_id`).
		WithArgs("user-123").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "token", "platform", "app_version", "is_active", "updated_at",
		}).AddRow("tok-1", "user-123", "fcm-abc", "android", "2.4.1", true, time.Now()))

	tokens, err := repo.FindDeviceTokensByUserID(context.Background(), "user-123", false)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].Platform != "android" {
		t.Errorf("expected platform 'android', got '%s'", tokens[0].Platform)
	}
}

func TestFindNotificationByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewNotificationRepository(db)

	mock.ExpectQuery(`^SELECT (.+) FROM notification_logs WHERE id`).
		WithArgs("unknown", "user-123").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.FindNotificationByID(context.Background(), "unknown", "user-123")

	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestCreateTemplate_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewNotificationRepository(db)
	title := "Driver sedang menuju ke kamu"

	tmpl := &NotificationTemplate{
		EventType:     "Driver_Found",
		Channel:       "push",
		TitleTemplate: &title,
		BodyTemplate:  "{{driver_name}} akan tiba dalam {{eta_minutes}} menit",
	}

	mock.ExpectQuery(`^INSERT INTO notification_templates`).
		WithArgs(tmpl.EventType, tmpl.Channel, tmpl.TitleTemplate, tmpl.BodyTemplate).
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_active", "updated_at"}).
			AddRow("tmpl-uuid-123", true, time.Now()))

	created, err := repo.CreateTemplate(context.Background(), tmpl)

	if err != nil {
		t.Errorf("error was not expected: %s", err)
	}
	if created.ID != "tmpl-uuid-123" {
		t.Errorf("expected id 'tmpl-uuid-123', got '%s'", created.ID)
	}
	if !created.IsActive {
		t.Error("expected is_active to be true")
	}
}
