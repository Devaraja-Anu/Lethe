# CLAUDE.md

See [AGENTS.md](file:///C:/Programming/Projects/Lethe/AGENTS.md) for full contributor guidelines, invariants, and division of labor.

## Quick Reference

### Architecture & Specs
- Current State & Continuity: [CONTEXT.md](file:///C:/Programming/Projects/Lethe/CONTEXT.md)
- Spec: [project_spec.md](file:///C:/Programming/Projects/Lethe/project_spec.md)
- Decisions: [decisions.md](file:///C:/Programming/Projects/Lethe/decisions.md)
- Policy Guide: [POLICY.md](file:///C:/Programming/Projects/Lethe/POLICY.md)
- Plan: Implementation plan in conversation artifacts.

### Key Rules
- Division of labor: High-risk core (batch loop, `internal/sqlsafe`, safety guards) is owner-written. Agent assists with boilerplate, fixtures, testing, and docs.
- Dry-run by default (`--apply` required for changes).
- No string concatenation in SQL; strict parameterization.
- No PII in logs or audit records.

### Commands
- Run unit tests: `go test -v ./...`
- Run integration tests: `go test -v -tags=integration ./...`
- Start DB: `docker compose up -d`
- Apply migrations: `make migrate`
- Load fixtures: `make fixtures`
