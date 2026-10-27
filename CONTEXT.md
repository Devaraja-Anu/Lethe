# Lethe Project Context & Continuity Log

This file records the current state of the repository, key architectural decisions, completed work, and active tasks. When starting a new conversation or agent session, read this file first to resume immediately without redundant discovery.

---

## 1. Project Overview & Mission

- **Project:** Lethe (safe, auditable data retention for Go & PostgreSQL).
- **Core Action:** Batched, transactional deletion (`delete`) and redaction (`anonymize`) of expired rows under the DPDP Act, 2023 / DPDP Rules, 2025.
- **Invariants:**
  1. Dry-run by default (`--apply` required to mutate).
  2. Zero PII in logs or `lethe_audit`.
  3. No string-concatenated SQL; strictly parameterized values with identifiers quoted via `internal/sqlsafe`.
  4. Point-in-time consistency via a single UTC `runStart` parameter.
  5. Atomic anonymization (marker column updated in the exact same `UPDATE` as redacted fields).
  6. Foreign key preflight checks against `pg_constraint`.
  7. Advisory locking per entity (`0x4C455448`, CRC32 hash).

---

## 2. Division of Labor (`steps.md`)

Per contributor guidelines, the project owner hand-writes high-risk core components:
- **Human-Written (H):**
  - Batch loop & transaction logic (`FOR UPDATE SKIP LOCKED`).
  - `internal/sqlsafe` identifier validation & quoting.
  - Safety guards (circuit breakers, statement timeouts, advisory locks).
  - Policy schema design & decision functions.
  - Adapter interface extraction.
- **Agent-Assisted (AI):**
  - Boilerplate, test scaffolding, and fixtures.
  - Adversarial reviews (identifying edge cases, deadlocks, leaked rows, race conditions).
  - Documentation, schema definitions, and tooling configs.
  - CLI routing and argument wiring.
- **Rule:** The agent must **never** make changes to code or architecture without explicit human permission.

---

## 3. Completed Milestones

### Step 0: Setup, Fixtures & Tooling (Completed)
- **Dependencies:** Configured in `go.mod` (Go 1.26+, `pgx/v5`, `goccy/go-yaml`, `google/uuid`).
- **Postgres Migrations:** [migrations/001_create_lethe_tables.sql](file:///C:/Programming/Projects/Lethe/migrations/001_create_lethe_tables.sql) (`lethe_holds` with active partial index + `lethe_audit`).
- **Test Schema & Fixtures:** 
  - [testdata/schema.sql](file:///C:/Programming/Projects/Lethe/testdata/schema.sql): Parents, children (cascade vs restrict FKs), nullable triggers, and composite PKs.
  - [testdata/fixtures.sql](file:///C:/Programming/Projects/Lethe/testdata/fixtures.sql): Expired users, active users, active/expired/released holds, and already-anonymized records.
- **Developer Experience & Policy Documentation:**
  - [POLICY.md](file:///C:/Programming/Projects/Lethe/POLICY.md): Comprehensive developer guide for `policy.yaml`.
  - [schema/policy.schema.json](file:///C:/Programming/Projects/Lethe/schema/policy.schema.json): JSON Schema Draft-07 for IDE autocomplete & validation.
  - [examples/policy.yaml](file:///C:/Programming/Projects/Lethe/examples/policy.yaml): Richly commented reference starter template.
- **Task Runner:** Standardized on [Makefile](file:///C:/Programming/Projects/Lethe/Makefile). Removed redundant PowerShell scripts.

### Step 1: Throwaway Spike (Completed)
- **Standalone Spike:** Implemented in [cmd/spike/cmd.go](file:///C:/Programming/Projects/Lethe/cmd/spike/cmd.go).
- **Verified Mechanics:**
  - `FOR UPDATE SKIP LOCKED` candidate selection and locking in ascending PK order.
  - Child-first cascading deletion (`DELETE FROM user_sessions WHERE user_id = ANY($ids)`).
  - Parent deletion (`DELETE FROM users WHERE id = ANY($ids)`).
  - Resumable batch loop committing per iteration with immediate deferred rollback cleanup (`defer tx.Rollback(ctx)` inside `processBatch`).
  - Point-in-time consistency with single `runStart` UTC timestamp.
  - Successfully verified against edge cases in `testdata/fixtures.sql` (active holds, future holds, expired holds, released holds, KYC predicate, and NULL triggers).

---

## 4. Current Active Milestone: Step 2 (`internal/sqlsafe`)

- **Objective:** Human owner hand-writes identifier validation (allowed characters, length, non-empty) and quoting, using `pgx.Identifier.Sanitize()` as a building block.
- **Invariants:**
  - No SQL string concatenation exists outside `internal/sqlsafe`.
  - Validates and sanitizes table names, column names, schema names.
- **Agent Role for Step 2:** Provide a large list of hostile identifier strings for tests, plus adversarial review.

---

## 5. Architecture Decisions Log (Summary of `decisions.md`)

- **ADR 1 (Anonymization Idempotency):** Marker column (`anonymized_at TIMESTAMPTZ`) updated in the same atomic `UPDATE` statement; candidate query includes `AND <marker> IS NULL`.
- **ADR 2 (Retention Arithmetic):** Dedicated struct `Retention{Years, Months, Days}` using Go's `AddDate(-y, -m, -d)` to avoid leap-year and variable-month drift.
- **ADR 3 (Point-in-Time Consistency):** Go computes single UTC `runStart` at initialization, passed as parameter for both retention cutoff and hold expiry checks.
- **ADR 4 (Active Holds Partial Index):** Partial index on `lethe_holds (entity, record_id) WHERE released_at IS NULL` for microsecond `NOT EXISTS` checks.
- **ADR 5 (Advisory Locking Scheme):** 2-argument integer form: `pg_try_advisory_lock(0x4C455448, crc32(entity_name))`.
- **ADR 6 (Dependency Selection):** `pgx/v5`, `goccy/go-yaml` (strict mode), `google/uuid`, stdlib `flag` and `log/slog`.
- **Documentation Strategy:** Single, curated [decisions.md](file:///C:/Programming/Projects/Lethe/decisions.md) in the project root to eliminate process bloat.

---

## 6. Quick Commands

```bash
make db-up         # Start Postgres test container
make migrate       # Run lethe_holds and lethe_audit migrations
make fixtures      # Seed test schema and edge-case fixtures
make test          # Run unit tests
make test-integration # Run real Postgres integration tests
```
