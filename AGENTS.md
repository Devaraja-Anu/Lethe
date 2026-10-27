# AGENTS.md — Contributor Guidelines for AI Agents

Welcome to **Lethe**. This document defines operational guidelines, architectural invariants, and the division of labor for AI agents working in this repository.

---

## 1. Project Overview & Mission

Lethe is an embeddable Go library and CLI for safe, auditable data retention in PostgreSQL under the Digital Personal Data Protection (DPDP) Act, 2023 and similar retention frameworks.
- **Tagline:** Safe, auditable data retention for Go and Postgres. Dry-run by default.
- **Primary References:**
  - Read [CONTEXT.md](file:///C:/Programming/Projects/Lethe/CONTEXT.md) for current project state, active milestone, and session continuity log.
  - Read [project_spec.md](file:///C:/Programming/Projects/Lethe/project_spec.md) for full project requirements and build roadmap.
  - Read [decisions.md](file:///C:/Programming/Projects/Lethe/decisions.md) for Architecture Decision Records (ADRs).
  - Read [POLICY.md](file:///C:/Programming/Projects/Lethe/POLICY.md) for declarative retention policy specifications.

---

## 2. Division of Labor (Mandatory)

Per Section 17 of `project_spec.md`, the human owner wants to understand this system intimately and will hand-write the high-risk core. 

### Owner-Written (Do NOT silently author; propose designs and review only):
- The batch loop and transaction logic (`FOR UPDATE SKIP LOCKED`).
- `internal/sqlsafe` (identifier validation and quoting).
- Safety guards (circuit breakers, statement timeouts, advisory locks).
- Policy schema design.
- Adapter interface extraction.

### Agent-Assisted:
- Boilerplate (YAML structs, CLI routing, CI configuration, docker-compose).
- Test scaffolding, fixtures, and adversarial test suites (`testcontainers-go`).
- Documentation, ADR logs, and error-message wording.
- Explanations of PostgreSQL internals (`pg_catalog`, lock behavior, query planners).
- Adversarial review (identifying edge cases where deletes could remove the wrong rows, leak data, or deadlock).

---

## 3. Non-Negotiable Invariants

1. **Dry-Run by Default:** Nothing is modified without an explicit `--apply` flag.
2. **Zero PII in Logs or Audit:** Never log row contents or personal data. Use `log/slog` for structured metadata, counts, and run IDs only.
3. **No Concatenated SQL:** Never construct SQL using string concatenation with user-supplied inputs. All identifiers must pass through `internal/sqlsafe`, and all values must be parameterized (`$1`, `$2`).
4. **Point-in-Time Consistency:** Compute a single `runStart` UTC timestamp in Go at run initialization. Pass it as a parameter for both retention cutoff (`< $cutoff`) and legal hold checks (`until > $runStart`).
5. **Atomic Anonymization:** When anonymizing, update the `anonymized_marker` column in the exact same `UPDATE` transaction that redacts personal data. Selection queries must include `AND <anonymized_marker> IS NULL`.
6. **FK Preflight:** Inspect `pg_constraint` prior to deletion. If any inbound foreign key lacks `ON DELETE CASCADE` and is not listed in `children`, refuse to run.
7. **Advisory Locking:** Acquire a session advisory lock per entity (`pg_try_advisory_lock(0x4C455448, entity_hash)`) to prevent overlapping runs.
8. **Never Weaken Guards:** Never bypass or loosen a safety guard to make a test pass. Fix the test or raise the design conflict.

---

## 4. Tech Stack & Dependencies

- **Language:** Go (1.26+)
- **Database Driver:** `github.com/jackc/pgx/v5` (`pgxpool`)
- **YAML Parser:** `github.com/goccy/go-yaml` (strict mode: `yaml.DisallowUnknownField()`)
- **UUID:** `github.com/google/uuid` (v1.6.0)
- **CLI Routing:** Standard library `flag` (`flag.NewFlagSet`)
- **Logging:** Standard library `log/slog`
- **Testing:** Standard library `testing` (table-driven); `testcontainers-go` for real Postgres tests.

---

## 5. Development & Testing Commands

Local development environment runs on Docker:

```bash
# Start test database
make db-up

# Apply Lethe tables (lethe_holds, lethe_audit)
make migrate

# Load test schema and seed fixtures
make fixtures

# Run unit tests
make test        # or go test -v ./...

# Run integration tests against real PostgreSQL
make test-integration
```

---

## 6. Rules for Making Changes

1. **Explain First:** Always explain what you are changing and why before touching code.
2. **Small PRs/Changes:** One concern per change. Keep diffs clean and reviewable.
3. **Strict V1 Scope:** Do not add features outside V1 scope (no daemons, no HTTP APIs, no multi-DB support, no crypto-shredding). Record future ideas in a backlog.
4. **Log Decisions:** Any architectural or policy decision must be appended to `decisions.md`.
5. **Do NOT make changes to the code before asking permisson** 