# Project Spec: Lethe (working name)

> Context file for any AI coding agent or human contributor. It is tool-agnostic: plain Markdown, no vendor-specific instructions. Drop it in the repo root (many agents also look for `AGENTS.md`; copying or symlinking this file to that name is fine).

**Status:** pre-implementation planning. Everything below is a plan, not existing code.
**Name:** `lethe` is a placeholder. Verify the Go module path, GitHub name and CLI binary name are free before committing.
**Tagline:** Safe, auditable data retention for Go and Postgres. Dry-run by default.

---

## 1. Problem and purpose

Most applications store personal data and must delete it when it is no longer needed, because data protection laws limit retention. In India this is the Digital Personal Data Protection (DPDP) Act, 2023 and the DPDP Rules, 2025. Teams typically write ad hoc cleanup scripts that are unsafe, unaudited and forgotten.

This project is an **embeddable Go library plus CLI** that reads a declarative retention policy and safely deletes or anonymizes expired rows in Postgres, with guardrails, legal-hold support and a PII-free audit log.

**What it is:** a tool that enforces retention policies the user defines.
**What it is not:** a compliance guarantee. It must never be described as "making you compliant". Include a "not legal advice" notice in the README.

## 2. Legal background (context only; verify with counsel)

This section explains why the features exist. It is not authoritative and may be out of date. Do not hardcode legal periods into the engine; periods come from the user's policy file.

- India has **no single fixed retention period**. The DPDP Act is purpose-based: keep personal data only as long as the purpose requires.
- The DPDP Rules were notified on 13 November 2025. The retention and erasure provisions are scheduled to take effect on **13 May 2027** (18 months after notification).
- Specific points from the Rules (as understood at planning time, October 2026):
  - A minimum of **1 year** retention for personal data, traffic data and processing logs for certain specified purposes, unless another law requires longer.
  - A **3-year** period (from last user interaction) for certain large platforms (large e-commerce, online gaming, social media above user thresholds). Exact thresholds are in the Rules' Third Schedule; check them.
  - **48 hours' prior notice** to the user before erasure, unless the user logs in or otherwise contacts the data fiduciary.
- Other laws can require longer retention (tax and accounting records, RBI KYC, PMLA, CERT-In log requirements, sector rules). This is why **legal holds** are a first-class concept.
- Later versions may add other jurisdictions (e.g. GDPR). Keep the engine jurisdiction-neutral and put legal specifics in user policy, and later in separate versioned "policy packs".

## 3. Goals and non-goals (V1)

**Goals**
- Read a YAML policy and validate it.
- Postgres only (pgx v5).
- Actions: `delete` and `anonymize`.
- Batched, resumable, concurrency-safe processing.
- Dry-run by default; `--apply` required to change anything.
- Legal holds (a holds table plus an optional per-entity predicate).
- Preflight safety checks (FK coverage, column existence, NULL triggers).
- PII-free audit log.
- CLI: `validate`, `plan`, `run`, `hold add|list|release`.

**Non-goals for V1 (do not build; record ideas in a backlog instead)**
- Other databases (MySQL, Mongo), S3 or other stores.
- 48-hour notice sending (a later hook; the app owns email).
- User-triggered erasure requests.
- Daemon, HTTP/gRPC API, web UI, dashboard.
- Crypto-shredding, processor/third-party propagation.
- Policy linter, data discovery, consent management.
- Composite primary keys (must fail validation with a clear error).

## 4. Roadmap (for orientation)

| Version | Focus |
|---|---|
| V1 | Core engine (this spec) |
| V2 | Usable by others: more adapters, notice hook, erasure requests, linter, daemon/API, docs |
| V3 | India-specific policy packs, crypto-shredding, processor propagation, tamper-evident erasure certificates |
| V4 | Optional hosted control plane (only if real demand) |

Do not plan or build beyond V1 yet.

## 5. Tech stack and conventions

- Go (latest stable), modules enabled.
- `github.com/jackc/pgx/v5` for Postgres.
- YAML parsing: `gopkg.in/yaml.v3` (strict mode: reject unknown fields).
- CLI: stdlib `flag` or `cobra`; keep dependencies minimal.
- Tests: stdlib `testing`, table-driven; `testcontainers-go` for integration tests against real Postgres. No mocking of the database for the core batch logic.
- Logging: structured (`log/slog`). Never log row contents or personal data.
- Errors: wrap with context; the CLI exits non-zero on any failed batch.
- Every user-supplied identifier goes through `internal/sqlsafe`. **No string-concatenated SQL anywhere else.**

## 6. Repository layout

```
lethe/
├── cmd/lethe/main.go          # CLI entrypoint
├── policy/                    # YAML types, parsing, validation
├── engine/                    # orchestration: plan, run, batching, guards
├── adapter/
│   ├── adapter.go             # interface (extracted AFTER postgres works)
│   └── postgres/              # V1 implementation
├── hold/                      # hold model and queries
├── audit/                     # audit records and writer
├── internal/sqlsafe/          # identifier validation and quoting
├── migrations/                # SQL for lethe_holds, lethe_audit
├── examples/                  # sample policy + docker-compose
├── testdata/                  # fixture schema and data
├── PROJECT_SPEC.md            # this file
└── README.md
```

## 7. Policy schema

```yaml
version: 1
defaults:
  batch_size: 500
  max_rows_per_run: 50000          # circuit breaker per entity
  max_percent_of_table: 20         # circuit breaker: abort if a run would exceed this
entities:
  - name: users
    table: users
    primary_key: id                # single column only in V1
    trigger: last_activity_at      # timestamptz column
    retention: 3y                  # units: d, m, y
    action: delete
    children:                      # deleted first, in the same transaction
      - table: user_sessions
        fk: user_id
    hold_if: "kyc_verified = true" # optional; TRUSTED SQL predicate

  - name: support_tickets
    table: support_tickets
    primary_key: id
    trigger: closed_at
    retention: 1y
    action: anonymize
    anonymize:
      email: "redacted-{id}@invalid"
      phone: null
      name: "REDACTED"
    anonymized_marker: anonymized_at   # see open question 1
```

Validation rules:
- Unknown fields are errors.
- `retention` must parse and be positive.
- `table`, `primary_key`, `trigger`, child tables/columns and anonymize columns are validated as identifiers (`sqlsafe`) and, at preflight time, checked to exist in the database.
- Composite primary keys: reject with a clear message.
- `hold_if` is raw SQL from **trusted configuration**. Document this prominently: the policy file must not be writable by untrusted parties.

## 8. Core types (starting point, not final)

```go
type Entity struct {
    Name, Table, PrimaryKey, Trigger string
    Retention  Duration
    Action     Action            // Delete | Anonymize
    Children   []Child
    Anonymize  map[string]*string
    HoldIf     string
    BatchSize  int
}

// Extract this interface only after the concrete Postgres code works.
type Adapter interface {
    Preflight(ctx context.Context, e Entity) ([]Issue, error)
    Plan(ctx context.Context, e Entity, cutoff time.Time) (Plan, error)
    ProcessBatch(ctx context.Context, e Entity, cutoff time.Time) (BatchResult, error)
}

type Plan struct {
    Eligible    int64 // expired and not held
    Held        int64 // expired but held
    NullTrigger int64 // rows that can never expire
}
```

## 9. Batch algorithm (Postgres)

For each entity, loop until a batch returns fewer rows than `batch_size` or a guard trips. **Each batch is one transaction:**

1. Select expired, non-held ids and lock them:
   ```sql
   SELECT id FROM users
   WHERE last_activity_at < $1
     AND NOT (kyc_verified = true)                 -- hold_if, if set
     AND NOT EXISTS (
       SELECT 1 FROM lethe_holds h
       WHERE h.entity = 'users'
         AND h.record_id = users.id::text
         AND h.released_at IS NULL
         AND (h.until IS NULL OR h.until > now()))
   ORDER BY id
   LIMIT $2
   FOR UPDATE SKIP LOCKED
   ```
2. Delete child rows: `DELETE FROM child WHERE fk = ANY($ids)`.
3. Delete or anonymize the parent: `... WHERE id = ANY($ids)`.
4. Insert an audit row in the same transaction.
5. Commit.

Properties this gives: short locks, safe alongside live traffic, resumable after a crash (only the in-flight batch is lost). Note that a whole run is **not atomic**; this is intentional.

## 10. Safety requirements (invariants)

These matter more than any feature. Implement guards **at the same time** as the capability they protect, never afterwards.

1. **Dry-run by default.** Nothing changes without `--apply`.
2. **Circuit breakers:** abort if `max_rows_per_run` is exceeded, or if a run would affect more than `max_percent_of_table`.
3. **FK preflight:** inspect `pg_constraint` for every FK referencing the table. If one is not covered by `ON DELETE CASCADE` or `children`, refuse to run and name the constraint.
4. **NULL-trigger report:** rows with a NULL trigger never match `< cutoff`; `plan` must surface their count.
5. **Policy hash** stored in every audit row.
6. **Advisory lock** per entity (`pg_try_advisory_lock`) to prevent overlapping runs.
7. **Statement timeout** per batch.
8. **Graceful cancellation:** respect `context` cancellation and signals; finish or roll back the current batch cleanly.
9. **No PII in logs or audit.** Counts and metadata only.
10. **Post-run invariant (tested):** after a successful apply, no row older than the cutoff exists unless held.

## 11. Tables the tool owns

```sql
CREATE TABLE lethe_holds (
  id          BIGSERIAL PRIMARY KEY,
  entity      TEXT NOT NULL,
  record_id   TEXT NOT NULL,
  reason      TEXT NOT NULL,
  basis       TEXT,                 -- e.g. 'RBI KYC' (free text in V1)
  until       TIMESTAMPTZ,          -- NULL = indefinite
  created_at  TIMESTAMPTZ DEFAULT now(),
  released_at TIMESTAMPTZ
);

CREATE TABLE lethe_audit (
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
  started_at    TIMESTAMPTZ,
  finished_at   TIMESTAMPTZ
);
```

The audit log stores **no record contents**. Raw record ids can be linkable to a person; consider HMAC-hashed ids in a later version.

## 12. CLI

```
lethe validate -p policy.yaml          # schema validation + preflight; no changes
lethe plan     -p policy.yaml          # counts: eligible / held / null-trigger
lethe run      -p policy.yaml          # dry-run (default)
lethe run      -p policy.yaml --apply  # actually changes data
lethe hold add --entity users --id 42 --reason "RBI KYC" --until 2031-01-01
lethe hold list
lethe hold release --id <hold id>
```

Connection string via flag or environment variable (e.g. `DATABASE_URL`), never stored in the policy file. Exit codes: 0 success, non-zero on any validation failure, guard trip or failed batch.

## 13. Build order

Build thin vertical slices; riskiest first; read-only before destructive; safety before capability; concrete before abstract.

| Step | Work | Notes |
|---|---|---|
| 0 | Repo setup, `go mod init`, docker-compose Postgres, Makefile, CI, **fixture schema** | Fixture must include: parent with two children, an FK without cascade, a nullable trigger column, a composite-key table, rows of varied ages |
| 1 | **Throwaway spike:** hardcoded table and cutoff, batch loop with `FOR UPDATE SKIP LOCKED` | Proves the core approach; expect to rewrite |
| 2 | `internal/sqlsafe` | Identifier validation and quoting; injection-string tests |
| 3 | Policy types, parser, validation | Spike becomes config-driven |
| 4 | `plan` command (read-only) | Eligible / held / NULL-trigger counts |
| 5 | Preflight checks | FK coverage, column existence, trigger column type |
| 6 | Migrations, holds, audit; dry-run end to end | Dry-run writes an audit row marked `dry_run` |
| 7 | Real `delete` with guards | Advisory lock, circuit breakers, statement timeout, cancellation, all together |
| 8 | Extract `Adapter` interface | Shape it from the working code |
| 9 | `anonymize` action | Reuse the batch loop; handle idempotency (open question 1) |
| 10 | `hold` CLI | Writes to the table the engine already reads |
| 11 | Hardening tests | Concurrency, crash/resume, 1M-row performance, invariant check |
| 12 | Dogfood on a real project's staging data; README with a 15-minute quickstart | Budget slack: real schemas always surprise |

## 14. Testing requirements

- **Unit:** policy parsing and validation, duration parsing, `sqlsafe` against injection strings, cutoff arithmetic.
- **Integration (real Postgres via testcontainers):**
  - expired vs. unexpired rows
  - held rows survive; `hold_if` respected; released and expired holds no longer protect
  - child rows are deleted with parents
  - FK preflight rejects an uncovered reference
  - anonymize leaves the row in place and is idempotent
  - circuit breakers trip as configured
  - two concurrent runs do not double-process
  - a crash mid-run followed by a re-run completes correctly
  - NULL-trigger rows are reported, not silently ignored
- **Invariant/property check:** after apply, no non-held row is older than the cutoff.
- **Performance sanity:** a table with ~1M rows processes in bounded time without long locks.

## 15. Definition of done (V1)

- Dry-run on a real project's staging data matches expectations.
- A real `--apply` run deletes exactly the expected rows and nothing else.
- Held rows survive.
- A killed run resumes cleanly.
- A new developer goes from README to a working dry-run in under 15 minutes.

## 16. Open questions and assumptions

These are unresolved. Do not silently decide them; surface them to the human owner.

1. **Anonymize idempotency.** After anonymizing, the trigger value is still older than the cutoff, so the row would be selected on every run. Proposed: an `anonymized_marker` column in the policy (a timestamp set on anonymization and excluded from selection). Needs confirmation, and requires users to add a column.
2. **`hold_if` as raw SQL** is flexible but dangerous if the policy file is not trusted. Alternative: a restricted expression syntax (more V1 scope). Current decision: raw SQL, documented as trusted input.
3. **Postgres and pgx** were assumed, not specified by the owner. Confirm.
4. **Primary key type:** holds store `record_id` as TEXT via `id::text`; confirm this is acceptable for UUID and integer keys, and consider index implications on large tables.
5. **Cutoff clock:** use the database clock (`now()`) or the application clock? Default proposal: pass the cutoff from the application, computed once per run, and record it in the audit row.
6. **Timeline** (about 8 weeks part-time) is an estimate; FK preflight and concurrency/crash tests are the likeliest to overrun.

## 17. Working agreements for AI agents and contributors

**Division of labor (decided by the human owner):** the owner wants to understand this system intimately and will hand-write the high-risk core. Treat the following as **owner-written**; propose designs, review and critique, but do not silently author them:
- the batch loop and transaction logic
- `internal/sqlsafe`
- the safety guards
- the policy schema design
- the adapter interface extraction

An agent may freely help with: boilerplate (YAML structs, CLI wiring, CI config, docker-compose), test scaffolding and fixture generation, documentation and error-message wording, explanations of Postgres internals, and **adversarial review** (finding ways the delete path could remove the wrong rows, leak data or deadlock).

**Rules for any code change:**
1. Explain what you are changing and why before changing it.
2. Keep changes small and reviewable; one concern per change.
3. Do not add features outside the V1 scope in section 3. Put ideas in a backlog file instead.
4. Never construct SQL by string concatenation with user input; use `sqlsafe` and parameters.
5. Never log or persist personal data.
6. Every destructive code path must have an integration test and a dry-run equivalent.
7. Do not weaken or bypass a safety guard to make a test pass; change the test or raise the issue.
8. Do not invent legal claims. If a statement about the law is needed, mark it "verify with counsel".
9. When unsure, ask. Prefer surfacing an open question to making an assumption.
10. Do not run destructive commands against any database that is not the local test container unless the human explicitly instructs it.

## 18. Glossary

- **Data fiduciary / data principal:** DPDP terms for the organization processing data and the person the data is about.
- **Legal hold:** a rule that prevents deletion of specific records (e.g. active dispute, regulatory retention) even when they are past retention.
- **Anonymize:** irreversibly remove or replace identifying fields while keeping the row for analytics or integrity.
- **Dry-run:** compute and report what would change without changing it.
- **Circuit breaker:** a limit that aborts a run when it would affect more rows than expected.
- **Crypto-shredding (V3):** making data unrecoverable by deleting its encryption key.
