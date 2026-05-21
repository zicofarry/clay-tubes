//go:build functional

package functional

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/database"
	"go.mongodb.org/mongo-driver/mongo"
)

func setupTestDB(t *testing.T) (*sql.DB, *mongo.Client, *redis.Client) {
	dsn := "postgres://clay_user:clay_password@localhost:5447/clay_food_order?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open postgres: %v", err)
	}
	for i := 0; i < 5; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	schema := `
	CREATE EXTENSION IF NOT EXISTS "pgcrypto";
	DROP TABLE IF EXISTS food_fare_breakdown CASCADE;
	DROP TABLE IF EXISTS food_order_state_logs CASCADE;
	DROP TABLE IF EXISTS food_orders CASCADE;

	CREATE TABLE IF NOT EXISTS food_orders (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL, merchant_id UUID NOT NULL, driver_id UUID,
		status VARCHAR(50) NOT NULL, payment_method VARCHAR(50) NOT NULL,
		payment_hold_id VARCHAR(100),
		subtotal_cents BIGINT NOT NULL, delivery_fee_cents BIGINT NOT NULL,
		discount_cents BIGINT NOT NULL, total_cents BIGINT NOT NULL,
		promo_code VARCHAR(50), notes TEXT, est_prep_time_min INT,
		cancelled_by VARCHAR(50), cancel_reason TEXT,
		rating_submitted BOOLEAN DEFAULT FALSE,
		confirmed_at TIMESTAMP WITH TIME ZONE, cancel_deadline TIMESTAMP WITH TIME ZONE,
		delivered_at TIMESTAMP WITH TIME ZONE,
		delivery_lat DECIMAL(10,7) NOT NULL, delivery_lng DECIMAL(10,7) NOT NULL,
		delivery_address TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS food_order_state_logs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		order_id UUID NOT NULL REFERENCES food_orders(id) ON DELETE CASCADE,
		from_state VARCHAR(50) NOT NULL, to_state VARCHAR(50) NOT NULL,
		actor_id UUID NOT NULL, actor_role VARCHAR(50) NOT NULL,
		notes TEXT, created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS food_fare_breakdown (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		order_id UUID NOT NULL REFERENCES food_orders(id) ON DELETE CASCADE,
		subtotal_cents BIGINT NOT NULL, delivery_fee_cents BIGINT NOT NULL,
		service_fee_cents BIGINT NOT NULL, discount_cents BIGINT NOT NULL,
		total_cents BIGINT NOT NULL, distance_km DECIMAL(10,2) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	TRUNCATE TABLE food_orders CASCADE;
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	mongoCfg := database.MongoConfig{URI: "mongodb://localhost:27021", Database: "clay_food_order_test"}
	mongoClient, _, err := database.NewMongoClient(mongoCfg)
	if err != nil {
		t.Fatalf("failed to connect to mongodb: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6387"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("failed to connect to redis: %v", err)
	}

	return db, mongoClient, rdb
}

func newRepo(t *testing.T) (repository.FoodOrderRepositoryInterface, func()) {
	db, mongoClient, rdb := setupTestDB(t)
	ctx := context.Background()
	mongoDB := mongoClient.Database("clay_food_order_test")
	_ = mongoDB.Collection("order_items").Drop(ctx)
	repo := repository.NewFoodOrderRepository(db, mongoDB, rdb)
	cleanup := func() {
		db.Close()
		mongoClient.Disconnect(ctx)
		rdb.Close()
	}
	return repo, cleanup
}

func createTestOrder(t *testing.T, repo repository.FoodOrderRepositoryInterface, userID, merchantID string) *model.FoodOrder {
	o := &model.FoodOrder{
		UserID: userID, MerchantID: merchantID,
		Status: model.StatusPending, PaymentMethod: model.PaymentGoPay,
		SubtotalCents: 20000, DeliveryFee: 10000, DiscountCents: 0, TotalCents: 30000,
		DeliveryLat: -6.2, DeliveryLng: 106.8, DeliveryAddress: "Test Address",
	}
	require.NoError(t, repo.Create(context.Background(), o))
	require.NotEmpty(t, o.ID)
	return o
}

// ─── Order CRUD ──────────────────────────────────────────────────────────────

func TestCreateAndGetOrder(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	o := createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001")

	found, err := repo.GetByID(ctx, o.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, model.StatusPending, found.Status)
	assert.Equal(t, int64(30000), found.TotalCents)
	assert.Equal(t, -6.2, found.DeliveryLat)
}

func TestGetByID_NotFound(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()

	found, err := repo.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	assert.Nil(t, found)
}

// ─── Status Transitions ─────────────────────────────────────────────────────

func TestUpdateStatus(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	o := createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001")

	err := repo.UpdateStatus(ctx, o.ID, model.StatusPreparing, nil)
	require.NoError(t, err)

	found, _ := repo.GetByID(ctx, o.ID)
	assert.Equal(t, model.StatusPreparing, found.Status)
}

func TestUpdateConfirmed(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	o := createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001")

	err := repo.UpdateConfirmed(ctx, o.ID, 20)
	require.NoError(t, err)

	found, _ := repo.GetByID(ctx, o.ID)
	assert.Equal(t, model.StatusConfirmed, found.Status)
	assert.NotNil(t, found.ConfirmedAt)
	assert.NotNil(t, found.CancelDeadline)
}

func TestUpdateCancelled(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	o := createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001")

	err := repo.UpdateCancelled(ctx, o.ID, model.CancelledByUser, "Berubah pikiran")
	require.NoError(t, err)

	found, _ := repo.GetByID(ctx, o.ID)
	assert.Equal(t, model.StatusCancelled, found.Status)
	require.NotNil(t, found.CancelledBy)
	assert.Equal(t, model.CancelledByUser, *found.CancelledBy)
	require.NotNil(t, found.CancelReason)
	assert.Equal(t, "Berubah pikiran", *found.CancelReason)
}

func TestUpdateDelivered(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	o := createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001")

	err := repo.UpdateDelivered(ctx, o.ID)
	require.NoError(t, err)

	found, _ := repo.GetByID(ctx, o.ID)
	assert.Equal(t, model.StatusDelivered, found.Status)
	assert.NotNil(t, found.DeliveredAt)
}

func TestAssignDriver(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	o := createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001")
	driverID := "123e4567-e89b-12d3-a456-426614174099"

	err := repo.AssignDriver(ctx, o.ID, driverID)
	require.NoError(t, err)

	found, _ := repo.GetByID(ctx, o.ID)
	require.NotNil(t, found.DriverID)
	assert.Equal(t, driverID, *found.DriverID)
}

func TestMarkRatingSubmitted(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	o := createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001")
	assert.False(t, o.RatingSubmitted)

	err := repo.MarkRatingSubmitted(ctx, o.ID)
	require.NoError(t, err)

	found, _ := repo.GetByID(ctx, o.ID)
	assert.True(t, found.RatingSubmitted)
}

// ─── List Queries ────────────────────────────────────────────────────────────

func TestListByUser(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	userID := "123e4567-e89b-12d3-a456-426614174000"
	merchantID := "123e4567-e89b-12d3-a456-426614174001"

	createTestOrder(t, repo, userID, merchantID)
	createTestOrder(t, repo, userID, merchantID)
	createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174099", merchantID) // different user

	t.Run("All orders for user", func(t *testing.T) {
		orders, _, err := repo.ListByUser(ctx, userID, "", 1, 10)
		require.NoError(t, err)
		assert.Len(t, orders, 2)
	})

	t.Run("Filter by status", func(t *testing.T) {
		orders, _, err := repo.ListByUser(ctx, userID, "pending", 1, 10)
		require.NoError(t, err)
		assert.Len(t, orders, 2)

		orders2, _, err := repo.ListByUser(ctx, userID, "delivered", 1, 10)
		require.NoError(t, err)
		assert.Empty(t, orders2)
	})
}

func TestListByMerchant(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	merchantID := "123e4567-e89b-12d3-a456-426614174001"
	createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174000", merchantID)
	createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174099", merchantID)

	orders, _, err := repo.ListByMerchant(ctx, merchantID, "", 1, 10)
	require.NoError(t, err)
	assert.Len(t, orders, 2)
}

// ─── Fare Breakdown ──────────────────────────────────────────────────────────

func TestFareBreakdown(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	o := createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001")

	t.Run("Save and Get", func(t *testing.T) {
		fb := &model.FareBreakdown{
			OrderID: o.ID, SubtotalCents: 20000, DeliveryFee: 10000,
			ServiceFee: 2000, DiscountCents: 5000, TotalCents: 27000, DistanceKm: 3.5,
		}
		err := repo.SaveFareBreakdown(ctx, fb)
		require.NoError(t, err)
		assert.NotEmpty(t, fb.ID)

		found, err := repo.GetFareBreakdown(ctx, o.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, int64(20000), found.SubtotalCents)
		assert.Equal(t, int64(2000), found.ServiceFee)
		assert.Equal(t, 3.5, found.DistanceKm)
	})

	t.Run("Not Found", func(t *testing.T) {
		found, err := repo.GetFareBreakdown(ctx, "00000000-0000-0000-0000-000000000000")
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// ─── State Log ───────────────────────────────────────────────────────────────

func TestLogStateTransition(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	o := createTestOrder(t, repo, "123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174001")

	log := &model.FoodOrderStateLog{
		OrderID: o.ID, FromState: model.StatusPending, ToState: model.StatusConfirmed,
		ActorID: "123e4567-e89b-12d3-a456-426614174001", ActorRole: "merchant",
	}
	err := repo.LogStateTransition(ctx, log)
	require.NoError(t, err)
	assert.NotEmpty(t, log.ID)
}

// ─── MongoDB Items ───────────────────────────────────────────────────────────

func TestMongoItems(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("Save and Get Items", func(t *testing.T) {
		orderID := "order-mongo-001"
		note := "Extra pedas"
		items := []model.FoodOrderItem{
			{
				OrderID: orderID, MenuItemID: "item-1", Name: "Nasi Goreng",
				Quantity: 2, UnitPrice: 15000, Subtotal: 30000, Notes: &note,
				Variants: []model.ItemVariant{{VariantID: "v1", VariantName: "Size", OptionID: "o1", OptionName: "Large", ExtraPrice: 5000}},
				AddOns:   []model.ItemAddOn{{AddOnID: "a1", Name: "Extra Telur", Price: 3000, Quantity: 1}},
			},
			{
				OrderID: orderID, MenuItemID: "item-2", Name: "Es Teh",
				Quantity: 1, UnitPrice: 5000, Subtotal: 5000,
			},
		}
		err := repo.SaveItems(ctx, items)
		require.NoError(t, err)

		found, err := repo.GetItemsByOrderID(ctx, orderID)
		require.NoError(t, err)
		assert.Len(t, found, 2)

		assert.Equal(t, "Nasi Goreng", found[0].Name)
		assert.Equal(t, 2, found[0].Quantity)
		require.NotNil(t, found[0].Notes)
		assert.Equal(t, "Extra pedas", *found[0].Notes)
		assert.Len(t, found[0].Variants, 1)
		assert.Equal(t, "Large", found[0].Variants[0].OptionName)
		assert.Len(t, found[0].AddOns, 1)
		assert.Equal(t, "Extra Telur", found[0].AddOns[0].Name)
	})

	t.Run("Get Items Empty", func(t *testing.T) {
		found, err := repo.GetItemsByOrderID(ctx, "nonexistent-order")
		require.NoError(t, err)
		assert.Empty(t, found)
	})
}

// ─── Redis Operations ────────────────────────────────────────────────────────

func TestRedisActiveOrder(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	userID := "123e4567-e89b-12d3-a456-426614174000"

	t.Run("Set and Get", func(t *testing.T) {
		require.NoError(t, repo.SetActiveOrder(ctx, userID, "order-001"))
		val, err := repo.GetActiveOrderID(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, "order-001", val)
	})

	t.Run("Overwrite Active Order", func(t *testing.T) {
		require.NoError(t, repo.SetActiveOrder(ctx, userID, "order-002"))
		val, err := repo.GetActiveOrderID(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, "order-002", val)
	})

	t.Run("Clear Active Order", func(t *testing.T) {
		require.NoError(t, repo.ClearActiveOrder(ctx, userID))
		val, err := repo.GetActiveOrderID(ctx, userID)
		require.NoError(t, err)
		assert.Empty(t, val)
	})

	t.Run("Get Non-Existing", func(t *testing.T) {
		val, err := repo.GetActiveOrderID(ctx, "nonexistent-user")
		require.NoError(t, err)
		assert.Empty(t, val)
	})
}

func TestMerchantQueue(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	merchantID := "123e4567-e89b-12d3-a456-426614174001"

	t.Run("Add to Queue", func(t *testing.T) {
		require.NoError(t, repo.AddToMerchantQueue(ctx, merchantID, "order-a", 1.0))
		require.NoError(t, repo.AddToMerchantQueue(ctx, merchantID, "order-b", 2.0))
	})

	t.Run("Remove from Queue", func(t *testing.T) {
		require.NoError(t, repo.RemoveFromMerchantQueue(ctx, merchantID, "order-a"))
	})

	t.Run("Remove Non-Existing (no error)", func(t *testing.T) {
		err := repo.RemoveFromMerchantQueue(ctx, merchantID, "order-zzz")
		require.NoError(t, err)
	})
}

// ─── Full Order Lifecycle ────────────────────────────────────────────────────

func TestFullOrderLifecycle(t *testing.T) {
	repo, cleanup := newRepo(t)
	defer cleanup()
	ctx := context.Background()

	userID := "123e4567-e89b-12d3-a456-426614174000"
	merchantID := "123e4567-e89b-12d3-a456-426614174001"
	driverID := "123e4567-e89b-12d3-a456-426614174099"

	// 1. Create order
	o := createTestOrder(t, repo, userID, merchantID)
	require.NoError(t, repo.SetActiveOrder(ctx, userID, o.ID))
	require.NoError(t, repo.AddToMerchantQueue(ctx, merchantID, o.ID, float64(time.Now().Unix())))

	// 2. Merchant confirms
	require.NoError(t, repo.UpdateConfirmed(ctx, o.ID, 15))
	require.NoError(t, repo.LogStateTransition(ctx, &model.FoodOrderStateLog{
		OrderID: o.ID, FromState: model.StatusPending, ToState: model.StatusConfirmed,
		ActorID: merchantID, ActorRole: "merchant",
	}))
	require.NoError(t, repo.RemoveFromMerchantQueue(ctx, merchantID, o.ID))

	// 3. Preparing
	require.NoError(t, repo.UpdateStatus(ctx, o.ID, model.StatusPreparing, nil))

	// 4. Ready
	require.NoError(t, repo.UpdateStatus(ctx, o.ID, model.StatusReady, nil))

	// 5. Assign driver
	require.NoError(t, repo.AssignDriver(ctx, o.ID, driverID))

	// 6. Picked up
	require.NoError(t, repo.UpdateStatus(ctx, o.ID, model.StatusPickedUp, nil))

	// 7. On delivery
	require.NoError(t, repo.UpdateStatus(ctx, o.ID, model.StatusOnDelivery, nil))

	// 8. Delivered
	require.NoError(t, repo.UpdateDelivered(ctx, o.ID))
	require.NoError(t, repo.ClearActiveOrder(ctx, userID))

	// 9. Save fare breakdown
	require.NoError(t, repo.SaveFareBreakdown(ctx, &model.FareBreakdown{
		OrderID: o.ID, SubtotalCents: 20000, DeliveryFee: 10000,
		ServiceFee: 2000, DiscountCents: 0, TotalCents: 32000, DistanceKm: 4.2,
	}))

	// 10. Rating
	require.NoError(t, repo.MarkRatingSubmitted(ctx, o.ID))

	// Verify final state
	final, err := repo.GetByID(ctx, o.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusDelivered, final.Status)
	assert.NotNil(t, final.DriverID)
	assert.Equal(t, driverID, *final.DriverID)
	assert.NotNil(t, final.DeliveredAt)
	assert.True(t, final.RatingSubmitted)

	fb, err := repo.GetFareBreakdown(ctx, o.ID)
	require.NoError(t, err)
	require.NotNil(t, fb)
	assert.Equal(t, int64(32000), fb.TotalCents)

	activeID, _ := repo.GetActiveOrderID(ctx, userID)
	assert.Empty(t, activeID, "active order should be cleared after delivery")
}
