package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	activeOrderKeyPrefix    = "user:active_food:"
	activeOrderTTL          = 3 * time.Hour
	orderCacheKeyPrefix     = "food_order:"
	orderCacheTTL           = 3 * time.Hour
	merchantPendingSetPrefix = "merchant:pending_orders:"
)

// FoodOrderRepositoryInterface defines all persistence operations for food orders.
// It is implemented by FoodOrderRepository and mocked in service-level unit tests.
//
//go:generate mockgen -source=food_order_repository.go -destination=../../mocks/repomock/mock_food_order_repository.go -package=repomock
type FoodOrderRepositoryInterface interface {
	// ── PostgreSQL ────────────────────────────────────────────────────────

	// Create inserts a new food order into PostgreSQL.
	Create(ctx context.Context, order *model.FoodOrder) error

	// GetByID fetches a food order by primary key. Returns nil if not found.
	GetByID(ctx context.Context, orderID string) (*model.FoodOrder, error)

	// UpdateStatus sets a new status (and optional extra fields) on an order.
	UpdateStatus(ctx context.Context, orderID string, status model.OrderStatus, updates map[string]interface{}) error

	// UpdateConfirmed transitions to confirmed and records the cancel deadline.
	UpdateConfirmed(ctx context.Context, orderID string, estPrepMin int) error

	// UpdateCancelled marks an order as cancelled with actor and reason.
	UpdateCancelled(ctx context.Context, orderID string, by model.CancelledBy, reason string) error

	// AssignDriver assigns a driver_id to an existing order.
	AssignDriver(ctx context.Context, orderID, driverID string) error

	// UpdateDelivered marks the order as delivered and sets delivered_at.
	UpdateDelivered(ctx context.Context, orderID string) error

	// MarkRatingSubmitted sets rating_submitted = true on an order.
	MarkRatingSubmitted(ctx context.Context, orderID string) error

	// ListByUser returns paginated orders for a given user.
	ListByUser(ctx context.Context, userID, status string, page, limit int) ([]model.FoodOrder, int, error)

	// ListByMerchant returns paginated orders for a given merchant.
	ListByMerchant(ctx context.Context, merchantID, status string, page, limit int) ([]model.FoodOrder, int, error)

	// SaveFareBreakdown inserts fare breakdown data after order completion.
	SaveFareBreakdown(ctx context.Context, fb *model.FareBreakdown) error

	// GetFareBreakdown retrieves fare breakdown by order ID.
	GetFareBreakdown(ctx context.Context, orderID string) (*model.FareBreakdown, error)

	// LogStateTransition records a state change to food_order_state_logs.
	LogStateTransition(ctx context.Context, log *model.FoodOrderStateLog) error

	// ── MongoDB ───────────────────────────────────────────────────────────

	// SaveItems inserts order items into MongoDB.
	SaveItems(ctx context.Context, items []model.FoodOrderItem) error

	// GetItemsByOrderID retrieves order items from MongoDB.
	GetItemsByOrderID(ctx context.Context, orderID string) ([]model.FoodOrderItem, error)

	// ── Redis ─────────────────────────────────────────────────────────────

	// SetActiveOrder writes user:active_food:{userID} = orderID with TTL.
	SetActiveOrder(ctx context.Context, userID, orderID string) error

	// GetActiveOrderID reads the active order ID for a user from Redis.
	GetActiveOrderID(ctx context.Context, userID string) (string, error)

	// ClearActiveOrder deletes the active order key for a user.
	ClearActiveOrder(ctx context.Context, userID string) error

	// AddToMerchantQueue adds an order to the merchant's pending sorted set.
	AddToMerchantQueue(ctx context.Context, merchantID, orderID string, score float64) error

	// RemoveFromMerchantQueue removes an order from the merchant's pending sorted set.
	RemoveFromMerchantQueue(ctx context.Context, merchantID, orderID string) error
}

// FoodOrderRepository handles all persistence for food orders.
type FoodOrderRepository struct {
	db      *sql.DB
	mongo   *mongo.Database
	redis   *redis.Client
}

// NewFoodOrderRepository creates a new repository.
func NewFoodOrderRepository(db *sql.DB, mongoDb *mongo.Database, rdb *redis.Client) *FoodOrderRepository {
	return &FoodOrderRepository{db: db, mongo: mongoDb, redis: rdb}
}

// ── PostgreSQL operations ────────────────────────────────────────────────────

func (r *FoodOrderRepository) Create(ctx context.Context, order *model.FoodOrder) error {
	order.ID = uuid.New().String()
	order.CreatedAt = time.Now().UTC()
	order.UpdatedAt = time.Now().UTC()

	query := `
		INSERT INTO food_orders (
			id, user_id, merchant_id, status, payment_method,
			subtotal_cents, delivery_fee_cents, discount_cents, total_cents,
			promo_code, notes, delivery_lat, delivery_lng, delivery_address,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`

	_, err := r.db.ExecContext(ctx, query,
		order.ID, order.UserID, order.MerchantID, order.Status, order.PaymentMethod,
		order.SubtotalCents, order.DeliveryFee, order.DiscountCents, order.TotalCents,
		order.PromoCode, order.Notes, order.DeliveryLat, order.DeliveryLng, order.DeliveryAddress,
		order.CreatedAt, order.UpdatedAt,
	)
	return err
}

func (r *FoodOrderRepository) GetByID(ctx context.Context, orderID string) (*model.FoodOrder, error) {
	query := `
		SELECT id, user_id, merchant_id, driver_id, status, payment_method,
		       payment_hold_id, subtotal_cents, delivery_fee_cents, discount_cents,
		       total_cents, promo_code, notes, est_prep_time_min, cancelled_by,
		       cancel_reason, rating_submitted, confirmed_at, cancel_deadline,
		       delivered_at, delivery_lat, delivery_lng, delivery_address,
		       created_at, updated_at
		FROM food_orders WHERE id = $1`

	var o model.FoodOrder
	err := r.db.QueryRowContext(ctx, query, orderID).Scan(
		&o.ID, &o.UserID, &o.MerchantID, &o.DriverID, &o.Status, &o.PaymentMethod,
		&o.PaymentHoldID, &o.SubtotalCents, &o.DeliveryFee, &o.DiscountCents,
		&o.TotalCents, &o.PromoCode, &o.Notes, &o.EstPrepTimeMин, &o.CancelledBy,
		&o.CancelReason, &o.RatingSubmitted, &o.ConfirmedAt, &o.CancelDeadline,
		&o.DeliveredAt, &o.DeliveryLat, &o.DeliveryLng, &o.DeliveryAddress,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &o, err
}

func (r *FoodOrderRepository) UpdateStatus(ctx context.Context, orderID string, status model.OrderStatus, updates map[string]interface{}) error {
	if updates == nil {
		updates = make(map[string]interface{})
	}
	updates["status"] = string(status)
	updates["updated_at"] = time.Now().UTC()

	query := `UPDATE food_orders SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, updates["updated_at"], orderID)
	return err
}

func (r *FoodOrderRepository) UpdateConfirmed(ctx context.Context, orderID string, estPrepMin int) error {
	now := time.Now().UTC()
	deadline := now.Add(2 * time.Minute)
	query := `
		UPDATE food_orders
		SET status = $1, confirmed_at = $2, cancel_deadline = $3,
		    est_prep_time_min = $4, updated_at = $5
		WHERE id = $6`
	_, err := r.db.ExecContext(ctx, query,
		model.StatusConfirmed, now, deadline, estPrepMin, now, orderID)
	return err
}

func (r *FoodOrderRepository) UpdateCancelled(ctx context.Context, orderID string, by model.CancelledBy, reason string) error {
	now := time.Now().UTC()
	query := `
		UPDATE food_orders
		SET status = $1, cancelled_by = $2, cancel_reason = $3, updated_at = $4
		WHERE id = $5`
	_, err := r.db.ExecContext(ctx, query, model.StatusCancelled, by, reason, now, orderID)
	return err
}

func (r *FoodOrderRepository) AssignDriver(ctx context.Context, orderID, driverID string) error {
	now := time.Now().UTC()
	query := `UPDATE food_orders SET driver_id = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, driverID, now, orderID)
	return err
}

func (r *FoodOrderRepository) UpdateDelivered(ctx context.Context, orderID string) error {
	now := time.Now().UTC()
	query := `
		UPDATE food_orders
		SET status = $1, delivered_at = $2, updated_at = $3
		WHERE id = $4`
	_, err := r.db.ExecContext(ctx, query, model.StatusDelivered, now, now, orderID)
	return err
}

func (r *FoodOrderRepository) MarkRatingSubmitted(ctx context.Context, orderID string) error {
	query := `UPDATE food_orders SET rating_submitted = true, updated_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now().UTC(), orderID)
	return err
}

func (r *FoodOrderRepository) ListByUser(ctx context.Context, userID string, status string, page, limit int) ([]model.FoodOrder, int, error) {
	offset := (page - 1) * limit
	args := []interface{}{userID, limit, offset}
	whereExtra := ""
	if status != "" {
		args = append(args, status)
		whereExtra = fmt.Sprintf(" AND status = $%d", len(args))
	}

	countQuery := `SELECT COUNT(*) FROM food_orders WHERE user_id = $1` + whereExtra
	var total int
	_ = r.db.QueryRowContext(ctx, countQuery, args[:len(args)-2+len(whereExtra)/10+1]...).Scan(&total)

	query := fmt.Sprintf(`
		SELECT id, user_id, merchant_id, driver_id, status, payment_method,
		       subtotal_cents, delivery_fee_cents, discount_cents, total_cents,
		       promo_code, delivery_lat, delivery_lng, delivery_address,
		       created_at, updated_at
		FROM food_orders WHERE user_id = $1%s
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, whereExtra)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []model.FoodOrder
	for rows.Next() {
		var o model.FoodOrder
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.MerchantID, &o.DriverID, &o.Status, &o.PaymentMethod,
			&o.SubtotalCents, &o.DeliveryFee, &o.DiscountCents, &o.TotalCents,
			&o.PromoCode, &o.DeliveryLat, &o.DeliveryLng, &o.DeliveryAddress,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}
	return orders, total, rows.Err()
}

func (r *FoodOrderRepository) ListByMerchant(ctx context.Context, merchantID string, status string, page, limit int) ([]model.FoodOrder, int, error) {
	offset := (page - 1) * limit
	args := []interface{}{merchantID}
	whereExtra := ""
	if status != "" {
		args = append(args, status)
		whereExtra = fmt.Sprintf(" AND status = $%d", len(args))
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, merchant_id, driver_id, status, payment_method,
		       subtotal_cents, delivery_fee_cents, discount_cents, total_cents,
		       delivery_lat, delivery_lng, delivery_address, created_at, updated_at
		FROM food_orders WHERE merchant_id = $1%s
		ORDER BY created_at ASC LIMIT $%d OFFSET $%d`,
		whereExtra, len(args)+1, len(args)+2)

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []model.FoodOrder
	for rows.Next() {
		var o model.FoodOrder
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.MerchantID, &o.DriverID, &o.Status, &o.PaymentMethod,
			&o.SubtotalCents, &o.DeliveryFee, &o.DiscountCents, &o.TotalCents,
			&o.DeliveryLat, &o.DeliveryLng, &o.DeliveryAddress, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}
	return orders, len(orders), rows.Err()
}

func (r *FoodOrderRepository) SaveFareBreakdown(ctx context.Context, fb *model.FareBreakdown) error {
	fb.ID = uuid.New().String()
	fb.CreatedAt = time.Now().UTC()
	query := `
		INSERT INTO food_fare_breakdown
		(id, order_id, subtotal_cents, delivery_fee_cents, service_fee_cents,
		 discount_cents, total_cents, distance_km, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.db.ExecContext(ctx, query,
		fb.ID, fb.OrderID, fb.SubtotalCents, fb.DeliveryFee, fb.ServiceFee,
		fb.DiscountCents, fb.TotalCents, fb.DistanceKm, fb.CreatedAt)
	return err
}

func (r *FoodOrderRepository) GetFareBreakdown(ctx context.Context, orderID string) (*model.FareBreakdown, error) {
	query := `
		SELECT id, order_id, subtotal_cents, delivery_fee_cents, service_fee_cents,
		       discount_cents, total_cents, distance_km, created_at
		FROM food_fare_breakdown WHERE order_id = $1`
	var fb model.FareBreakdown
	err := r.db.QueryRowContext(ctx, query, orderID).Scan(
		&fb.ID, &fb.OrderID, &fb.SubtotalCents, &fb.DeliveryFee, &fb.ServiceFee,
		&fb.DiscountCents, &fb.TotalCents, &fb.DistanceKm, &fb.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &fb, err
}

// LogStateTransition records a state change to food_order_state_logs.
func (r *FoodOrderRepository) LogStateTransition(ctx context.Context, log *model.FoodOrderStateLog) error {
	log.ID = uuid.New().String()
	log.CreatedAt = time.Now().UTC()
	query := `
		INSERT INTO food_order_state_logs (id, order_id, from_state, to_state, actor_id, actor_role, notes, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := r.db.ExecContext(ctx, query,
		log.ID, log.OrderID, log.FromState, log.ToState, log.ActorID, log.ActorRole, log.Notes, log.CreatedAt)
	return err
}

// ── MongoDB operations ───────────────────────────────────────────────────────

func (r *FoodOrderRepository) SaveItems(ctx context.Context, items []model.FoodOrderItem) error {
	docs := make([]interface{}, len(items))
	for i, item := range items {
		docs[i] = item
	}
	_, err := r.mongo.Collection("order_items").InsertMany(ctx, docs)
	return err
}

func (r *FoodOrderRepository) GetItemsByOrderID(ctx context.Context, orderID string) ([]model.FoodOrderItem, error) {
	cursor, err := r.mongo.Collection("order_items").Find(ctx,
		bson.M{"order_id": orderID},
		options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []model.FoodOrderItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// ── Redis operations ─────────────────────────────────────────────────────────

func (r *FoodOrderRepository) SetActiveOrder(ctx context.Context, userID, orderID string) error {
	key := activeOrderKeyPrefix + userID
	return r.redis.Set(ctx, key, orderID, activeOrderTTL).Err()
}

func (r *FoodOrderRepository) GetActiveOrderID(ctx context.Context, userID string) (string, error) {
	key := activeOrderKeyPrefix + userID
	val, err := r.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (r *FoodOrderRepository) ClearActiveOrder(ctx context.Context, userID string) error {
	return r.redis.Del(ctx, activeOrderKeyPrefix+userID).Err()
}

func (r *FoodOrderRepository) AddToMerchantQueue(ctx context.Context, merchantID, orderID string, score float64) error {
	key := merchantPendingSetPrefix + merchantID
	return r.redis.ZAdd(ctx, key, redis.Z{Score: score, Member: orderID}).Err()
}

func (r *FoodOrderRepository) RemoveFromMerchantQueue(ctx context.Context, merchantID, orderID string) error {
	key := merchantPendingSetPrefix + merchantID
	return r.redis.ZRem(ctx, key, orderID).Err()
}
