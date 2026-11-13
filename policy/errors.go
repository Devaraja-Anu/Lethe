package policy

import (
	"fmt"
	"strings"
)

// ValidationError represents a specific semantic validation error within a policy.
type ValidationError struct {
	Entity  string // Logical name of the entity, if applicable
	Field   string // Specific field that failed validation
	Message string // Human-readable explanation and resolution
}

func (e ValidationError) Error() string {
	if e.Entity != "" && e.Field != "" {
		return fmt.Sprintf("entity %q -> field %q: %s", e.Entity, e.Field, e.Message)
	}
	if e.Entity != "" {
		return fmt.Sprintf("entity %q: %s", e.Entity, e.Message)
	}
	if e.Field != "" {
		return fmt.Sprintf("field %q: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidationErrors collects multiple validation errors to display all issues at once.
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return "no validation errors"
	}
	if len(ve) == 1 {
		return fmt.Sprintf("policy validation error: %s", ve[0].Error())
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("policy validation failed with %d errors:\n", len(ve)))
	for i, err := range ve {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}
	return strings.TrimRight(sb.String(), "\n")
}
