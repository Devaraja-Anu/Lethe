# Lethe Architecture & Design Decisions

This document records the architectural and design decisions made during the planning and implementation of Lethe, including context, options considered, decisions taken, and rationale.

---

## Decision 1: Anonymization Idempotency via Marker Column

- **Status:** Accepted (V1)
- **Context:**
  When a row is anonymized, its trigger timestamp (e.g. `last_activity_at`) remains older than the retention cutoff. Without a state change, subsequent runs would repeatedly select and re-anonymize the same rows.
- **Options Considered:**
  1. *Marker column on the target table (e.g. `anonymized_at TIMESTAMPTZ`).*
  2. *Internal tracking table (`lethe_anonymized(entity, record_id, anonymized_at)`).*
- **Decision:**
  Adopt Option 1 (marker column) for V1.
- **Details & Operational Rules:**
  - **Atomic UPDATE:** The marker column must be updated in the exact same `UPDATE` statement that redacts the personal data fields. This prevents partial state in the event of a crash.
  - **Selector Predicate:** The candidate query appends `AND <anonymized_marker> IS NULL`.
  - **Zero-Downtime Migration:** In PostgreSQL 11+, `ALTER TABLE ... ADD COLUMN ... TIMESTAMPTZ` (nullable, no default) is a metadata-only change that does not rewrite the table or lock it for extended periods. This will be highlighted in documentation.
  - **Partial Index:** Recommend `CREATE INDEX ON <table> (<trigger>) WHERE <anonymized_marker> IS NULL;` in docs for large tables.
  - **Re-activation Contract:** If an anonymized user returns and receives new personal data, clearing the marker column is the host application's responsibility in V1.
  - **Rejection of Tracking Table:** The `lethe_anonymized` alternative was rejected for V1 to avoid anti-join overhead on every batch and orphan management when parent rows are hard-deleted.

---

## Decision 2: Retention Duration Representation & Calendar Arithmetic

- **Status:** Accepted
- **Context:**
  Go's standard `time.Duration` operates strictly in nanoseconds up to hours. It cannot accurately represent calendar months (`m`) or years (`y`) due to variable month lengths (28–31 days) and leap years. Using a naive 365-day approximation introduces drift and subtle legal retention calculation bugs.
- **Decision:**
  - Parse policy retention durations into a dedicated struct:
    ```go
    type Retention struct {
        Years  int
        Months int
        Days   int
    }
    ```
  - Compute the cutoff using Go's calendar-aware arithmetic:
    ```go
    cutoff := runStart.AddDate(-retention.Years, -retention.Months, -retention.Days)
    ```

---

## Decision 3: Single Point-in-Time `runStart` Parameter

- **Status:** Accepted
- **Context:**
  The draft specification used the application clock for retention cutoff while relying on PostgreSQL's `now()` in SQL for legal hold expiry (`AND (h.until IS NULL OR h.until > now())`). This created clock discrepancy and non-deterministic behavior across long-running batch iterations.
- **Decision:**
  Generate a single UTC timestamp (`runStart`) at the beginning of a run in Go and pass it as a query parameter for both:
  1. Retention cutoff: `WHERE <trigger> < $cutoff`
  2. Hold expiration check: `AND (h.until IS NULL OR h.until > $runStart)`
- **Rationale:**
  - Guarantees point-in-time determinism across all batches in a run.
  - Conservative safety posture: A legal hold scheduled to expire mid-run continues to protect records until the entire run completes.
  - The exact `runStart` and `cutoff` are recorded in `lethe_audit` for audit reproducibility.

---

## Decision 4: Partial Index on `lethe_holds` for Fast Subqueries

- **Status:** Accepted
- **Context:**
  The batch query uses a `NOT EXISTS` correlated subquery against `lethe_holds`:
  ```sql
  AND NOT EXISTS (
    SELECT 1 FROM lethe_holds h
    WHERE h.entity = $entity
      AND h.record_id = <table>.<pk>::text
      AND h.released_at IS NULL
      AND (h.until IS NULL OR h.until > $runStart)
  )
  ```
  On tables with millions of candidate rows, evaluating this subquery without an optimized index would cause sequential scans or slow index scans.
- **Decision:**
  Include a partial index in the migration:
  ```sql
  CREATE INDEX idx_lethe_holds_active 
  ON lethe_holds (entity, record_id) 
  WHERE released_at IS NULL;
  ```
- **Notes:**
  - Because the cast `<table>.<pk>::text` is on the candidate table side of the comparison, PostgreSQL can perform direct index lookups into `lethe_holds`.
  - The test suite will maintain a 1M-row fixture and execute `EXPLAIN (ANALYZE, BUFFERS)` to verify the execution plan.

---

## Decision 5: Advisory Lock Partitioning

- **Status:** Accepted
- **Context:**
  Postgres `pg_try_advisory_lock` must prevent overlapping runs for the same entity while avoiding cross-entity or cross-application lock collisions.
- **Decision:**
  Use the 2-argument integer form:
  ```sql
  SELECT pg_try_advisory_lock($namespace, $entity_hash);
  ```
  - `$namespace`: Constant 32-bit integer for Lethe (`0x4C455448`, ASCII for "LETH").
  - `$entity_hash`: Deterministic 32-bit hash (e.g. CRC32 or FNV-1a) of the entity name string.

---

## Decision 6: Dependency Selection & Footprint Minimization

- **Status:** Accepted
- **Context:**
  The project aims for safety, security, and minimal dependency attack surface, while replacing archived packages and keeping the CLI simple.
- **Decisions:**
  1. **PostgreSQL Driver:** `github.com/jackc/pgx/v5` (`v5.11.0` with `pgxpool`).
  2. **YAML Parsing:** `github.com/goccy/go-yaml` (replaces archived `gopkg.in/yaml.v3`; pure Go, actively maintained, supports `DisallowUnknownField()`).
  3. **UUID Generation:** `github.com/google/uuid` (`v1.6.0`). Zero external dependencies, native `database/sql` driver scanner/valuer support, battle-tested standard.
  4. **CLI Subcommands:** Go standard library `flag` (`flag.NewFlagSet`). Cobra held to minimize third-party dependencies.
  5. **Logging:** Go standard library `log/slog` (structured, strictly PII-free).
  6. **Checksums & Hashes:** Go standard library `crypto/sha256` and `hash/crc32` / `hash/fnv`.
