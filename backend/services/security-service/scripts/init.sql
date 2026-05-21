-- ─────────────────────────────────────────────────────────────────────────────
-- Schema for clay-security-service
-- Auto-applied on first container start by docker-compose.
-- ─────────────────────────────────────────────────────────────────────────────

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS login_attempts (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL,
    ip_address     VARCHAR(45) NOT NULL,
    user_agent     VARCHAR(500),
    success        BOOLEAN NOT NULL,
    failure_reason VARCHAR(100),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_login_attempts_user_id      ON login_attempts(user_id);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip_address   ON login_attempts(ip_address);
CREATE INDEX IF NOT EXISTS idx_login_attempts_user_created ON login_attempts(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip_created   ON login_attempts(ip_address, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_attempts_success      ON login_attempts(success, created_at DESC);

CREATE TABLE IF NOT EXISTS fraud_flags (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    flag_type       VARCHAR(50) NOT NULL,
    severity        VARCHAR(20) NOT NULL CHECK (severity IN ('low','medium','high','critical')),
    description     TEXT,
    source          VARCHAR(50),
    resolved        BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_by     UUID,
    resolved_at     TIMESTAMPTZ,
    resolution_note TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fraud_flags_user_id           ON fraud_flags(user_id);
CREATE INDEX IF NOT EXISTS idx_fraud_flags_user_resolved     ON fraud_flags(user_id, resolved);
CREATE INDEX IF NOT EXISTS idx_fraud_flags_severity_resolved ON fraud_flags(severity, resolved);
CREATE INDEX IF NOT EXISTS idx_fraud_flags_flag_type         ON fraud_flags(flag_type, created_at DESC);

CREATE TABLE IF NOT EXISTS ip_blacklist (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address  VARCHAR(45) NOT NULL UNIQUE,
    reason      TEXT,
    blocked_by  UUID,
    expires_at  TIMESTAMPTZ,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ip_blacklist_ip_address ON ip_blacklist(ip_address);
CREATE INDEX IF NOT EXISTS idx_ip_blacklist_is_active  ON ip_blacklist(is_active, expires_at);
CREATE INDEX IF NOT EXISTS idx_ip_blacklist_created_at ON ip_blacklist(created_at DESC);
