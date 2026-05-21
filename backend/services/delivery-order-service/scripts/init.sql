-- ─────────────────────────────────────────────────────────────────────────────
-- Schema for clay-delivery-order-service
-- Auto-applied on first container start by docker-compose.
-- ─────────────────────────────────────────────────────────────────────────────

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS delivery_orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL,
    driver_id           UUID,
    status              VARCHAR(40) NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending','finding_driver','assigned','on_pickup','picked_up','on_delivery','delivered','cancelled')),
    sender_name         VARCHAR(100) NOT NULL,
    sender_phone        VARCHAR(20) NOT NULL,
    pickup_lat          DECIMAL(10,7) NOT NULL,
    pickup_lng          DECIMAL(10,7) NOT NULL,
    pickup_address      TEXT NOT NULL,
    pickup_notes        TEXT,
    recipient_name      VARCHAR(100) NOT NULL,
    recipient_phone     VARCHAR(20) NOT NULL,
    dest_lat            DECIMAL(10,7) NOT NULL,
    dest_lng            DECIMAL(10,7) NOT NULL,
    dest_address        TEXT NOT NULL,
    dest_notes          TEXT,
    fare_estimate       DECIMAL(12,2),
    fare_final          DECIMAL(12,2),
    promo_id            UUID,
    payment_method      VARCHAR(20) NOT NULL CHECK (payment_method IN ('gopay','cash')),
    cancel_reason       TEXT,
    cancelled_by        VARCHAR(20) CHECK (cancelled_by IN ('user','driver','system')),
    pickup_photo_url    TEXT,
    delivery_photo_url  TEXT,
    picked_up_at        TIMESTAMPTZ,
    delivered_at        TIMESTAMPTZ,
    actual_distance_km  DECIMAL(8,3),
    actual_duration_min INTEGER,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_delivery_orders_user_id     ON delivery_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_delivery_orders_driver_id   ON delivery_orders(driver_id);
CREATE INDEX IF NOT EXISTS idx_delivery_orders_status      ON delivery_orders(status);
CREATE INDEX IF NOT EXISTS idx_delivery_orders_created_at  ON delivery_orders(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_delivery_orders_user_status ON delivery_orders(user_id, status);

CREATE TABLE IF NOT EXISTS delivery_packages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL UNIQUE REFERENCES delivery_orders(id) ON DELETE CASCADE,
    category        VARCHAR(30) NOT NULL CHECK (category IN ('document','food','electronics','clothing','fragile','other')),
    weight_kg       DECIMAL(5,2),
    size            VARCHAR(20) NOT NULL CHECK (size IN ('small','medium','large')),
    is_fragile      BOOLEAN NOT NULL DEFAULT FALSE,
    description     TEXT,
    insurance_value DECIMAL(12,2),
    photo_url       TEXT
);
CREATE INDEX IF NOT EXISTS idx_delivery_packages_order_id ON delivery_packages(order_id);

CREATE TABLE IF NOT EXISTS delivery_state_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES delivery_orders(id) ON DELETE CASCADE,
    from_state  VARCHAR(40),
    to_state    VARCHAR(40) NOT NULL,
    actor_id    UUID,
    actor_type  VARCHAR(20) NOT NULL CHECK (actor_type IN ('user','driver','system')),
    reason      TEXT,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_delivery_state_logs_order_id   ON delivery_state_logs(order_id);
CREATE INDEX IF NOT EXISTS idx_delivery_state_logs_changed_at ON delivery_state_logs(changed_at DESC);

CREATE TABLE IF NOT EXISTS delivery_fare_breakdown (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID NOT NULL UNIQUE REFERENCES delivery_orders(id) ON DELETE CASCADE,
    base_fare        DECIMAL(12,2) NOT NULL,
    distance_fare    DECIMAL(12,2) NOT NULL,
    weight_surcharge DECIMAL(12,2) NOT NULL DEFAULT 0,
    insurance_fee    DECIMAL(12,2) NOT NULL DEFAULT 0,
    promo_discount   DECIMAL(12,2) NOT NULL DEFAULT 0,
    platform_fee     DECIMAL(12,2) NOT NULL DEFAULT 0,
    total            DECIMAL(12,2) NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_delivery_fare_breakdown_order_id ON delivery_fare_breakdown(order_id);
