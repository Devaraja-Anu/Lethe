package policy

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

var (
	// ErrEmptyRetention is returned when an empty retention string is provided.
	ErrEmptyRetention = errors.New("retention duration cannot be empty")

	// ErrInvalidRetentionFormat is returned when the retention string does not conform to [1-9][0-9]*[dmy].
	ErrInvalidRetentionFormat = errors.New("invalid retention format; must be a positive integer followed by 'd' (days), 'm' (months), or 'y' (years) (e.g. '3y', '90d')")

	// ErrZeroRetention is returned when a retention duration is zero.
	ErrZeroRetention = errors.New("retention duration must be greater than zero")
)

// Retention represents a calendar-aware retention duration window.
// It explicitly avoids time.Duration nanosecond approximations to prevent
// calendar and leap-year drift under statutory retention requirements (ADR 2).
type Retention struct {
	Years  int
	Months int
	Days   int
}

// ParseRetention parses a declarative retention duration string (e.g. "3y", "6m", "90d").
// Accepted units:
//   - 'y': Calendar years
//   - 'm': Calendar months
//   - 'd': Days
//
// The value must be a strictly positive integer. Compound units (e.g. "1y6m") and
// non-calendar units (e.g. "h", "s", "w") are rejected.
func ParseRetention(s string) (Retention, error) {
	if s == "" {
		return Retention{}, ErrEmptyRetention
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	if numStr == "" {
		return Retention{}, ErrInvalidRetentionFormat
	}

	val, err := strconv.Atoi(numStr)
	if err != nil {
		return Retention{}, fmt.Errorf("%w: %v", ErrInvalidRetentionFormat, err)
	}

	if val <= 0 {
		return Retention{}, ErrZeroRetention
	}

	switch unit {
	case 'y':
		return Retention{Years: val}, nil
	case 'm':
		return Retention{Months: val}, nil
	case 'd':
		return Retention{Days: val}, nil
	default:
		return Retention{}, ErrInvalidRetentionFormat
	}
}

// IsZero returns true if no retention duration is specified.
func (r Retention) IsZero() bool {
	return r.Years == 0 && r.Months == 0 && r.Days == 0
}

// String returns the canonical declarative string representation of the retention duration.
func (r Retention) String() string {
	switch {
	case r.Years > 0 && r.Months == 0 && r.Days == 0:
		return fmt.Sprintf("%dy", r.Years)
	case r.Months > 0 && r.Years == 0 && r.Days == 0:
		return fmt.Sprintf("%dm", r.Months)
	case r.Days > 0 && r.Years == 0 && r.Months == 0:
		return fmt.Sprintf("%dd", r.Days)
	case r.IsZero():
		return "0d"
	default:
		// Fallback for composite/programmatic instances if ever constructed
		return fmt.Sprintf("%dy%dm%dd", r.Years, r.Months, r.Days)
	}
}

// Cutoff computes the point-in-time timestamp before which database rows are expired:
//
//	cutoff = runStart - retention
//
// Per ADR 2, it uses Go's calendar-aware time.Time.AddDate(-Years, -Months, -Days)
// rather than nanosecond approximations to prevent leap-year and variable month-length drift.
func Cutoff(runStart time.Time, r Retention) time.Time {
	return runStart.AddDate(-r.Years, -r.Months, -r.Days)
}

// Cutoff is a convenience method on Retention that calculates the retention cutoff date.
func (r Retention) Cutoff(runStart time.Time) time.Time {
	return Cutoff(runStart, r)
}

// RetentionDuration parses and returns the entity's configured calendar retention duration.
func (e *EntityPolicy) RetentionDuration() (Retention, error) {
	return ParseRetention(e.Retention)
}
