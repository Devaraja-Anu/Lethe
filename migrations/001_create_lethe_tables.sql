-- Lethe Migration 001: Core metadata and audit tables

CREATE TABLE IF NOT EXISTS lethe_holds (
  id          BIGSERIAL PRIMARY KEY,
  entity      TEXT NOT NULL,
  record_id   TEXT NOT NULL,
  reason      TEXT NOT NULL,
  basis       TEXT,                 -- e.g. 'RBI KYC' (free text in V1)
  until       TIMESTAMPTZ,          -- NULL = indefinite
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  released_at TIMESTAMPTZ
);

-- Partial index for active holds lookup during retention batch processing
CREATE INDEX IF NOT EXISTS idx_lethe_holds_active
  ON lethe_holds (entity, record_id)
  WHERE released_at IS NULL;

CREATE TABLE IF NOT EXISTS lethe_audit (
  id            BIGSERIAL PRIMARY KEY,
  run_id        UUID NOT NULL,
  entity        TEXT NOT NULL,
  action        TEXT NOT NULL,
  rows_affected INT NOT NULL,
  rows_held     INT NOT NULL DEFAULT 0,
  cutoff        TIMESTAMPTZ NOT NULL,
  policy_hash   TEXT NOT NULL,
  dry_run       BOOLEAN NOT NULL,
  error         TEXT,
  started_at    TIMESTAMPTZ NOT NULL,
  finished_at   TIMESTAMPTZ NOT NULL
);

-- Index audit log by run_id to correlate batches across a run
CREATE INDEX IF NOT EXISTS idx_lethe_audit_run_id
  ON lethe_audit (run_id);
