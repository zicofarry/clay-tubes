// Package repository implements the data access layer for the Ride Order Service.
package repository

import (
	"context"
	"database/sql"
	"time"
)

// ── Models ───────────────────────────────────────────────────────────────────

// RideOrder represents a row in the `ride_orders` table.
type RideOrder struct {
	ID             string         `json:"id"`
	UserID         string         `json:"user_id"`
	DriverID       sql.NullString `json:"driver_id,omitempty"`
	ServiceType    string         `json:"service_type"` // goride | gocar
	VehicleType    string         `json:"vehicle_type"` // motor | car
	Status         string         `json:"status"`
	OriginLat      float64        `json:"origin_lat"`
	OriginLng      float64        `json:"origin_lng"`
	OriginAddress  sql.NullString `json:"origin_address,omitempty"`
	DestLat        float64        `json:"dest_lat"`
	DestLng        float64        `json:"dest_lng"`
	DestAddress    sql.NullString `json:"dest_address,omitempty"`
	FareEstimate   sql.NullFloat64 `json:"fare_estimate,omitempty"`
	FareFinal      sql.NullFloat64 `json:"fare_final,omitempty"`
	PromoID        sql.NullString `json:"promo_id,omitempty"`
	PaymentMethod  string         `json:"payment_method"` // gopay | cash
	OTPCode        sql.NullString `json:"otp_code,omitempty"`
	CancelReason   sql.NullString `json:"cancel_reason,omitempty"`
	CancelledBy    sql.NullString `json:"cancelled_by,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// OrderStateLog represents a row in `order_state_logs`.
type OrderStateLog struct {
	ID        string         `json:"id"`
	OrderID   string         `json:"order_id"`
	FromState sql.NullString `json:"from_state"`
	ToState   string         `json:"to_state"`
	ActorID   sql.NullString `json:"actor_id"`
	ActorType string         `json:"actor_type"` // user | driver | sys
	Reason    sql.NullString `json:"reason"`
	Metadata  []byte         `json:"metadata"` // JSONB
	ChangedAt time.Time      `json:"changed_at"`
}

// TripDetails represents a row in `trip_details`.
type TripDetails struct {
	ID                 string          `json:"id"`
	OrderID            string          `json:"order_id"`
	Polyline           sql.NullString  `json:"polyline"`
	EstDistanceKm      sql.NullFloat64 `json:"est_distance_km"`
	EstDurationMin     sql.NullInt32   `json:"est_duration_min"`
	ActualDistanceKm   sql.NullFloat64 `json:"actual_distance_km"`
	ActualDurationMin  sql.NullInt32   `json:"actual_duration_min"`
	RouteDeviationKm   sql.NullFloat64 `json:"route_deviation_km"`
	PickupTime         sql.NullTime    `json:"pickup_time"`
	DropoffTime        sql.NullTime    `json:"dropoff_time"`
}

// FareBreakdown represents a row in `order_fare_breakdown`.
type FareBreakdown struct {
	ID              string  `json:"id"`
	OrderID         string  `json:"order_id"`
	BaseFare        float64 `json:"base_fare"`
	DistanceFare    float64 `json:"distance_fare"`
	TimeFare        float64 `json:"time_fare"`
	SurgeMultiplier float64 `json:"surge_multiplier"`
	PromoDiscount   float64 `json:"promo_discount"`
	PlatformFee     float64 `json:"platform_fee"`
	Total           float64 `json:"total"`
}

// HistoryFilter holds filter options for ListUserHistory.
type HistoryFilter struct {
	Status      string
	ServiceType string
	From        *time.Time
	To          *time.Time
	Limit       int
	Offset      int
}

// ── Interface ────────────────────────────────────────────────────────────────

// RideOrderRepositoryInterface defines the contract for ride-order data access.
//
//go:generate mockgen -source=ride_order_repository.go -destination=../../mocks/repomock/mock_ride_order_repository.go -package=repomock
type RideOrderRepositoryInterface interface {
	// Order CRUD
	CreateOrder(ctx context.Context, order *RideOrder) (*RideOrder, error)
	GetOrderByID(ctx context.Context, orderID string) (*RideOrder, error)
	GetActiveOrderByUserID(ctx context.Context, userID string) (*RideOrder, error)
	ListUserHistory(ctx context.Context, userID string, f HistoryFilter) ([]*RideOrder, int, error)

	// Status / driver assignment
	UpdateStatus(ctx context.Context, orderID, fromStatus, toStatus string) error
	AssignDriver(ctx context.Context, orderID, driverID, otpCode string) error
	SetCancelled(ctx context.Context, orderID, reason, cancelledBy string) error
	SetFareFinal(ctx context.Context, orderID string, fareFinal float64) error

	// Logs / details / fare
	InsertStateLog(ctx context.Context, log *OrderStateLog) error
	ListStateLogs(ctx context.Context, orderID string) ([]*OrderStateLog, error)
	UpsertTripDetails(ctx context.Context, td *TripDetails) error
	GetTripDetails(ctx context.Context, orderID string) (*TripDetails, error)
	UpsertFareBreakdown(ctx context.Context, fb *FareBreakdown) error
	GetFareBreakdown(ctx context.Context, orderID string) (*FareBreakdown, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

// RideOrderRepository implements RideOrderRepositoryInterface using PostgreSQL.
type RideOrderRepository struct {
	db    *sql.DB
	redis interface{} // TODO: replace with *redis.Client once Redis client adopted
}

// NewRideOrderRepository creates a new RideOrderRepository.
func NewRideOrderRepository(db *sql.DB, redis interface{}) *RideOrderRepository {
	return &RideOrderRepository{db: db, redis: redis}
}

// ── Order CRUD ──────────────────────────────────────────────────────────────

func (r *RideOrderRepository) CreateOrder(ctx context.Context, o *RideOrder) (*RideOrder, error) {
	const q = `
		INSERT INTO ride_orders (
			user_id, service_type, vehicle_type, status,
			origin_lat, origin_lng, origin_address,
			dest_lat, dest_lng, dest_address,
			fare_estimate, promo_id, payment_method
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, status, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, q,
		o.UserID, o.ServiceType, o.VehicleType, o.Status,
		o.OriginLat, o.OriginLng, o.OriginAddress,
		o.DestLat, o.DestLng, o.DestAddress,
		o.FareEstimate, o.PromoID, o.PaymentMethod,
	).Scan(&o.ID, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *RideOrderRepository) GetOrderByID(ctx context.Context, orderID string) (*RideOrder, error) {
	const q = `
		SELECT id, user_id, driver_id, service_type, vehicle_type, status,
		       origin_lat, origin_lng, origin_address,
		       dest_lat, dest_lng, dest_address,
		       fare_estimate, fare_final, promo_id, payment_method,
		       otp_code, cancel_reason, cancelled_by,
		       created_at, updated_at
		FROM ride_orders WHERE id = $1
	`
	o := &RideOrder{}
	err := r.db.QueryRowContext(ctx, q, orderID).Scan(
		&o.ID, &o.UserID, &o.DriverID, &o.ServiceType, &o.VehicleType, &o.Status,
		&o.OriginLat, &o.OriginLng, &o.OriginAddress,
		&o.DestLat, &o.DestLng, &o.DestAddress,
		&o.FareEstimate, &o.FareFinal, &o.PromoID, &o.PaymentMethod,
		&o.OTPCode, &o.CancelReason, &o.CancelledBy,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *RideOrderRepository) GetActiveOrderByUserID(ctx context.Context, userID string) (*RideOrder, error) {
	const q = `
		SELECT id, user_id, driver_id, service_type, vehicle_type, status,
		       origin_lat, origin_lng, origin_address,
		       dest_lat, dest_lng, dest_address,
		       fare_estimate, fare_final, promo_id, payment_method,
		       otp_code, cancel_reason, cancelled_by,
		       created_at, updated_at
		FROM ride_orders
		WHERE user_id = $1
		  AND status NOT IN ('completed','cancelled')
		ORDER BY created_at DESC
		LIMIT 1
	`
	o := &RideOrder{}
	err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&o.ID, &o.UserID, &o.DriverID, &o.ServiceType, &o.VehicleType, &o.Status,
		&o.OriginLat, &o.OriginLng, &o.OriginAddress,
		&o.DestLat, &o.DestLng, &o.DestAddress,
		&o.FareEstimate, &o.FareFinal, &o.PromoID, &o.PaymentMethod,
		&o.OTPCode, &o.CancelReason, &o.CancelledBy,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *RideOrderRepository) ListUserHistory(ctx context.Context, userID string, f HistoryFilter) ([]*RideOrder, int, error) {
	// Count first
	const countQ = `
		SELECT COUNT(1) FROM ride_orders
		WHERE user_id = $1
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR service_type = $3)
		  AND ($4::timestamptz IS NULL OR created_at >= $4)
		  AND ($5::timestamptz IS NULL OR created_at <= $5)
	`
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, userID, f.Status, f.ServiceType, f.From, f.To).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `
		SELECT id, user_id, driver_id, service_type, vehicle_type, status,
		       origin_lat, origin_lng, origin_address,
		       dest_lat, dest_lng, dest_address,
		       fare_estimate, fare_final, promo_id, payment_method,
		       otp_code, cancel_reason, cancelled_by,
		       created_at, updated_at
		FROM ride_orders
		WHERE user_id = $1
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR service_type = $3)
		  AND ($4::timestamptz IS NULL OR created_at >= $4)
		  AND ($5::timestamptz IS NULL OR created_at <= $5)
		ORDER BY created_at DESC
		LIMIT $6 OFFSET $7
	`
	rows, err := r.db.QueryContext(ctx, q, userID, f.Status, f.ServiceType, f.From, f.To, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*RideOrder
	for rows.Next() {
		o := &RideOrder{}
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.DriverID, &o.ServiceType, &o.VehicleType, &o.Status,
			&o.OriginLat, &o.OriginLng, &o.OriginAddress,
			&o.DestLat, &o.DestLng, &o.DestAddress,
			&o.FareEstimate, &o.FareFinal, &o.PromoID, &o.PaymentMethod,
			&o.OTPCode, &o.CancelReason, &o.CancelledBy,
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
func (r *RideOrderRepository) UpdateStatus(ctx context.Context, orderID, fromStatus, toStatus string) error {
	const q = `
		UPDATE ride_orders SET status = $1, updated_at = NOW()
		WHERE id = $2 AND status = $3
	`
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

// AssignDriver atomically moves order from finding_driver -> assigned and stamps driver_id + otp_code.
func (r *RideOrderRepository) AssignDriver(ctx context.Context, orderID, driverID, otpCode string) error {
	const q = `
		UPDATE ride_orders
		SET driver_id = $1, otp_code = $2, status = 'assigned', updated_at = NOW()
		WHERE id = $3 AND status = 'finding_driver'
	`
	res, err := r.db.ExecContext(ctx, q, driverID, otpCode, orderID)
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

func (r *RideOrderRepository) SetCancelled(ctx context.Context, orderID, reason, cancelledBy string) error {
	const q = `
		UPDATE ride_orders
		SET status = 'cancelled', cancel_reason = $1, cancelled_by = $2, updated_at = NOW()
		WHERE id = $3 AND status NOT IN ('completed','cancelled','on_trip')
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

func (r *RideOrderRepository) SetFareFinal(ctx context.Context, orderID string, fareFinal float64) error {
	const q = `UPDATE ride_orders SET fare_final = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, q, fareFinal, orderID)
	return err
}

// ── State logs / trip details / fare ────────────────────────────────────────

func (r *RideOrderRepository) InsertStateLog(ctx context.Context, l *OrderStateLog) error {
	const q = `
		INSERT INTO order_state_logs (order_id, from_state, to_state, actor_id, actor_type, reason, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, changed_at
	`
	return r.db.QueryRowContext(ctx, q,
		l.OrderID, l.FromState, l.ToState, l.ActorID, l.ActorType, l.Reason, l.Metadata,
	).Scan(&l.ID, &l.ChangedAt)
}

func (r *RideOrderRepository) ListStateLogs(ctx context.Context, orderID string) ([]*OrderStateLog, error) {
	const q = `
		SELECT id, order_id, from_state, to_state, actor_id, actor_type, reason, metadata, changed_at
		FROM order_state_logs WHERE order_id = $1 ORDER BY changed_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*OrderStateLog
	for rows.Next() {
		l := &OrderStateLog{}
		if err := rows.Scan(
			&l.ID, &l.OrderID, &l.FromState, &l.ToState,
			&l.ActorID, &l.ActorType, &l.Reason, &l.Metadata, &l.ChangedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *RideOrderRepository) UpsertTripDetails(ctx context.Context, td *TripDetails) error {
	const q = `
		INSERT INTO trip_details (
			order_id, polyline, est_distance_km, est_duration_min,
			actual_distance_km, actual_duration_min, route_deviation_km,
			pickup_time, dropoff_time
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (order_id) DO UPDATE SET
			polyline             = COALESCE(EXCLUDED.polyline, trip_details.polyline),
			est_distance_km      = COALESCE(EXCLUDED.est_distance_km, trip_details.est_distance_km),
			est_duration_min     = COALESCE(EXCLUDED.est_duration_min, trip_details.est_duration_min),
			actual_distance_km   = COALESCE(EXCLUDED.actual_distance_km, trip_details.actual_distance_km),
			actual_duration_min  = COALESCE(EXCLUDED.actual_duration_min, trip_details.actual_duration_min),
			route_deviation_km   = COALESCE(EXCLUDED.route_deviation_km, trip_details.route_deviation_km),
			pickup_time          = COALESCE(EXCLUDED.pickup_time, trip_details.pickup_time),
			dropoff_time         = COALESCE(EXCLUDED.dropoff_time, trip_details.dropoff_time)
	`
	_, err := r.db.ExecContext(ctx, q,
		td.OrderID, td.Polyline, td.EstDistanceKm, td.EstDurationMin,
		td.ActualDistanceKm, td.ActualDurationMin, td.RouteDeviationKm,
		td.PickupTime, td.DropoffTime,
	)
	return err
}

func (r *RideOrderRepository) GetTripDetails(ctx context.Context, orderID string) (*TripDetails, error) {
	const q = `
		SELECT id, order_id, polyline, est_distance_km, est_duration_min,
		       actual_distance_km, actual_duration_min, route_deviation_km,
		       pickup_time, dropoff_time
		FROM trip_details WHERE order_id = $1
	`
	td := &TripDetails{}
	err := r.db.QueryRowContext(ctx, q, orderID).Scan(
		&td.ID, &td.OrderID, &td.Polyline, &td.EstDistanceKm, &td.EstDurationMin,
		&td.ActualDistanceKm, &td.ActualDurationMin, &td.RouteDeviationKm,
		&td.PickupTime, &td.DropoffTime,
	)
	if err != nil {
		return nil, err
	}
	return td, nil
}

func (r *RideOrderRepository) UpsertFareBreakdown(ctx context.Context, fb *FareBreakdown) error {
	const q = `
		INSERT INTO order_fare_breakdown (
			order_id, base_fare, distance_fare, time_fare,
			surge_multiplier, promo_discount, platform_fee, total
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (order_id) DO UPDATE SET
			base_fare        = EXCLUDED.base_fare,
			distance_fare    = EXCLUDED.distance_fare,
			time_fare        = EXCLUDED.time_fare,
			surge_multiplier = EXCLUDED.surge_multiplier,
			promo_discount   = EXCLUDED.promo_discount,
			platform_fee     = EXCLUDED.platform_fee,
			total            = EXCLUDED.total
	`
	_, err := r.db.ExecContext(ctx, q,
		fb.OrderID, fb.BaseFare, fb.DistanceFare, fb.TimeFare,
		fb.SurgeMultiplier, fb.PromoDiscount, fb.PlatformFee, fb.Total,
	)
	return err
}

func (r *RideOrderRepository) GetFareBreakdown(ctx context.Context, orderID string) (*FareBreakdown, error) {
	const q = `
		SELECT id, order_id, base_fare, distance_fare, time_fare,
		       surge_multiplier, promo_discount, platform_fee, total
		FROM order_fare_breakdown WHERE order_id = $1
	`
	fb := &FareBreakdown{}
	err := r.db.QueryRowContext(ctx, q, orderID).Scan(
		&fb.ID, &fb.OrderID, &fb.BaseFare, &fb.DistanceFare, &fb.TimeFare,
		&fb.SurgeMultiplier, &fb.PromoDiscount, &fb.PlatformFee, &fb.Total,
	)
	if err != nil {
		return nil, err
	}
	return fb, nil
}
