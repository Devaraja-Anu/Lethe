-- Test Schema for Lethe Verification & Integration Tests

DROP TABLE IF EXISTS user_orders CASCADE;
DROP TABLE IF EXISTS user_sessions CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS support_tickets CASCADE;
DROP TABLE IF EXISTS composite_pk_table CASCADE;

-- 1. Parent table: users
CREATE TABLE users (
  id               BIGSERIAL PRIMARY KEY,
  name             TEXT NOT NULL,
  email            TEXT NOT NULL,
  phone            TEXT,
  kyc_verified     BOOLEAN NOT NULL DEFAULT false,
  last_activity_at TIMESTAMPTZ, -- Nullable trigger column
  anonymized_at    TIMESTAMPTZ, -- Marker column
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. Child 1: user_sessions (Covered FK with ON DELETE CASCADE)
CREATE TABLE user_sessions (
  id            BIGSERIAL PRIMARY KEY,
  user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  session_token TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3. Child 2: user_orders (Uncovered FK with ON DELETE RESTRICT to test preflight rejection)
CREATE TABLE user_orders (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  total_cents INT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4. Entity for Anonymization testing: support_tickets
CREATE TABLE support_tickets (
  id            BIGSERIAL PRIMARY KEY,
  email         TEXT,
  phone         TEXT,
  name          TEXT,
  details       TEXT,
  closed_at     TIMESTAMPTZ, -- Trigger column
  anonymized_at TIMESTAMPTZ  -- Marker column
);

-- 5. Composite Primary Key table (Used to test validation rejection in V1)
CREATE TABLE composite_pk_table (
  tenant_id    INT NOT NULL,
  item_id      INT NOT NULL,
  name         TEXT NOT NULL,
  last_used_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (tenant_id, item_id)
);
