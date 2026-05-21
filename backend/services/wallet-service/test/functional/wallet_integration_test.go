//go:build functional

package functional

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/wallet-service/internal/repository"
)

func setupTestDB(t *testing.T) *sql.DB {
	dsn := "postgres://clay_user:clay_password@localhost:5452/wallet_db?sslmode=disable"
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
	
	DROP TABLE IF EXISTS wallet_transactions CASCADE;
	DROP TABLE IF EXISTS wallets CASCADE;

	CREATE TABLE IF NOT EXISTS wallets (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID UNIQUE NOT NULL,
		balance BIGINT NOT NULL DEFAULT 0,
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS wallet_transactions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
		type VARCHAR(50) NOT NULL,
		amount BIGINT NOT NULL,
		balance_after BIGINT NOT NULL,
		reference_id UUID NOT NULL,
		description TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	TRUNCATE TABLE wallets CASCADE;
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

func TestWalletRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Wallet Service (Database Integration)...")
	
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewWalletRepository(db)
	ctx := context.Background()

	t.Run("Create and Get Wallet", func(t *testing.T) {
		userID := uuid.New()

		// 1. Create
		created, err := repo.CreateWallet(ctx, userID)
		if err != nil {
			t.Fatalf("failed to create wallet: %v", err)
		}
		t.Logf("Successfully inserted wallet with ID: %s", created.ID)

		if created.Balance != 0 {
			t.Errorf("expected balance 0, got %d", created.Balance)
		}

		// 2. Get
		found, err := repo.GetWalletByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("failed to get wallet: %v", err)
		}
		
		if found.ID != created.ID {
			t.Errorf("expected ID %s, got %s", created.ID, found.ID)
		}
	})

	t.Run("Credit and Debit Wallet", func(t *testing.T) {
		userID := uuid.New()
		_, err := repo.CreateWallet(ctx, userID)
		if err != nil {
			t.Fatalf("failed to create wallet: %v", err)
		}

		// Credit
		refID := uuid.New()
		txCredit, err := repo.CreditWallet(ctx, userID, 50000, "top_up", "Test Topup", refID)
		if err != nil {
			t.Fatalf("failed to credit wallet: %v", err)
		}
		if txCredit.BalanceAfter != 50000 {
			t.Errorf("expected balance after 50000, got %d", txCredit.BalanceAfter)
		}

		// Debit
		refID2 := uuid.New()
		txDebit, err := repo.DebitWallet(ctx, userID, 15000, "payment", "Test Payment", refID2)
		if err != nil {
			t.Fatalf("failed to debit wallet: %v", err)
		}
		if txDebit.BalanceAfter != 35000 {
			t.Errorf("expected balance after 35000, got %d", txDebit.BalanceAfter)
		}

		// Debit insufficient
		refID3 := uuid.New()
		_, err = repo.DebitWallet(ctx, userID, 40000, "payment", "Test Payment Failed", refID3)
		if err != repository.ErrInsufficientBalance {
			t.Fatalf("expected ErrInsufficientBalance, got %v", err)
		}
	})
}
