# GEMINI.md — Antigravity & Gemini Agent Guidelines

This file provides project-level instructions for Gemini and Google Antigravity agents working in the **Lethe** repository.

---

## 1. Project Context & Philosophy

- **Project:** Lethe (safe, auditable data retention for Go & PostgreSQL).
- **Primary References:**
  - Continuity & State: [CONTEXT.md](file:///C:/Programming/Projects/Lethe/CONTEXT.md) (read first for current state)
  - Specifications: [project_spec.md](file:///C:/Programming/Projects/Lethe/project_spec.md)
  - Architecture Decisions: [decisions.md](file:///C:/Programming/Projects/Lethe/decisions.md)
  - Policy Guide: [POLICY.md](file:///C:/Programming/Projects/Lethe/POLICY.md)
  - Contributor Guidelines: [AGENTS.md](file:///C:/Programming/Projects/Lethe/AGENTS.md)

---

## 2. Division of Labor (Strict)

Per Section 17 of `project_spec.md`:
- **Owner-Written Core:** Do NOT silently author or rewrite:
  - Batch loop and transaction logic (`FOR UPDATE SKIP LOCKED`).
  - `internal/sqlsafe` identifier validation & quoting.
  - Safety guards (circuit breakers, statement timeouts, advisory locks).
  - Policy schema design.
  - Adapter interface extraction.
  *Agent role on these components:* Propose designs, review, identify bugs/race conditions, and write tests.
- **Agent-Assisted Scope:**
  - YAML structs, CLI wiring, environment setup, and test scaffolding.
  - Test suites (`testdata/schema.sql`, `testdata/fixtures.sql`, `testcontainers-go`).
  - Adversarial review (edge cases where queries delete wrong rows, lock excessively, or fail on FKs).
  - Documentation and error messaging.

---

## 3. Core Architectural Invariants

1. **Dry-Run by Default:** Every operation is read-only unless `--apply` is explicitly passed.
2. **Zero PII Logging:** Never log row data, emails, names, or record content. Log structured counts, durations, and metadata only using `log/slog`.
3. **No String Concatenation in SQL:** All identifiers must pass through `internal/sqlsafe`. All values must be parameterized (`$1`, `$2`).
4. **Point-in-Time Consistency:** Compute a single UTC `runStart` in Go at run initialization. Pass it as a query parameter for both retention cutoff (`trigger < $cutoff`) and legal hold checks (`until > $runStart`).
5. **Atomic Anonymization:** Redacting updates and `anonymized_marker` timestamp updates must occur in the exact same `UPDATE` transaction. Selection queries filter `AND <anonymized_marker> IS NULL`.
6. **Preflight Safety:** Always inspect `pg_constraint` prior to deletion to ensure all referencing foreign keys have `ON DELETE CASCADE` or are registered in `children`.
7. **Advisory Locks:** Acquire `pg_try_advisory_lock(0x4C455448, entity_hash)` to protect entities from concurrent runs.
8. **Never Weaken Guards:** If a test fails because of a safety guard, do not loosen the guard.

---

## 4. Stack & Development Commands

- **Go 1.26+**
- **Dependencies:** `github.com/jackc/pgx/v5`, `github.com/goccy/go-yaml`, `github.com/google/uuid`.
- **CLI & Logging:** Go standard library `flag` and `log/slog`.

### Developer Commands
```bash
# Docker DB management
make db-up         # Start Postgres container
make db-down       # Stop and clean container
make migrate       # Apply lethe_holds and lethe_audit tables
make fixtures      # Load test schema and seed fixtures

# Testing
make test             # Unit tests (go test -v ./...)
make test-integration # Integration tests with real Postgres
```
