//go:build functional

package functional

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/repository"
)

func setupTestDB(t *testing.T) *sql.DB {
	dsn := "postgres://clay_user:clay_password@localhost:5434/payment_db?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

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

	DROP TABLE IF EXISTS cod_verifications CASCADE;
	DROP TABLE IF EXISTS settlements CASCADE;
	DROP TABLE IF EXISTS refunds CASCADE;
	DROP TABLE IF EXISTS holds CASCADE;
	DROP TABLE IF EXISTS transactions CASCADE;
	DROP TABLE IF EXISTS payment_methods CASCADE;

	CREATE TABLE IF NOT EXISTS payment_methods (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL,
		type VARCHAR(50) NOT NULL,
		display_name VARCHAR(255) NOT NULL,
		last_four VARCHAR(4),
		expiry_month SMALLINT,
		expiry_year SMALLINT,
		is_default BOOLEAN NOT NULL DEFAULT false,
		card_token TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS transactions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL,
		order_id UUID,
		type VARCHAR(20) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		amount INTEGER NOT NULL,
		payment_method_type VARCHAR(50) NOT NULL,
		description TEXT,
		gateway_reference TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS holds (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		order_id UUID NOT NULL,
		user_id UUID NOT NULL,
		amount INTEGER NOT NULL,
		payment_method_type VARCHAR(50) NOT NULL,
		payment_method_id UUID,
		status VARCHAR(20) NOT NULL DEFAULT 'held',
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS refunds (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		transaction_id UUID NOT NULL,
		order_id UUID NOT NULL,
		user_id UUID NOT NULL,
		amount INTEGER NOT NULL,
		reason VARCHAR(50) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		estimated_completion TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS settlements (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		order_id UUID NOT NULL UNIQUE,
		driver_id UUID NOT NULL,
		gross_fare INTEGER NOT NULL,
		platform_fee INTEGER NOT NULL,
		driver_payout INTEGER NOT NULL,
		service_type VARCHAR(20) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS cod_verifications (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL,
		recipient_phone VARCHAR(20) NOT NULL,
		order_type VARCHAR(20) NOT NULL,
		order_summary TEXT,
		verification_type VARCHAR(30) NOT NULL,
		recipient_has_clay_acct BOOLEAN NOT NULL DEFAULT false,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		otp_hash TEXT,
		otp_attempts_remaining SMALLINT NOT NULL DEFAULT 3,
		cod_token TEXT,
		expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	TRUNCATE TABLE payment_methods, transactions, holds, refunds, settlements, cod_verifications CASCADE;
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	return db
}

func TestPaymentRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Payment Service (Database Integration)...")

	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewPaymentRepository(db, nil)
	ctx := context.Background()

	t.Run("Create and List Payment Method", func(t *testing.T) {
		pm := &repository.PaymentMethod{
			UserID:      "00000000-0000-0000-0000-000000000001",
			Type:        "credit_card",
			DisplayName: "Visa •••• 1234",
			IsDefault:   true,
		}

		created, err := repo.CreatePaymentMethod(ctx, pm)
		if err != nil {
			t.Fatalf("failed to create payment method: %v", err)
		}
		t.Logf("Created payment method ID: %s", created.ID)

		if created.ID == "" {
			t.Error("expected generated ID")
		}

		methods, err := repo.ListPaymentMethods(ctx, "00000000-0000-0000-0000-000000000001")
		if err != nil {
			t.Fatalf("failed to list payment methods: %v", err)
		}
		if len(methods) != 1 {
			t.Errorf("expected 1 method, got %d", len(methods))
		}
	})

	t.Run("Create Transaction and Find", func(t *testing.T) {
		orderID := "00000000-0000-0000-0000-000000000099"
		tx := &repository.Transaction{
			UserID:            "00000000-0000-0000-0000-000000000001",
			OrderID:           &orderID,
			Type:              "charge",
			Status:            "completed",
			Amount:            50000,
			PaymentMethodType: "clay_wallet",
			Description:       "ClayRide E2E test",
		}

		created, err := repo.CreateTransaction(ctx, tx)
		if err != nil {
			t.Fatalf("failed to create transaction: %v", err)
		}
		t.Logf("Created transaction ID: %s", created.ID)

		found, err := repo.FindTransactionByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("failed to find transaction: %v", err)
		}
		if found.Amount != 50000 {
			t.Errorf("expected amount 50000, got %d", found.Amount)
		}
		t.Log("Successfully retrieved transaction from PostgreSQL")
	})

	t.Run("Create Settlement and Check Duplicate", func(t *testing.T) {
		s := &repository.Settlement{
			OrderID:      "00000000-0000-0000-0000-000000000099",
			DriverID:     "00000000-0000-0000-0000-000000000002",
			GrossFare:    45000,
			PlatformFee:  9000,
			DriverPayout: 36000,
			ServiceType:  "ride",
			Status:       "settled",
		}

		created, err := repo.CreateSettlement(ctx, s)
		if err != nil {
			t.Fatalf("failed to create settlement: %v", err)
		}
		t.Logf("Created settlement ID: %s", created.ID)

		exists, err := repo.SettlementExistsByOrderID(ctx, "00000000-0000-0000-0000-000000000099")
		if err != nil {
			t.Fatalf("failed to check settlement: %v", err)
		}
		if !exists {
			t.Error("expected settlement to exist")
		}
		t.Log("Settlement duplicate check works correctly")
	})
}
