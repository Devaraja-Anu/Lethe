package policy_test

import (
	"strings"
	"testing"

	"github.com/devaraja-anu/lethe/policy"
)

func TestLoad_ExamplePolicy(t *testing.T) {
	p, err := policy.LoadFile("../examples/policy.yaml")
	if err != nil {
		t.Fatalf("expected examples/policy.yaml to load cleanly, got error: %v", err)
	}

	if p.Version != 1 {
		t.Errorf("expected version 1, got %d", p.Version)
	}
	if p.Defaults.BatchSize != 500 {
		t.Errorf("expected default batch_size 500, got %d", p.Defaults.BatchSize)
	}
	if p.Defaults.MaxRowsPerRun != 50000 {
		t.Errorf("expected default max_rows_per_run 50000, got %d", p.Defaults.MaxRowsPerRun)
	}
	if p.Defaults.MaxPercentOfTable != 20 {
		t.Errorf("expected default max_percent_of_table 20, got %d", p.Defaults.MaxPercentOfTable)
	}
	if len(p.Entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(p.Entities))
	}

	// Entity 1: users (delete)
	e1 := p.Entities[0]
	if e1.Name != "users" || e1.Action != policy.ActionDelete {
		t.Errorf("unexpected entity 1 config: %+v", e1)
	}
	if len(e1.Children) != 1 || e1.Children[0].Table != "user_sessions" || e1.Children[0].ForeignKeyColumn != "user_id" {
		t.Errorf("unexpected children config: %+v", e1.Children)
	}

	// Entity 2: support_tickets (anonymize)
	e2 := p.Entities[1]
	if e2.Name != "support_tickets" || e2.Action != policy.ActionAnonymize {
		t.Errorf("unexpected entity 2 config: %+v", e2)
	}
	if e2.AnonymizedMarker != "anonymized_at" {
		t.Errorf("expected anonymized_marker 'anonymized_at', got %q", e2.AnonymizedMarker)
	}
	if len(e2.Anonymize) != 3 {
		t.Errorf("expected 3 anonymize rules, got %d", len(e2.Anonymize))
	}
}

func TestLoad_StrictUnknownFields(t *testing.T) {
	yamlInput := `
version: 1
entities:
  - name: users
    tabel: users # Typo here
    primary_key: id
    trigger: created_at
    retention: 30d
    action: delete
`
	_, err := policy.Load(strings.NewReader(yamlInput))
	if err == nil {
		t.Fatal("expected error on unknown field 'tabel', got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "unknown field") || !strings.Contains(errMsg, "tabel") {
		t.Errorf("expected error message to mention unknown field 'tabel', got:\n%s", errMsg)
	}
}

func TestLoad_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		errContains []string
	}{
		{
			name: "composite primary key rejection",
			yaml: `
version: 1
entities:
  - name: composite_test
    table: users
    primary_key: "id, tenant_id"
    trigger: created_at
    retention: 1y
    action: delete
`,
			errContains: []string{
				"composite primary keys are not supported; primary_key must be a single column",
				"id, tenant_id",
			},
		},
		{
			name: "invalid identifier in table name",
			yaml: `
version: 1
entities:
  - name: inject_test
    table: "users; DROP TABLE users;--"
    primary_key: id
    trigger: created_at
    retention: 1y
    action: delete
`,
			errContains: []string{
				"table \"users; DROP TABLE users;--\" is invalid",
			},
		},
		{
			name: "duplicate entity names",
			yaml: `
version: 1
entities:
  - name: duplicate_users
    table: users
    primary_key: id
    trigger: created_at
    retention: 1y
    action: delete
  - name: duplicate_users
    table: users_archive
    primary_key: id
    trigger: created_at
    retention: 1y
    action: delete
`,
			errContains: []string{
				"duplicate entity name \"duplicate_users\"",
			},
		},
		{
			name: "non-positive retention duration",
			yaml: `
version: 1
entities:
  - name: zero_retention
    table: users
    primary_key: id
    trigger: created_at
    retention: 0d
    action: delete
`,
			errContains: []string{
				"invalid retention \"0d\"",
			},
		},
		{
			name: "invalid retention unit (hours not supported in calendar arithmetic)",
			yaml: `
version: 1
entities:
  - name: hours_retention
    table: users
    primary_key: id
    trigger: created_at
    retention: 24h
    action: delete
`,
			errContains: []string{
				"invalid retention \"24h\"",
			},
		},
		{
			name: "action delete with anonymize mapping",
			yaml: `
version: 1
entities:
  - name: conflict_delete
    table: users
    primary_key: id
    trigger: created_at
    retention: 1y
    action: delete
    anonymize:
      name: "REDACTED"
`,
			errContains: []string{
				"anonymize map cannot be specified when action is 'delete'",
			},
		},
		{
			name: "action delete with anonymized_marker",
			yaml: `
version: 1
entities:
  - name: conflict_marker
    table: users
    primary_key: id
    trigger: created_at
    retention: 1y
    action: delete
    anonymized_marker: anonymized_at
`,
			errContains: []string{
				"anonymized_marker cannot be specified when action is 'delete'",
			},
		},
		{
			name: "action anonymize missing anonymized_marker",
			yaml: `
version: 1
entities:
  - name: missing_marker
    table: users
    primary_key: id
    trigger: created_at
    retention: 1y
    action: anonymize
    anonymize:
      name: "REDACTED"
`,
			errContains: []string{
				"anonymized_marker column is required when action is 'anonymize'",
			},
		},
		{
			name: "action anonymize with empty anonymize block",
			yaml: `
version: 1
entities:
  - name: empty_anonymize
    table: users
    primary_key: id
    trigger: created_at
    retention: 1y
    action: anonymize
    anonymized_marker: anonymized_at
    anonymize: {}
`,
			errContains: []string{
				"anonymize map must define at least one column replacement",
			},
		},
		{
			name: "action anonymize with children",
			yaml: `
version: 1
entities:
  - name: anonymize_children
    table: users
    primary_key: id
    trigger: created_at
    retention: 1y
    action: anonymize
    anonymized_marker: anonymized_at
    anonymize:
      name: "REDACTED"
    children:
      - table: user_sessions
        fk: user_id
`,
			errContains: []string{
				"cascading children cannot be specified when action is 'anonymize'; child cascading is only valid for action 'delete'",
			},
		},
		{
			name: "unsupported template token in anonymize",
			yaml: `
version: 1
entities:
  - name: token_error
    table: users
    primary_key: id
    trigger: created_at
    retention: 1y
    action: anonymize
    anonymized_marker: anonymized_at
    anonymize:
      email: "user-{user_uuid}@invalid"
`,
			errContains: []string{
				"unsupported template token {user_uuid}; only {id} is supported",
			},
		},
		{
			name: "email anonymization without at symbol",
			yaml: `
version: 1
entities:
  - name: email_at_error
    table: users
    primary_key: id
    trigger: created_at
    retention: 1y
    action: anonymize
    anonymized_marker: anonymized_at
    anonymize:
      email: "redacted-id-invalid"
`,
			errContains: []string{
				"email replacement \"redacted-id-invalid\" must contain an '@' symbol",
			},
		},
		{
			name: "circuit breaker out of bounds",
			yaml: `
version: 1
defaults:
  batch_size: 50000 # exceeds 10000 max
  max_percent_of_table: 105 # exceeds 100%
entities:
  - name: users
    table: users
    primary_key: id
    trigger: created_at
    retention: 1y
    action: delete
`,
			errContains: []string{
				"batch_size must be between 1 and 10000 (got 50000)",
				"max_percent_of_table must be between 1 and 100 (got 105)",
			},
		},
		{
			name: "unsupported version",
			yaml: `
version: 2
entities:
  - name: users
    table: users
    primary_key: id
    trigger: created_at
    retention: 1y
    action: delete
`,
			errContains: []string{
				"unsupported version 2; only version 1 is supported",
			},
		},
		{
			name: "empty entities",
			yaml: `
version: 1
entities: []
`,
			errContains: []string{
				"policy must define at least one entity",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := policy.Load(strings.NewReader(tt.yaml))
			if err == nil {
				t.Fatalf("expected validation error, got nil")
			}

			errStr := err.Error()
			for _, expected := range tt.errContains {
				if !strings.Contains(errStr, expected) {
					t.Errorf("expected error to contain %q, but got:\n%s", expected, errStr)
				}
			}
		})
	}
}

func TestLoad_MultiErrorAggregation(t *testing.T) {
	// A policy with multiple errors across different fields and entities
	yamlInput := `
version: 1
entities:
  - name: entity_one
    table: users
    primary_key: "id, tenant_id" # Error 1: composite key
    trigger: created_at
    retention: 0d                 # Error 2: non-positive retention
    action: delete
  - name: entity_two
    table: support_tickets
    primary_key: id
    trigger: closed_at
    retention: 1y
    action: anonymize             # Error 3: missing anonymized_marker
    anonymize: {}                 # Error 4: empty anonymize map
`
	_, err := policy.Load(strings.NewReader(yamlInput))
	if err == nil {
		t.Fatal("expected multi-error validation failure, got nil")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "policy validation failed with 4 errors:") {
		t.Errorf("expected header with 4 errors, got:\n%s", errStr)
	}
	if !strings.Contains(errStr, "1. entity \"entity_one\" -> field \"primary_key\"") {
		t.Errorf("expected line item for primary_key, got:\n%s", errStr)
	}
	if !strings.Contains(errStr, "2. entity \"entity_one\" -> field \"retention\"") {
		t.Errorf("expected line item for retention, got:\n%s", errStr)
	}
	if !strings.Contains(errStr, "3. entity \"entity_two\" -> field \"anonymized_marker\"") {
		t.Errorf("expected line item for anonymized_marker, got:\n%s", errStr)
	}
	if !strings.Contains(errStr, "4. entity \"entity_two\" -> field \"anonymize\"") {
		t.Errorf("expected line item for anonymize, got:\n%s", errStr)
	}
}
