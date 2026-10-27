# Policy Specification & Configuration Guide

Lethe uses a declarative YAML policy file (conventionally `policy.yaml`) to define retention rules, safety circuit breakers, anonymization templates, and cascading relationships for your PostgreSQL database.

---

## 1. Quickstart & Editor Autocompletion

To enable instant autocompletion, field documentation tooltips, and real-time schema validation in VS Code, Cursor, or JetBrains IDEs, add the YAML Language Server schema header at the very top of your policy file:

```yaml
# yaml-language-server: $schema=../schema/policy.schema.json
version: 1

defaults:
  batch_size: 500
  max_rows_per_run: 50000
  max_percent_of_table: 20

entities:
  - name: user_sessions
    table: user_sessions
    primary_key: id
    trigger: created_at
    retention: 90d
    action: delete
```

---

## 2. Top-Level Structure

A valid Lethe policy contains three sections:

| Field | Type | Required | Description |
|---|---|---|---|
| `version` | `int` | **Yes** | Schema version. Must be `1` in Lethe V1. |
| `defaults` | `object` | No | Global batching sizes and safety circuit breaker thresholds. |
| `entities` | `list` | **Yes** | One or more entity retention definitions. |

---

## 3. Global Defaults & Circuit Breakers

The `defaults` block sets baseline safety limits for every entity unless overridden at the entity level:

```yaml
defaults:
  batch_size: 500              # Rows processed per transaction batch (default: 500)
  max_rows_per_run: 50000      # Circuit breaker: hard cap on total rows per entity per run
  max_percent_of_table: 20     # Circuit breaker: abort if run would exceed X% of table
```

### Safety Invariants
- **`batch_size`:** Keeps lock durations minimal. Lethe locks and processes rows in chunks of this size using `FOR UPDATE SKIP LOCKED`.
- **`max_rows_per_run`:** Prevents unbounded deletions if a retention rule misbehaves or an incident introduces anomalous records.
- **`max_percent_of_table`:** Guards against catastrophic deletion caused by a miscalculated retention trigger. If the eligible rows exceed this percentage of the table, Lethe halts before modifying any data.

---

## 4. Entity Configuration Reference

Each item under `entities` configures retention for a single PostgreSQL table.

```yaml
entities:
  - name: users
    table: users
    primary_key: id
    trigger: last_activity_at
    retention: 3y
    action: delete
    children:
      - table: user_sessions
        fk: user_id
    hold_if: "kyc_verified = true"
```

### Field Definitions

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | **Yes** | Logical identifier for the entity (used in audit logs, advisory lock hashes, and CLI commands). |
| `table` | `string` | **Yes** | PostgreSQL table name. Must be a valid identifier. |
| `primary_key` | `string` | **Yes** | Primary key column name. Single-column PK only in V1. |
| `trigger` | `string` | **Yes** | `TIMESTAMPTZ` column compared against the retention cutoff (e.g. `last_login_at`, `closed_at`). |
| `retention` | `string` | **Yes** | Duration string specifying how long records are retained (see below). |
| `action` | `enum` | **Yes** | Action to take: `delete` or `anonymize`. |
| `batch_size` | `int` | No | Overrides `defaults.batch_size` for this specific entity. |
| `children` | `list` | No | Dependent child tables deleted in the same transaction prior to parent deletion. |
| `hold_if` | `string` | No | Raw SQL boolean predicate. Records where this evaluates to `TRUE` are protected from deletion. |
| `anonymize` | `map` | Conditional | Column replacement map. **Required** when `action: anonymize`. |
| `anonymized_marker` | `string` | Conditional | `TIMESTAMPTZ` marker column. **Required** when `action: anonymize`. |

---

## 5. Retention Duration Syntax

Retention durations use calendar-aware intervals rather than fixed seconds to prevent leap year and variable month length drift:

- `d`: Calendar days (e.g., `30d`, `90d`)
- `m`: Calendar months (e.g., `6m`, `18m`)
- `y`: Calendar years (e.g., `1y`, `3y`)

**Cutoff Calculation:**
The retention cutoff is computed once per run at `runStart` (UTC) in Go:
$$\text{cutoff} = \text{runStart} - (\text{Years}, \text{Months}, \text{Days})$$
Rows with `trigger < cutoff` are eligible for retention processing.

> [!NOTE]
> **NULL Trigger Handling:**
> Rows with `trigger IS NULL` will never match `< cutoff`. `lethe plan` will explicitly count and report NULL-trigger rows so you can identify data hygiene issues.

---

## 6. Actions: `delete` vs `anonymize`

### Action A: `delete` (with Cascading Children)
Hard deletes expired rows. If child tables reference the parent, they must be listed under `children`:

```yaml
- name: accounts
  table: accounts
  primary_key: id
  trigger: deactivated_at
  retention: 2y
  action: delete
  children:
    - table: account_tokens
      fk: account_id
    - table: account_devices
      fk: account_id
```

> [!IMPORTANT]
> **Foreign Key Preflight Inspection:**
> Before running a delete, Lethe inspects PostgreSQL's `pg_constraint`. If an inbound foreign key exists on another table that lacks `ON DELETE CASCADE` and is **not** declared in `children`, Lethe refuses to run and reports the violating constraint.

---

### Action B: `anonymize` (Atomic Redaction)
Replaces identifying PII in-place while keeping the record structure intact for reporting or referential integrity:

```yaml
- name: support_tickets
  table: support_tickets
  primary_key: id
  trigger: closed_at
  retention: 1y
  action: anonymize
  anonymize:
    name: "REDACTED"
    email: "redacted-{id}@invalid"
    phone: null
    notes: "Redacted per retention policy"
  anonymized_marker: anonymized_at
```

#### Anonymization Rules & Templating:
- **`null`**: Sets the column to `NULL` (column must be nullable).
- **String literals**: Replaces the column value with the specified string literal (e.g., `"REDACTED"`).
- **`{id}` token**: Interpolates the row's primary key value (e.g., `"user-{id}@anon.local"`), ensuring uniqueness constraints are not violated.
- **`anonymized_marker`**: Must name a `TIMESTAMPTZ` column. Lethe updates this marker to `now()` in the exact same `UPDATE` statement that redacts personal data. Subsequent runs automatically exclude rows where `<anonymized_marker> IS NOT NULL`.

---

## 7. Legal Holds & `hold_if`

Records can be exempted from retention through two mechanisms:

### 1. Central Legal Holds (`lethe_holds` table)
Specific records subject to regulatory audit, litigation, or tax inquiry can be held via the CLI:
```bash
lethe hold add --entity users --id 42 --reason "Tax Audit 2024" --basis "Income Tax Act" --until 2031-01-01
```
Held records are never deleted or anonymized while an active hold exists.

### 2. Policy-Level Predicate (`hold_if`)
An optional SQL boolean expression evaluated per row. If the expression evaluates to `true`, the record is spared:
```yaml
hold_if: "kyc_verified = true OR account_balance > 0"
```

> [!WARNING]
> **Security Requirement: Trusted Input**
> The `hold_if` expression is incorporated directly into candidate selection queries. The policy file must always be treated as **trusted application configuration** (managed in source control or restricted CI/CD). Never allow untrusted users to write to `policy.yaml`.

---

## 8. CLI Workflow & Validation

Before applying changes, use the CLI's read-only commands to verify your policy:

```bash
# 1. Validate policy syntax and run preflight checks against PostgreSQL
lethe validate -p policy.yaml

# 2. Plan and inspect eligible, held, and NULL-trigger row counts (read-only)
lethe plan -p policy.yaml

# 3. Dry-run batch execution (audited, but modifies no data)
lethe run -p policy.yaml

# 4. Apply retention modifications (requires explicit --apply flag)
lethe run -p policy.yaml --apply
```
