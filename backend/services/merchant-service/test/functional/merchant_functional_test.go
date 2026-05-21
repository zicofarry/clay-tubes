//go:build functional

package functional

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/repository"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	testPostgresDSN = "postgres://clay_user:clay_password@localhost:5441/clay_merchant?sslmode=disable"
	testMongoURI    = "mongodb://localhost:27020"
	testMongoDB     = "clay_merchant_test"
)

func setupTestPostgres(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", testPostgresDSN)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	schema := `
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
DROP TABLE IF EXISTS merchant_bank_accounts CASCADE;
DROP TABLE IF EXISTS merchant_operating_hours CASCADE;
DROP TABLE IF EXISTS merchants CASCADE;
CREATE TABLE merchants (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id UUID NOT NULL,
	name VARCHAR(100) NOT NULL,
	description TEXT,
	category VARCHAR(50),
	status VARCHAR(30) NOT NULL DEFAULT 'pending_review',
	phone_number VARCHAR(20),
	email VARCHAR(100),
	address TEXT,
	city VARCHAR(100),
	lat DECIMAL(10,7),
	lng DECIMAL(10,7),
	logo_url TEXT,
	banner_url TEXT,
	min_order_cents BIGINT NOT NULL DEFAULT 0,
	est_delivery_min INT NOT NULL DEFAULT 30,
	rating DECIMAL(3,2) NOT NULL DEFAULT 0,
	total_reviews INT NOT NULL DEFAULT 0,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE TABLE merchant_operating_hours (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
	day_of_week SMALLINT NOT NULL,
	open_time TIME,
	close_time TIME,
	is_closed BOOLEAN NOT NULL DEFAULT false,
	updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
	UNIQUE (merchant_id, day_of_week)
);
CREATE TABLE merchant_bank_accounts (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
	bank_code VARCHAR(50) NOT NULL,
	account_number VARCHAR(30) NOT NULL UNIQUE,
	account_name VARCHAR(100) NOT NULL,
	is_primary BOOLEAN NOT NULL DEFAULT false,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
TRUNCATE merchants CASCADE;`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func setupTestMongo(t *testing.T) *mongo.Database {
	t.Helper()
	// Use a short timeout so the test fails fast when MongoDB is not running
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOpts := options.Client().
		ApplyURI(testMongoURI).
		SetConnectTimeout(5 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		t.Fatalf("connect mongo: %v\nMake sure MongoDB is running: docker-compose up -d mongodb-merchant", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		t.Fatalf("ping mongo at %s failed: %v\nMake sure MongoDB is running: docker-compose up -d mongodb-merchant", testMongoURI, err)
	}

	db := client.Database(testMongoDB)
	_ = db.Collection("menu_categories").Drop(ctx)
	_ = db.Collection("menu_items").Drop(ctx)
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return db
}

func TestMerchantRepository_E2E(t *testing.T) {
	db := setupTestPostgres(t)
	defer db.Close()
	repo := repository.NewMerchantRepository(db)
	ctx := context.Background()
	var merchantID string
	testUserID := "6fc9d511-f932-42d8-9624-db59a94c752e"

	t.Run("Create", func(t *testing.T) {
		m := &model.Merchant{
			UserID: testUserID, Name: "Warung Enak", Category: model.CategoryFood,
			PhoneNumber: "08123", Address: "Jl Sudirman", City: "Jakarta",
			Lat: -6.2, Lng: 106.8, MinOrderCents: 15000, EstDeliveryMin: 30,
		}
		if err := repo.Create(ctx, m); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if m.ID == "" {
			t.Error("expected generated ID")
		}
		if m.Status != model.StatusPendingReview {
			t.Errorf("expected pending_review, got %s", m.Status)
		}
		merchantID = m.ID
	})

	t.Run("GetByID", func(t *testing.T) {
		m, err := repo.GetByID(ctx, merchantID)
		if err != nil || m == nil {
			t.Fatalf("GetByID: %v", err)
		}
		if m.Name != "Warung Enak" {
			t.Errorf("name: %s", m.Name)
		}
	})

	t.Run("GetByUserID", func(t *testing.T) {
		m, err := repo.GetByUserID(ctx, testUserID)
		if err != nil || m == nil {
			t.Fatalf("GetByUserID: %v", err)
		}
		if m.ID != merchantID {
			t.Errorf("id mismatch")
		}
	})

	t.Run("ExistsByUserID_true", func(t *testing.T) {
		exists, err := repo.ExistsByUserID(ctx, testUserID)
		if err != nil || !exists {
			t.Errorf("ExistsByUserID: err=%v exists=%v", err, exists)
		}
	})

	t.Run("ExistsByUserID_false", func(t *testing.T) {
		exists, err := repo.ExistsByUserID(ctx, "00000000-0000-0000-0000-000000000000")
		if err != nil || exists {
			t.Errorf("expected false: err=%v exists=%v", err, exists)
		}
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		m, err := repo.UpdateStatus(ctx, merchantID, model.StatusActive)
		if err != nil || m == nil {
			t.Fatalf("UpdateStatus: %v", err)
		}
		if m.Status != model.StatusActive {
			t.Errorf("expected active, got %s", m.Status)
		}
	})

	t.Run("Update", func(t *testing.T) {
		name := "Warung Super Enak"
		m, err := repo.Update(ctx, merchantID, model.UpdateMerchantRequest{Name: &name})
		if err != nil || m == nil {
			t.Fatalf("Update: %v", err)
		}
		if m.Name != name {
			t.Errorf("name: %s", m.Name)
		}
	})
}

func TestOperatingHoursRepository_E2E(t *testing.T) {
	db := setupTestPostgres(t)
	defer db.Close()
	repo := repository.NewMerchantRepository(db)
	ctx := context.Background()

	m := &model.Merchant{UserID: "02596c59-2648-4a86-8ea7-7404ba4a957f", Name: "Hours Merchant", Category: model.CategoryFood, PhoneNumber: "0811", Address: "Jl X", City: "Kota"}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("create merchant: %v", err)
	}

	t.Run("UpsertOperatingHours", func(t *testing.T) {
		req := model.UpsertOperatingHoursRequest{Hours: []model.UpsertOperatingHoursItem{
			{DayOfWeek: 0, IsClosed: true, OpenTime: "00:00", CloseTime: "00:00"},
			{DayOfWeek: 1, OpenTime: "08:00", CloseTime: "22:00"},
			{DayOfWeek: 2, OpenTime: "08:00", CloseTime: "22:00"},
			{DayOfWeek: 3, OpenTime: "08:00", CloseTime: "22:00"},
			{DayOfWeek: 4, OpenTime: "08:00", CloseTime: "22:00"},
			{DayOfWeek: 5, OpenTime: "08:00", CloseTime: "23:00"},
			{DayOfWeek: 6, OpenTime: "09:00", CloseTime: "23:00"},
		}}
		hours, err := repo.UpsertOperatingHours(ctx, m.ID, req)
		if err != nil {
			t.Fatalf("UpsertOperatingHours: %v", err)
		}
		if len(hours) != 7 {
			t.Errorf("expected 7, got %d", len(hours))
		}
	})

	t.Run("GetOperatingHours", func(t *testing.T) {
		hours, err := repo.GetOperatingHours(ctx, m.ID)
		if err != nil {
			t.Fatalf("GetOperatingHours: %v", err)
		}
		if len(hours) != 7 {
			t.Errorf("expected 7, got %d", len(hours))
		}
		if !hours[0].IsClosed {
			t.Error("day 0 should be closed")
		}
	})
}

func TestBankAccountRepository_E2E(t *testing.T) {
	db := setupTestPostgres(t)
	defer db.Close()
	repo := repository.NewMerchantRepository(db)
	ctx := context.Background()

	m := &model.Merchant{UserID: "da164eaf-596c-4b11-b18e-6416aa5eaf7c", Name: "Bank Merchant", Category: model.CategoryFood, PhoneNumber: "0812", Address: "Jl Y", City: "Kota"}
	if err := repo.Create(ctx, m); err != nil {
		t.Fatalf("create merchant: %v", err)
	}

	var baID string
	t.Run("AddBankAccount primary", func(t *testing.T) {
		ba, err := repo.AddBankAccount(ctx, m.ID, model.AddBankAccountRequest{
			BankCode: "BCA", AccountNumber: fmt.Sprintf("1234%d", time.Now().UnixNano()),
			AccountName: "Budi Santoso", SetPrimary: true,
		})
		if err != nil || ba == nil {
			t.Fatalf("AddBankAccount: %v", err)
		}
		if !ba.IsPrimary {
			t.Error("expected is_primary=true")
		}
		baID = ba.ID
	})

	t.Run("ListBankAccounts", func(t *testing.T) {
		accounts, err := repo.ListBankAccounts(ctx, m.ID)
		if err != nil || len(accounts) == 0 {
			t.Fatalf("ListBankAccounts: %v len=%d", err, len(accounts))
		}
	})

	t.Run("SetPrimaryBankAccount", func(t *testing.T) {
		ba2, _ := repo.AddBankAccount(ctx, m.ID, model.AddBankAccountRequest{
			BankCode: "Mandiri", AccountNumber: fmt.Sprintf("9999%d", time.Now().UnixNano()),
			AccountName: "Budi", SetPrimary: false,
		})
		result, err := repo.SetPrimaryBankAccount(ctx, m.ID, ba2.ID)
		if err != nil || !result.IsPrimary {
			t.Fatalf("SetPrimaryBankAccount: %v", err)
		}
	})

	t.Run("DeleteBankAccount", func(t *testing.T) {
		if err := repo.DeleteBankAccount(ctx, baID); err != nil {
			t.Fatalf("DeleteBankAccount: %v", err)
		}
	})
}

func TestMenuRepository_E2E(t *testing.T) {
	mongoDb := setupTestMongo(t)
	repo := repository.NewMenuRepository(mongoDb)
	ctx := context.Background()
	merchantID := "test-merchant-mongo-001"

	var itemID string

	t.Run("CreateCategory", func(t *testing.T) {
		cat, err := repo.CreateCategory(ctx, merchantID, model.CreateMenuCategoryRequest{Name: "Makanan Berat", DisplayOrder: 0})
		if err != nil || cat == nil {
			t.Fatalf("CreateCategory: %v", err)
		}
	})

	t.Run("ListCategories", func(t *testing.T) {
		cats, err := repo.ListCategories(ctx, merchantID)
		if err != nil || len(cats) == 0 {
			t.Fatalf("ListCategories: %v len=%d", err, len(cats))
		}
	})

	t.Run("UpdateCategory", func(t *testing.T) {
		cats, _ := repo.ListCategories(ctx, merchantID)
		if len(cats) == 0 {
			t.Fatal("no categories to update")
		}
		newName := "Makanan Ringan"
		cat, err := repo.UpdateCategory(ctx, cats[0].ID, merchantID, model.UpdateMenuCategoryRequest{
			Name: &newName,
		})
		if err != nil || cat == nil {
			t.Fatalf("UpdateCategory: %v", err)
		}
		if cat.Name != newName {
			t.Errorf("expected %s, got %s", newName, cat.Name)
		}
	})

	t.Run("ReorderCategories", func(t *testing.T) {
		// Create a second category to reorder
		cat2, err := repo.CreateCategory(ctx, merchantID, model.CreateMenuCategoryRequest{Name: "Minuman", DisplayOrder: 1})
		if err != nil || cat2 == nil {
			t.Fatalf("CreateCategory 2: %v", err)
		}
		cats, _ := repo.ListCategories(ctx, merchantID)
		if len(cats) < 2 {
			t.Fatal("need at least 2 categories")
		}
		reordered, err := repo.ReorderCategories(ctx, merchantID, model.ReorderCategoriesRequest{
			Orders: []struct {
				CategoryID   string `json:"category_id"`
				DisplayOrder int    `json:"display_order"`
			}{
				{CategoryID: cats[0].ID, DisplayOrder: 1},
				{CategoryID: cats[1].ID, DisplayOrder: 0},
			},
		})
		if err != nil {
			t.Fatalf("ReorderCategories: %v", err)
		}
		if len(reordered) < 2 {
			t.Errorf("expected >=2 categories, got %d", len(reordered))
		}
	})

	t.Run("DeleteCategory", func(t *testing.T) {
		cats, _ := repo.ListCategories(ctx, merchantID)
		if len(cats) == 0 {
			t.Fatal("no categories to delete")
		}
		lastCat := cats[len(cats)-1]
		if err := repo.DeleteCategory(ctx, lastCat.ID, merchantID); err != nil {
			t.Fatalf("DeleteCategory: %v", err)
		}
		catsAfter, _ := repo.ListCategories(ctx, merchantID)
		if len(catsAfter) >= len(cats) {
			t.Errorf("expected fewer categories after delete")
		}
	})

	t.Run("CreateItem", func(t *testing.T) {
		item, err := repo.CreateItem(ctx, merchantID, model.CreateMenuItemRequest{
			CategoryID: "cat-001", Name: "Nasi Goreng Special", PriceCents: 30000,
			Tags: []string{"rice", "spicy"},
		})
		if err != nil || item == nil {
			t.Fatalf("CreateItem: %v", err)
		}
		itemID = item.ID
	})

	t.Run("GetItemByID", func(t *testing.T) {
		item, err := repo.GetItemByID(ctx, itemID, merchantID)
		if err != nil || item == nil {
			t.Fatalf("GetItemByID: %v", err)
		}
		if item.Name != "Nasi Goreng Special" {
			t.Errorf("name: %s", item.Name)
		}
	})

	t.Run("UpdateItem", func(t *testing.T) {
		newName := "Nasi Goreng Extra Special"
		newPrice := int64(35000)
		item, err := repo.UpdateItem(ctx, itemID, merchantID, model.UpdateMenuItemRequest{
			Name:       &newName,
			PriceCents: &newPrice,
		})
		if err != nil || item == nil {
			t.Fatalf("UpdateItem: %v", err)
		}
		if item.Name != newName {
			t.Errorf("expected %s, got %s", newName, item.Name)
		}
		if item.PriceCents != newPrice {
			t.Errorf("expected %d, got %d", newPrice, item.PriceCents)
		}
	})

	t.Run("ListItems_all", func(t *testing.T) {
		items, err := repo.ListItems(ctx, merchantID, "", nil)
		if err != nil || len(items) == 0 {
			t.Fatalf("ListItems: %v len=%d", err, len(items))
		}
	})

	t.Run("ToggleAvailability_false", func(t *testing.T) {
		item, err := repo.ToggleAvailability(ctx, itemID, merchantID, false)
		if err != nil || item.IsAvailable {
			t.Fatalf("ToggleAvailability: %v isAvail=%v", err, item.IsAvailable)
		}
	})

	t.Run("ListItems_filtered_available", func(t *testing.T) {
		avail := true
		items, err := repo.ListItems(ctx, merchantID, "", &avail)
		if err != nil {
			t.Fatalf("ListItems: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0, got %d", len(items))
		}
	})

	t.Run("BatchGetItems", func(t *testing.T) {
		items, err := repo.BatchGetItems(ctx, []string{itemID})
		if err != nil || len(items) != 1 {
			t.Fatalf("BatchGetItems: %v len=%d", err, len(items))
		}
	})

	t.Run("DeleteItem", func(t *testing.T) {
		if err := repo.DeleteItem(ctx, itemID, merchantID); err != nil {
			t.Fatalf("DeleteItem: %v", err)
		}
	})
}
