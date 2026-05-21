// Package repository implements the data access layer for the Delivery Order Service.
package repository

import (
	"context"
	"database/sql"
	"time"
)

// ── Models ───────────────────────────────────────────────────────────────────

// DeliveryOrder represents a row in the `delivery_orders` table.
type DeliveryOrder struct {
	ID                string          `json:"id"`
	UserID            string          `json:"user_id"`
	DriverID          sql.NullString  `json:"driver_id,omitempty"`
	Status            string          `json:"status"`
	SenderName        string          `json:"sender_name"`
	SenderPhone       string          `json:"sender_phone"`
	PickupLat         float64         `json:"pickup_lat"`
	PickupLng         float64         `json:"pickup_lng"`
	PickupAddress     string          `json:"pickup_address"`
	PickupNotes       sql.NullString  `json:"pickup_notes,omitempty"`
	RecipientName     string          `json:"recipient_name"`
	RecipientPhone    string          `json:"recipient_phone"`
	DestLat           float64         `json:"dest_lat"`
	DestLng           float64         `json:"dest_lng"`
	DestAddress       string          `json:"dest_address"`
	DestNotes         sql.NullString  `json:"dest_notes,omitempty"`
	FareEstimate      sql.NullFloat64 `json:"fare_estimate,omitempty"`
	FareFinal         sql.NullFloat64 `json:"fare_final,omitempty"`
	PromoID           sql.NullString  `json:"promo_id,omitempty"`
	PaymentMethod     string          `json:"payment_method"`
	CancelReason      sql.NullString  `json:"cancel_reason,omitempty"`
	CancelledBy       sql.NullString  `json:"cancelled_by,omitempty"`
	PickupPhotoURL    sql.NullString  `json:"pickup_photo_url,omitempty"`
	DeliveryPhotoURL  sql.NullString  `json:"delivery_photo_url,omitempty"`
	PickedUpAt        sql.NullTime    `json:"picked_up_at,omitempty"`
	DeliveredAt       sql.NullTime    `json:"delivered_at,omitempty"`
	ActualDistanceKm  sql.NullFloat64 `json:"actual_distance_km,omitempty"`
	ActualDurationMin sql.NullInt32   `json:"actual_duration_min,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// DeliveryPackage represents a row in the `delivery_packages` table.
type DeliveryPackage struct {
	ID             string          `json:"id"`
	OrderID        string          `json:"order_id"`
	Category       string          `json:"category"`
	WeightKg       sql.NullFloat64 `json:"weight_kg,omitempty"`
	Size           string          `json:"size"`
	IsFragile      bool            `json:"is_fragile"`
	Description    sql.NullString  `json:"description,omitempty"`
	InsuranceValue sql.NullFloat64 `json:"insurance_value,omitempty"`
	PhotoURL       sql.NullString  `json:"photo_url,omitempty"`
}

// DeliveryStateLog represents a row in `delivery_state_logs`.
type DeliveryStateLog struct {
	ID        string         `json:"id"`
	OrderID   string         `json:"order_id"`
	FromState sql.NullString `json:"from_state,omitempty"`
	ToState   string         `json:"to_state"`
	ActorID   sql.NullString `json:"actor_id,omitempty"`
	ActorType string         `json:"actor_type"`
	Reason    sql.NullString `json:"reason,omitempty"`
	ChangedAt time.Time      `json:"changed_at"`
}

// DeliveryFareBreakdown represents a row in `delivery_fare_breakdown`.
type DeliveryFareBreakdown struct {
	ID              string  `json:"id"`
	OrderID         string  `json:"order_id"`
	BaseFare        float64 `json:"base_fare"`
	DistanceFare    float64 `json:"distance_fare"`
	WeightSurcharge float64 `json:"weight_surcharge"`
	InsuranceFee    float64 `json:"insurance_fee"`
	PromoDiscount   float64 `json:"promo_discount"`
	PlatformFee     float64 `json:"platform_fee"`
	Total           float64 `json:"total"`
}

// HistoryFilter holds filter options for ListUserHistory.
type HistoryFilter struct {
	Status string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

// ── Interface ────────────────────────────────────────────────────────────────

// DeliveryOrderRepositoryInterface defines the contract for delivery-order data access.
//
//go:generate mockgen -source=delivery_order_repository.go -destination=../../mocks/repomock/mock_delivery_order_repository.go -package=repomock
type DeliveryOrderRepositoryInterface interface {
	// Order + Package CRUD
	CreateOrder(ctx context.Context, o *DeliveryOrder, pkg *DeliveryPackage) (*DeliveryOrder, error)
	GetOrderByID(ctx context.Context, orderID string) (*DeliveryOrder, error)
	GetPackageByOrderID(ctx context.Context, orderID string) (*DeliveryPackage, error)
	GetActiveOrderByUserID(ctx context.Context, userID string) (*DeliveryOrder, error)
	ListUserHistory(ctx context.Context, userID string, f HistoryFilter) ([]*DeliveryOrder, int, error)

	// Status / driver assignment
	UpdateStatus(ctx context.Context, orderID, fromStatus, toStatus string) error
	AssignDriver(ctx context.Context, orderID, driverID string) error
	SetCancelled(ctx context.Context, orderID, reason, cancelledBy string) error
	SetFareFinal(ctx context.Context, orderID string, fareFinal float64) error
	SetPickupProof(ctx context.Context, orderID, photoURL string) error
	SetDeliveryDetails(ctx context.Context, orderID, photoURL string, distanceKm float64, durationMin int) error

	// Logs / fare
	InsertStateLog(ctx context.Context, log *DeliveryStateLog) error
	ListStateLogs(ctx context.Context, orderID string) ([]*DeliveryStateLog, error)
	UpsertFareBreakdown(ctx context.Context, fb *DeliveryFareBreakdown) error
	GetFareBreakdown(ctx context.Context, orderID string) (*DeliveryFareBreakdown, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

// DeliveryOrderRepository implements DeliveryOrderRepositoryInterface using PostgreSQL.
type DeliveryOrderRepository struct {
	db    *sql.DB
	redis interface{} // TODO: replace with *redis.Client once Redis client adopted
}

// NewDeliveryOrderRepository creates a new DeliveryOrderRepository.
func NewDeliveryOrderRepository(db *sql.DB, redis interface{}) *DeliveryOrderRepository {
	return &DeliveryOrderRepository{db: db, redis: redis}
}

// ── Order + Package CRUD ─────────────────────────────────────────────────────

func (r *DeliveryOrderRepository) CreateOrder(ctx context.Context, o *DeliveryOrder, pkg *DeliveryPackage) (*DeliveryOrder, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const orderQ = `
		INSERT INTO delivery_orders (
			user_id, status,
			sender_name, sender_phone,
			pickup_lat, pickup_lng, pickup_address, pickup_notes,
			recipient_name, recipient_phone,
			dest_lat, dest_lng, dest_address, dest_notes,
			fare_estimate, promo_id, payment_method
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		RETURNING id, status, created_at, updated_at
	`
	err = tx.QueryRowContext(ctx, orderQ,
		o.UserID, o.Status,
		o.SenderName, o.SenderPhone,
		o.PickupLat, o.PickupLng, o.PickupAddress, o.PickupNotes,
		o.RecipientName, o.RecipientPhone,
		o.DestLat, o.DestLng, o.DestAddress, o.DestNotes,
		o.FareEstimate, o.PromoID, o.PaymentMethod,
	).Scan(&o.ID, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}

	const pkgQ = `
		INSERT INTO delivery_packages (order_id, category, weight_kg, size, is_fragile, description, insurance_value)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id
	`
	pkg.OrderID = o.ID
	err = tx.QueryRowContext(ctx, pkgQ,
		pkg.OrderID, pkg.Category, pkg.WeightKg, pkg.Size,
		pkg.IsFragile, pkg.Description, pkg.InsuranceValue,
	).Scan(&pkg.ID)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return o, nil
}

func (r *DeliveryOrderRepository) GetOrderByID(ctx context.Context, orderID string) (*DeliveryOrder, error) {
	const q = `
		SELECT id, user_id, driver_id, status,
		       sender_name, sender_phone,
		       pickup_lat, pickup_lng, pickup_address, pickup_notes,
		       recipient_name, recipient_phone,
		       dest_lat, dest_lng, dest_address, dest_notes,
		       fare_estimate, fare_final, promo_id, payment_method,
		       cancel_reason, cancelled_by,
		       pickup_photo_url, delivery_photo_url,
		       picked_up_at, delivered_at,
		       actual_distance_km, actual_duration_min,
		       created_at, updated_at
		FROM delivery_orders WHERE id = $1
	`
	o := &DeliveryOrder{}
	err := r.db.QueryRowContext(ctx, q, orderID).Scan(
		&o.ID, &o.UserID, &o.DriverID, &o.Status,
		&o.SenderName, &o.SenderPhone,
		&o.PickupLat, &o.PickupLng, &o.PickupAddress, &o.PickupNotes,
		&o.RecipientName, &o.RecipientPhone,
		&o.DestLat, &o.DestLng, &o.DestAddress, &o.DestNotes,
		&o.FareEstimate, &o.FareFinal, &o.PromoID, &o.PaymentMethod,
		&o.CancelReason, &o.CancelledBy,
		&o.PickupPhotoURL, &o.DeliveryPhotoURL,
		&o.PickedUpAt, &o.DeliveredAt,
		&o.ActualDistanceKm, &o.ActualDurationMin,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *DeliveryOrderRepository) GetPackageByOrderID(ctx context.Context, orderID string) (*DeliveryPackage, error) {
	const q = `
		SELECT id, order_id, category, weight_kg, size, is_fragile, description, insurance_value, photo_url
		FROM delivery_packages WHERE order_id = $1
	`
	pkg := &DeliveryPackage{}
	err := r.db.QueryRowContext(ctx, q, orderID).Scan(
		&pkg.ID, &pkg.OrderID, &pkg.Category, &pkg.WeightKg,
		&pkg.Size, &pkg.IsFragile, &pkg.Description, &pkg.InsuranceValue, &pkg.PhotoURL,
	)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

func (r *DeliveryOrderRepository) GetActiveOrderByUserID(ctx context.Context, userID string) (*DeliveryOrder, error) {
	const q = `
		SELECT id, user_id, driver_id, status,
		       sender_name, sender_phone,
		       pickup_lat, pickup_lng, pickup_address, pickup_notes,
		       recipient_name, recipient_phone,
		       dest_lat, dest_lng, dest_address, dest_notes,
		       fare_estimate, fare_final, promo_id, payment_method,
		       cancel_reason, cancelled_by,
		       pickup_photo_url, delivery_photo_url,
		       picked_up_at, delivered_at,
		       actual_distance_km, actual_duration_min,
		       created_at, updated_at
		FROM delivery_orders
		WHERE user_id = $1
		  AND status NOT IN ('delivered','cancelled')
		ORDER BY created_at DESC
		LIMIT 1
	`
	o := &DeliveryOrder{}
	err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&o.ID, &o.UserID, &o.DriverID, &o.Status,
		&o.SenderName, &o.SenderPhone,
		&o.PickupLat, &o.PickupLng, &o.PickupAddress, &o.PickupNotes,
		&o.RecipientName, &o.RecipientPhone,
		&o.DestLat, &o.DestLng, &o.DestAddress, &o.DestNotes,
		&o.FareEstimate, &o.FareFinal, &o.PromoID, &o.PaymentMethod,
		&o.CancelReason, &o.CancelledBy,
		&o.PickupPhotoURL, &o.DeliveryPhotoURL,
		&o.PickedUpAt, &o.DeliveredAt,
		&o.ActualDistanceKm, &o.ActualDurationMin,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *DeliveryOrderRepository) ListUserHistory(ctx context.Context, userID string, f HistoryFilter) ([]*DeliveryOrder, int, error) {
	const countQ = `
		SELECT COUNT(1) FROM delivery_orders
		WHERE user_id = $1
		  AND ($2 = '' OR status = $2)
		  AND ($3::timestamptz IS NULL OR created_at >= $3)
		  AND ($4::timestamptz IS NULL OR created_at <= $4)
	`
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, userID, f.Status, f.From, f.To).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `
		SELECT id, user_id, driver_id, status,
		       sender_name, sender_phone,
		       pickup_lat, pickup_lng, pickup_address, pickup_notes,
		       recipient_name, recipient_phone,
		       dest_lat, dest_lng, dest_address, dest_notes,
		       fare_estimate, fare_final, promo_id, payment_method,
		       cancel_reason, cancelled_by,
		       pickup_photo_url, delivery_photo_url,
		       picked_up_at, delivered_at,
		       actual_distance_km, actual_duration_min,
		       created_at, updated_at
		FROM delivery_orders
		WHERE user_id = $1
		  AND ($2 = '' OR status = $2)
		  AND ($3::timestamptz IS NULL OR created_at >= $3)
		  AND ($4::timestamptz IS NULL OR created_at <= $4)
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6
	`
	rows, err := r.db.QueryContext(ctx, q, userID, f.Status, f.From, f.To, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*DeliveryOrder
	for rows.Next() {
		o := &DeliveryOrder{}
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.DriverID, &o.Status,
			&o.SenderName, &o.SenderPhone,
			&o.PickupLat, &o.PickupLng, &o.PickupAddress, &o.PickupNotes,
			&o.RecipientName, &o.RecipientPhone,
			&o.DestLat, &o.DestLng, &o.DestAddress, &o.DestNotes,
			&o.FareEstimate, &o.FareFinal, &o.PromoID, &o.PaymentMethod,
			&o.CancelReason, &o.CancelledBy,
			&o.PickupPhotoURL, &o.DeliveryPhotoURL,
			&o.PickedUpAt, &o.DeliveredAt,
			&o.ActualDistanceKm, &o.ActualDurationMin,
			&o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, o)
	}
	return out, total, rows.Err()
}

// ── Status / driver assignment ──────────────────────────────────────────────

// UpdateStatus performs an optimistic-concurrency status transition.
// Returns sql.ErrNoRows if the order is not in fromStatus.
func (r *DeliveryOrderRepository) UpdateStatus(ctx context.Context, orderID, fromStatus, toStatus string) error {
	const q = `UPDATE delivery_orders SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3`
	res, err := r.db.ExecContext(ctx, q, toStatus, orderID, fromStatus)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// AssignDriver atomically moves order from finding_driver -> assigned and stamps driver_id.
// Delivery orders use photo proof instead of OTP, so no otp_code is generated here.
func (r *DeliveryOrderRepository) AssignDriver(ctx context.Context, orderID, driverID string) error {
	const q = `
		UPDATE delivery_orders
		SET driver_id = $1, status = 'assigned', updated_at = NOW()
		WHERE id = $2 AND status = 'finding_driver'
	`
	res, err := r.db.ExecContext(ctx, q, driverID, orderID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *DeliveryOrderRepository) SetCancelled(ctx context.Context, orderID, reason, cancelledBy string) error {
	const q = `
		UPDATE delivery_orders
		SET status = 'cancelled', cancel_reason = $1, cancelled_by = $2, updated_at = NOW()
		WHERE id = $3 AND status NOT IN ('picked_up','on_delivery','delivered','cancelled')
	`
	res, err := r.db.ExecContext(ctx, q, reason, cancelledBy, orderID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *DeliveryOrderRepository) SetFareFinal(ctx context.Context, orderID string, fareFinal float64) error {
	const q = `UPDATE delivery_orders SET fare_final = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, q, fareFinal, orderID)
	return err
}

func (r *DeliveryOrderRepository) SetPickupProof(ctx context.Context, orderID, photoURL string) error {
	const q = `
		UPDATE delivery_orders
		SET pickup_photo_url = $1, picked_up_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, q, photoURL, orderID)
	return err
}

func (r *DeliveryOrderRepository) SetDeliveryDetails(ctx context.Context, orderID, photoURL string, distanceKm float64, durationMin int) error {
	const q = `
		UPDATE delivery_orders
		SET delivery_photo_url = $1, delivered_at = NOW(),
		    actual_distance_km = $2, actual_duration_min = $3,
		    updated_at = NOW()
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, q, photoURL, distanceKm, durationMin, orderID)
	return err
}

// ── Logs / fare ──────────────────────────────────────────────────────────────

func (r *DeliveryOrderRepository) InsertStateLog(ctx context.Context, l *DeliveryStateLog) error {
	const q = `
		INSERT INTO delivery_state_logs (order_id, from_state, to_state, actor_id, actor_type, reason)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, changed_at
	`
	return r.db.QueryRowContext(ctx, q,
		l.OrderID, l.FromState, l.ToState, l.ActorID, l.ActorType, l.Reason,
	).Scan(&l.ID, &l.ChangedAt)
}

func (r *DeliveryOrderRepository) ListStateLogs(ctx context.Context, orderID string) ([]*DeliveryStateLog, error) {
	const q = `
		SELECT id, order_id, from_state, to_state, actor_id, actor_type, reason, changed_at
		FROM delivery_state_logs WHERE order_id = $1 ORDER BY changed_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*DeliveryStateLog
	for rows.Next() {
		l := &DeliveryStateLog{}
		if err := rows.Scan(
			&l.ID, &l.OrderID, &l.FromState, &l.ToState,
			&l.ActorID, &l.ActorType, &l.Reason, &l.ChangedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *DeliveryOrderRepository) UpsertFareBreakdown(ctx context.Context, fb *DeliveryFareBreakdown) error {
	const q = `
		INSERT INTO delivery_fare_breakdown (
			order_id, base_fare, distance_fare, weight_surcharge,
			insurance_fee, promo_discount, platform_fee, total
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (order_id) DO UPDATE SET
			base_fare        = EXCLUDED.base_fare,
			distance_fare    = EXCLUDED.distance_fare,
			weight_surcharge = EXCLUDED.weight_surcharge,
			insurance_fee    = EXCLUDED.insurance_fee,
			promo_discount   = EXCLUDED.promo_discount,
			platform_fee     = EXCLUDED.platform_fee,
			total            = EXCLUDED.total
	`
	_, err := r.db.ExecContext(ctx, q,
		fb.OrderID, fb.BaseFare, fb.DistanceFare, fb.WeightSurcharge,
		fb.InsuranceFee, fb.PromoDiscount, fb.PlatformFee, fb.Total,
	)
	return err
}

func (r *DeliveryOrderRepository) GetFareBreakdown(ctx context.Context, orderID string) (*DeliveryFareBreakdown, error) {
	const q = `
		SELECT id, order_id, base_fare, distance_fare, weight_surcharge,
		       insurance_fee, promo_discount, platform_fee, total
		FROM delivery_fare_breakdown WHERE order_id = $1
	`
	fb := &DeliveryFareBreakdown{}
	err := r.db.QueryRowContext(ctx, q, orderID).Scan(
		&fb.ID, &fb.OrderID, &fb.BaseFare, &fb.DistanceFare, &fb.WeightSurcharge,
		&fb.InsuranceFee, &fb.PromoDiscount, &fb.PlatformFee, &fb.Total,
	)
	if err != nil {
		return nil, err
	}
	return fb, nil
}
