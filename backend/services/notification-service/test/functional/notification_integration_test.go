//go:build functional

package functional

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/notification-service/internal/repository"
)

func setupTestDB(t *testing.T) *sql.DB {
	dsn := "postgres://clay_user:clay_password@localhost:5435/notification_db?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Wait for db to be ready
	for i := 0; i < 5; i++ {
		err = db.Ping()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	schema := `
	CREATE EXTENSION IF NOT EXISTS "pgcrypto";

	DROP TABLE IF EXISTS notification_logs CASCADE;
	DROP TABLE IF EXISTS notification_templates CASCADE;
	DROP TABLE IF EXISTS user_notif_prefs CASCADE;
	DROP TABLE IF EXISTS device_tokens CASCADE;

	CREATE TABLE IF NOT EXISTS device_tokens (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL,
		token TEXT UNIQUE NOT NULL,
		platform VARCHAR(20) NOT NULL,
		app_version VARCHAR(20),
		is_active BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS user_notif_prefs (
		user_id UUID PRIMARY KEY,
		push_enabled BOOLEAN NOT NULL DEFAULT true,
		email_enabled BOOLEAN NOT NULL DEFAULT true,
		sms_enabled BOOLEAN NOT NULL DEFAULT true,
		quiet_hours_start TIME,
		quiet_hours_end TIME,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS notification_templates (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		event_type VARCHAR(100) NOT NULL,
		channel VARCHAR(20) NOT NULL,
		title_template TEXT,
		body_template TEXT NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT true,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		UNIQUE(event_type, channel)
	);

	CREATE TABLE IF NOT EXISTS notification_logs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		recipient_id UUID NOT NULL,
		event_type VARCHAR(100) NOT NULL,
		channel VARCHAR(20) NOT NULL,
		payload JSONB DEFAULT '{}',
		rendered_title TEXT,
		rendered_body TEXT,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		error_message TEXT,
		sent_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	TRUNCATE TABLE device_tokens CASCADE;
	TRUNCATE TABLE user_notif_prefs CASCADE;
	TRUNCATE TABLE notification_templates CASCADE;
	TRUNCATE TABLE notification_logs CASCADE;
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

func TestNotificationRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Notification Service (Database Integration)...")

	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewNotificationRepository(db)
	ctx := context.Background()

	t.Run("Create and Find DeviceToken", func(t *testing.T) {
		token := &repository.DeviceToken{
			UserID:     "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
			Token:      "fcm-token-e2e-test",
			Platform:   "android",
			AppVersion: "2.4.1",
		}

		created, err := repo.CreateDeviceToken(ctx, token)
		if err != nil {
			t.Fatalf("failed to create device token: %v", err)
		}
		t.Logf("Successfully inserted device token with ID: %s", created.ID)

		if created.ID == "" {
			t.Error("expected generated ID")
		}
		if !created.IsActive {
			t.Error("expected is_active to be true")
		}

		tokens, err := repo.FindDeviceTokensByUserID(ctx, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", true)
		if err != nil {
			t.Fatalf("failed to find device tokens: %v", err)
		}
		if len(tokens) != 1 {
			t.Errorf("expected 1 token, got %d", len(tokens))
		}
		if tokens[0].Platform != "android" {
			t.Errorf("expected platform 'android', got '%s'", tokens[0].Platform)
		}
		t.Log("Successfully retrieved device token from PostgreSQL")
	})

	t.Run("Create and Find Template", func(t *testing.T) {
		title := "Driver sedang menuju ke kamu"
		tmpl := &repository.NotificationTemplate{
			EventType:     "Driver_Found",
			Channel:       "push",
			TitleTemplate: &title,
			BodyTemplate:  "{{driver_name}} ({{vehicle_plate}}) akan tiba dalam {{eta_minutes}} menit",
		}

		created, err := repo.CreateTemplate(ctx, tmpl)
		if err != nil {
			t.Fatalf("failed to create template: %v", err)
		}
		t.Logf("Successfully inserted template with ID: %s", created.ID)

		found, err := repo.FindTemplateByEventAndChannel(ctx, "Driver_Found", "push")
		if err != nil {
			t.Fatalf("failed to find template: %v", err)
		}
		if found.BodyTemplate != tmpl.BodyTemplate {
			t.Errorf("expected body '%s', got '%s'", tmpl.BodyTemplate, found.BodyTemplate)
		}
		t.Log("Successfully retrieved template from PostgreSQL")
	})

	t.Run("Upsert and Get Preferences", func(t *testing.T) {
		pref := &repository.NotificationPreference{
			UserID:       "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
			PushEnabled:  true,
			EmailEnabled: true,
			SMSEnabled:   false,
		}

		upserted, err := repo.UpsertPreferences(ctx, pref)
		if err != nil {
			t.Fatalf("failed to upsert preferences: %v", err)
		}

		found, err := repo.GetPreferences(ctx, "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11")
		if err != nil {
			t.Fatalf("failed to get preferences: %v", err)
		}
		if found.SMSEnabled {
			t.Error("expected sms_enabled to be false")
		}
		if !found.PushEnabled {
			t.Error("expected push_enabled to be true")
		}
		t.Logf("Successfully verified preferences, updated_at: %v", upserted.UpdatedAt)
	})
}
