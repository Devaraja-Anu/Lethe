package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/devaraja-anu/lethe/internal/sqlsafe"
)

var (
	entityNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	templateRegex   = regexp.MustCompile(`\{([^}]*)\}`)
)

// Validate checks the policy for semantic invariants, schema validity,
// identifier safety via internal/sqlsafe, and business rule constraints.
// It applies default execution parameters if they were omitted.
func (p *Policy) Validate() error {
	var errs ValidationErrors

	// 1. Version check
	if p.Version == 0 {
		errs = append(errs, ValidationError{
			Field:   "version",
			Message: "version is required and must be 1",
		})
	} else if p.Version != 1 {
		errs = append(errs, ValidationError{
			Field:   "version",
			Message: fmt.Sprintf("unsupported version %d; only version 1 is supported", p.Version),
		})
	}

	// 2. Defaults bounds and fallback initialization
	if p.Defaults.BatchSize == 0 {
		p.Defaults.BatchSize = DefaultBatchSize
	} else if p.Defaults.BatchSize < 1 || p.Defaults.BatchSize > MaxBatchSize {
		errs = append(errs, ValidationError{
			Field:   "defaults.batch_size",
			Message: fmt.Sprintf("batch_size must be between 1 and %d (got %d)", MaxBatchSize, p.Defaults.BatchSize),
		})
	}

	if p.Defaults.MaxRowsPerRun == 0 {
		p.Defaults.MaxRowsPerRun = DefaultMaxRowsPerRun
	} else if p.Defaults.MaxRowsPerRun < 1 {
		errs = append(errs, ValidationError{
			Field:   "defaults.max_rows_per_run",
			Message: fmt.Sprintf("max_rows_per_run must be greater than 0 (got %d)", p.Defaults.MaxRowsPerRun),
		})
	}

	if p.Defaults.MaxPercentOfTable == 0 {
		p.Defaults.MaxPercentOfTable = DefaultMaxPercentOfTable
	} else if p.Defaults.MaxPercentOfTable < 1 || p.Defaults.MaxPercentOfTable > 100 {
		errs = append(errs, ValidationError{
			Field:   "defaults.max_percent_of_table",
			Message: fmt.Sprintf("max_percent_of_table must be between 1 and 100 (got %d)", p.Defaults.MaxPercentOfTable),
		})
	}

	// 3. Entities presence & uniqueness
	if len(p.Entities) == 0 {
		errs = append(errs, ValidationError{
			Field:   "entities",
			Message: "policy must define at least one entity",
		})
		return errs
	}

	seenNames := make(map[string]bool)
	for i := range p.Entities {
		e := &p.Entities[i]
		validateEntity(e, seenNames, &errs)
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func validateEntity(e *EntityPolicy, seenNames map[string]bool, errs *ValidationErrors) {
	// Entity Name
	if e.Name == "" {
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "name",
			Message: "entity name cannot be empty",
		})
	} else if !entityNameRegex.MatchString(e.Name) {
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "name",
			Message: fmt.Sprintf("entity name %q is invalid; must contain only alphanumeric characters, underscores, or hyphens", e.Name),
		})
	} else if seenNames[e.Name] {
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "name",
			Message: fmt.Sprintf("duplicate entity name %q; entity names must be unique", e.Name),
		})
	} else {
		seenNames[e.Name] = true
	}

	// Table Name
	if err := sqlsafe.Validate(e.Table); err != nil {
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "table",
			Message: fmt.Sprintf("table %q is invalid: %s", e.Table, err),
		})
	}

	// Primary Key
	if strings.ContainsAny(e.PrimaryKey, ", \t\n") {
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "primary_key",
			Message: fmt.Sprintf("composite primary keys are not supported; primary_key must be a single column (got %q)", e.PrimaryKey),
		})
	} else if err := sqlsafe.Validate(e.PrimaryKey); err != nil {
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "primary_key",
			Message: fmt.Sprintf("primary_key %q is invalid: %s", e.PrimaryKey, err),
		})
	}

	// Trigger Column
	if err := sqlsafe.Validate(e.Trigger); err != nil {
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "trigger",
			Message: fmt.Sprintf("trigger %q is invalid: %s", e.Trigger, err),
		})
	}

	// Retention Window
	if e.Retention == "" {
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "retention",
			Message: "retention duration is required",
		})
	} else if _, err := ParseRetention(e.Retention); err != nil {
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "retention",
			Message: fmt.Sprintf("invalid retention %q; must be a positive integer followed by 'd' (days), 'm' (months), or 'y' (years) (e.g. '3y', '90d')", e.Retention),
		})
	}

	// Entity BatchSize override
	if e.BatchSize != nil {
		if *e.BatchSize < 1 || *e.BatchSize > MaxBatchSize {
			*errs = append(*errs, ValidationError{
				Entity:  e.Name,
				Field:   "batch_size",
				Message: fmt.Sprintf("entity batch_size must be between 1 and %d (got %d)", MaxBatchSize, *e.BatchSize),
			})
		}
	}

	// Action Validation
	switch e.Action {
	case ActionDelete:
		if len(e.Anonymize) > 0 {
			*errs = append(*errs, ValidationError{
				Entity:  e.Name,
				Field:   "anonymize",
				Message: "anonymize map cannot be specified when action is 'delete'",
			})
		}
		if e.AnonymizedMarker != "" {
			*errs = append(*errs, ValidationError{
				Entity:  e.Name,
				Field:   "anonymized_marker",
				Message: "anonymized_marker cannot be specified when action is 'delete'",
			})
		}
		// Validate children
		for i, child := range e.Children {
			if err := sqlsafe.Validate(child.Table); err != nil {
				*errs = append(*errs, ValidationError{
					Entity:  e.Name,
					Field:   fmt.Sprintf("children[%d].table", i),
					Message: fmt.Sprintf("child table %q is invalid: %s", child.Table, err),
				})
			}
			if err := sqlsafe.Validate(child.ForeignKeyColumn); err != nil {
				*errs = append(*errs, ValidationError{
					Entity:  e.Name,
					Field:   fmt.Sprintf("children[%d].fk", i),
					Message: fmt.Sprintf("child foreign key %q is invalid: %s", child.ForeignKeyColumn, err),
				})
			}
		}

	case ActionAnonymize:
		if len(e.Children) > 0 {
			*errs = append(*errs, ValidationError{
				Entity:  e.Name,
				Field:   "children",
				Message: "cascading children cannot be specified when action is 'anonymize'; child cascading is only valid for action 'delete'",
			})
		}
		if e.AnonymizedMarker == "" {
			*errs = append(*errs, ValidationError{
				Entity:  e.Name,
				Field:   "anonymized_marker",
				Message: "anonymized_marker column is required when action is 'anonymize' to ensure run idempotency",
			})
		} else if err := sqlsafe.Validate(e.AnonymizedMarker); err != nil {
			*errs = append(*errs, ValidationError{
				Entity:  e.Name,
				Field:   "anonymized_marker",
				Message: fmt.Sprintf("anonymized_marker %q is invalid: %s", e.AnonymizedMarker, err),
			})
		}

		if len(e.Anonymize) == 0 {
			*errs = append(*errs, ValidationError{
				Entity:  e.Name,
				Field:   "anonymize",
				Message: "anonymize map must define at least one column replacement",
			})
		} else {
			for col, val := range e.Anonymize {
				if err := sqlsafe.Validate(col); err != nil {
					*errs = append(*errs, ValidationError{
						Entity:  e.Name,
						Field:   fmt.Sprintf("anonymize[%s]", col),
						Message: fmt.Sprintf("column %q is invalid: %s", col, err),
					})
				}
				if val != nil {
					// Verify template tokens
					matches := templateRegex.FindAllStringSubmatch(*val, -1)
					for _, m := range matches {
						if len(m) > 1 && m[1] != "id" {
							*errs = append(*errs, ValidationError{
								Entity:  e.Name,
								Field:   fmt.Sprintf("anonymize[%s]", col),
								Message: fmt.Sprintf("unsupported template token {%s}; only {id} is supported", m[1]),
							})
						}
					}
					// Verify email column contains '@'
					if col == "email" || strings.HasSuffix(col, "_email") {
						if !strings.Contains(*val, "@") {
							*errs = append(*errs, ValidationError{
								Entity:  e.Name,
								Field:   fmt.Sprintf("anonymize[%s]", col),
								Message: fmt.Sprintf("email replacement %q must contain an '@' symbol (e.g. 'anon-{id}@invalid')", *val),
							})
						}
					}
				}
			}
		}

	case "":
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "action",
			Message: "action is required; must be 'delete' or 'anonymize'",
		})

	default:
		*errs = append(*errs, ValidationError{
			Entity:  e.Name,
			Field:   "action",
			Message: fmt.Sprintf("unknown action %q; must be 'delete' or 'anonymize'", e.Action),
		})
	}
}
