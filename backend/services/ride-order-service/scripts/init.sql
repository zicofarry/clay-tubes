-- ─────────────────────────────────────────────────────────────────────────────
-- Schema for clay-ride-order-service
-- Auto-applied on first container start by docker-compose.
-- ─────────────────────────────────────────────────────────────────────────────

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS ride_orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    driver_id       UUID,
    service_type    VARCHAR(20) NOT NULL CHECK (service_type IN ('goride','gocar')),
    vehicle_type    VARCHAR(20) NOT NULL CHECK (vehicle_type IN ('motor','car')),
    status          VARCHAR(30) NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','finding_driver','assigned','on_pickup','on_trip','completed','cancelled')),
    origin_lat      DECIMAL(10,7) NOT NULL,
    origin_lng      DECIMAL(10,7) NOT NULL,
    origin_address  TEXT,
    dest_lat        DECIMAL(10,7) NOT NULL,
    dest_lng        DECIMAL(10,7) NOT NULL,
    dest_address    TEXT,
    fare_estimate   DECIMAL(12,2),
    fare_final      DECIMAL(12,2),
    promo_id        UUID,
    payment_method  VARCHAR(20) NOT NULL CHECK (payment_method IN ('gopay','cash')),
    otp_code        VARCHAR(6),
    cancel_reason   TEXT,
    cancelled_by    VARCHAR(20) CHECK (cancelled_by IN ('user','driver','system')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ride_orders_user_id     ON ride_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_ride_orders_driver_id   ON ride_orders(driver_id);
CREATE INDEX IF NOT EXISTS idx_ride_orders_status      ON ride_orders(status);
CREATE INDEX IF NOT EXISTS idx_ride_orders_created_at  ON ride_orders(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ride_orders_user_status ON ride_orders(user_id, status);

CREATE TABLE IF NOT EXISTS order_state_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES ride_orders(id) ON DELETE CASCADE,
    from_state  VARCHAR(30),
    to_state    VARCHAR(30) NOT NULL,
    actor_id    UUID,
    actor_type  VARCHAR(20) NOT NULL CHECK (actor_type IN ('user','driver','system')),
    reason      TEXT,
    metadata    JSONB,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_state_logs_order_id ON order_state_logs(order_id);
CREATE INDEX IF NOT EXISTS idx_state_logs_changed  ON order_state_logs(changed_at DESC);

CREATE TABLE IF NOT EXISTS trip_details (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id             UUID NOT NULL UNIQUE REFERENCES ride_orders(id) ON DELETE CASCADE,
    polyline             TEXT,
    est_distance_km      DECIMAL(8,3),
    est_duration_min     INTEGER,
    actual_distance_km   DECIMAL(8,3),
    actual_duration_min  INTEGER,
    route_deviation_km   DECIMAL(8,3),
    pickup_time          TIMESTAMPTZ,
    dropoff_time         TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS order_fare_breakdown (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id          UUID NOT NULL UNIQUE REFERENCES ride_orders(id) ON DELETE CASCADE,
    base_fare         DECIMAL(12,2) NOT NULL,
    distance_fare     DECIMAL(12,2) NOT NULL,
    time_fare         DECIMAL(12,2) NOT NULL,
    surge_multiplier  DECIMAL(4,2)  NOT NULL DEFAULT 1.0,
    promo_discount    DECIMAL(12,2) NOT NULL DEFAULT 0,
    platform_fee      DECIMAL(12,2) NOT NULL DEFAULT 0,
    total             DECIMAL(12,2) NOT NULL
);
