//go:build functional

package functional

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/auth-service/internal/repository"
)

func setupTestDB(t *testing.T) *sql.DB {
	dsn := "postgres://clay_user:clay_password@localhost:5431/auth_db?sslmode=disable"
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

	// Create table
	schema := `
	CREATE EXTENSION IF NOT EXISTS "pgcrypto";
	
	DROP TABLE IF EXISTS refresh_tokens CASCADE;
	DROP TABLE IF EXISTS otp_logs CASCADE;
	DROP TABLE IF EXISTS credentials CASCADE;

	CREATE TABLE IF NOT EXISTS credentials (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email VARCHAR(255) UNIQUE,
		phone VARCHAR(20) UNIQUE,
		password_hash TEXT NOT NULL,
		role VARCHAR(50) NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'active',
		email_verified BOOLEAN NOT NULL DEFAULT false,
		phone_verified BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS refresh_tokens (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES credentials(id) ON DELETE CASCADE,
		token_hash TEXT UNIQUE NOT NULL,
		device_id VARCHAR(100),
		ip_address INET,
		user_agent TEXT,
		scope VARCHAR(50),
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		revoked_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS otp_logs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		recipient VARCHAR(20) NOT NULL,
		type VARCHAR(20) NOT NULL,
		code_hash TEXT NOT NULL,
		attempts SMALLINT NOT NULL DEFAULT 0,
		max_attempts SMALLINT NOT NULL DEFAULT 3,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		ip_address INET,
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		used_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	TRUNCATE TABLE credentials CASCADE;
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

func TestAuthRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Auth Service (Database Integration)...")
	
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewAuthRepository(db, nil) // nil redis for now
	ctx := context.Background()

	t.Run("Create and Find Credential", func(t *testing.T) {
		cred := &repository.Credential{
			Email:          "e2e@example.com",
			Phone:          "+628111111111",
			PasswordHash: "hashed_super_secret",
			Role:           "user",
		}

		// 1. Create
		created, err := repo.CreateCredential(ctx, cred)
		if err != nil {
			t.Fatalf("failed to create credential: %v", err)
		}
		t.Logf("Successfully inserted credential with ID: %s", created.ID)

		if created.ID == "" {
			t.Error("expected generated ID")
		}
		if created.Status != "active" {
			t.Errorf("expected status 'active', got '%s'", created.Status)
		}

		// 2. Find by ID
		found, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("failed to find credential by ID: %v", err)
		}
		
		if found.Email != "e2e@example.com" {
			t.Errorf("expected email 'e2e@example.com', got '%s'", found.Email)
		}

		// 3. Find by Identifier (Phone)
		foundByIdent, err := repo.FindByIdentifier(ctx, "+628111111111")
		if err != nil {
			t.Fatalf("failed to find credential by phone: %v", err)
		}

		if foundByIdent.ID != created.ID {
			t.Errorf("expected ID %s, got %s", created.ID, foundByIdent.ID)
		}
		t.Log("Successfully retrieved credential from PostgreSQL")

		// 4. Exists by Email or Phone
		exists, err := repo.ExistsByEmailOrPhone(ctx, "e2e@example.com", "")
		if err != nil || !exists {
			t.Errorf("expected email to exist, err: %v", err)
		}
		
		exists, err = repo.ExistsByEmailOrPhone(ctx, "", "+628111111111")
		if err != nil || !exists {
			t.Errorf("expected phone to exist, err: %v", err)
		}
		
		exists, err = repo.ExistsByEmailOrPhone(ctx, "nonexistent@example.com", "+628999999999")
		if err != nil || exists {
			t.Errorf("expected email/phone to not exist, err: %v", err)
		}
		t.Log("Successfully verified ExistsByEmailOrPhone")

		// 5. Update Password
		err = repo.UpdatePassword(ctx, created.ID, "new_hashed_secret")
		if err != nil {
			t.Fatalf("failed to update password: %v", err)
		}

		updated, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("failed to find credential after password update: %v", err)
		}
		if updated.PasswordHash != "new_hashed_secret" {
			t.Errorf("expected updated password hash, got '%s'", updated.PasswordHash)
		}
		t.Log("Successfully updated and verified password")

		// 6. Set Phone Verified
		err = repo.SetPhoneVerified(ctx, "+628111111111")
		if err != nil {
			t.Fatalf("failed to set phone verified: %v", err)
		}

		verified, err := repo.FindByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("failed to find credential after phone verification: %v", err)
		}
		if !verified.PhoneVerified {
			t.Error("expected phone to be verified")
		}
		t.Log("Successfully verified phone number")
	})
}

