package policy_test

import (
	"errors"
	"testing"
	"time"

	"github.com/devaraja-anu/lethe/policy"
)

func TestParseRetention_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected policy.Retention
	}{
		{input: "1y", expected: policy.Retention{Years: 1}},
		{input: "3y", expected: policy.Retention{Years: 3}},
		{input: "10y", expected: policy.Retention{Years: 10}},
		{input: "100y", expected: policy.Retention{Years: 100}},
		{input: "1m", expected: policy.Retention{Months: 1}},
		{input: "6m", expected: policy.Retention{Months: 6}},
		{input: "12m", expected: policy.Retention{Months: 12}},
		{input: "36m", expected: policy.Retention{Months: 36}},
		{input: "1d", expected: policy.Retention{Days: 1}},
		{input: "30d", expected: policy.Retention{Days: 30}},
		{input: "90d", expected: policy.Retention{Days: 90}},
		{input: "180d", expected: policy.Retention{Days: 180}},
		{input: "365d", expected: policy.Retention{Days: 365}},
		{input: "1000d", expected: policy.Retention{Days: 1000}},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := policy.ParseRetention(tc.input)
			if err != nil {
				t.Fatalf("ParseRetention(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.expected {
				t.Errorf("ParseRetention(%q) = %+v; want %+v", tc.input, got, tc.expected)
			}
			// Verify String() round-trips
			if got.String() != tc.input {
				t.Errorf("Retention.String() = %q; want %q", got.String(), tc.input)
			}
		})
	}
}

func TestParseRetention_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		targetErr error
	}{
		{name: "empty string", input: "", targetErr: policy.ErrEmptyRetention},
		{name: "only unit year", input: "y", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "only unit month", input: "m", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "only unit day", input: "d", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "zero years", input: "0y", targetErr: policy.ErrZeroRetention},
		{name: "zero months", input: "0m", targetErr: policy.ErrZeroRetention},
		{name: "zero days", input: "0d", targetErr: policy.ErrZeroRetention},
		{name: "negative years", input: "-1y", targetErr: policy.ErrZeroRetention},
		{name: "negative days", input: "-30d", targetErr: policy.ErrZeroRetention},
		{name: "missing unit", input: "30", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "unsupported unit weeks", input: "4w", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "unsupported unit hours", input: "24h", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "unsupported unit seconds", input: "60s", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "verbose unit years", input: "3years", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "floating point value", input: "1.5y", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "compound units", input: "1y6m", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "leading space", input: " 3y", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "trailing space", input: "3y ", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "internal space", input: "3 y", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "integer overflow", input: "99999999999999999999999999999999999999999999999999y", targetErr: policy.ErrInvalidRetentionFormat},
		{name: "arbitrary string", input: "invalid", targetErr: policy.ErrInvalidRetentionFormat},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := policy.ParseRetention(tc.input)
			if err == nil {
				t.Fatalf("ParseRetention(%q) expected error, got nil", tc.input)
			}
			if tc.targetErr != nil && !errors.Is(err, tc.targetErr) {
				t.Errorf("ParseRetention(%q) error = %v; want errors.Is %v", tc.input, err, tc.targetErr)
			}
		})
	}
}

func TestRetention_Cutoff_CalendarArithmetic(t *testing.T) {
	utc := time.UTC

	tests := []struct {
		name      string
		runStart  time.Time
		retention policy.Retention
		expected  time.Time
	}{
		{
			name:      "standard 3 years calendar subtraction",
			runStart:  time.Date(2026, time.November, 21, 12, 30, 45, 123456000, utc),
			retention: policy.Retention{Years: 3},
			expected:  time.Date(2023, time.November, 21, 12, 30, 45, 123456000, utc),
		},
		{
			name:      "standard 6 months calendar subtraction",
			runStart:  time.Date(2026, time.November, 21, 10, 0, 0, 0, utc),
			retention: policy.Retention{Months: 6},
			expected:  time.Date(2026, time.May, 21, 10, 0, 0, 0, utc),
		},
		{
			name:      "standard 90 days subtraction across month boundaries",
			runStart:  time.Date(2026, time.November, 21, 0, 0, 0, 0, utc),
			retention: policy.Retention{Days: 90},
			expected:  time.Date(2026, time.August, 23, 0, 0, 0, 0, utc),
		},
		{
			name:      "year boundary transition",
			runStart:  time.Date(2025, time.January, 15, 8, 0, 0, 0, utc),
			retention: policy.Retention{Months: 2},
			expected:  time.Date(2024, time.November, 15, 8, 0, 0, 0, utc),
		},
		{
			name:      "leap year: Feb 29 minus 4 years lands on prior leap day Feb 29",
			runStart:  time.Date(2024, time.February, 29, 14, 0, 0, 0, utc),
			retention: policy.Retention{Years: 4},
			expected:  time.Date(2020, time.February, 29, 14, 0, 0, 0, utc),
		},
		{
			name:      "leap year: Feb 29 minus 1 year normalizes forward to March 1 (ADR 2 AddDate)",
			runStart:  time.Date(2024, time.February, 29, 14, 0, 0, 0, utc),
			retention: policy.Retention{Years: 1},
			expected:  time.Date(2023, time.March, 1, 14, 0, 0, 0, utc),
		},
		{
			name:      "non-leap year Feb 28 minus 1 year retains Feb 28 in prior leap year",
			runStart:  time.Date(2025, time.February, 28, 14, 0, 0, 0, utc),
			retention: policy.Retention{Years: 1},
			expected:  time.Date(2024, time.February, 28, 14, 0, 0, 0, utc),
		},
		{
			name:      "month-end normalization: March 31 minus 1 month normalizes to March 2 (Feb 31 does not exist)",
			runStart:  time.Date(2024, time.March, 31, 0, 0, 0, 0, utc),
			retention: policy.Retention{Months: 1},
			expected:  time.Date(2024, time.March, 2, 0, 0, 0, 0, utc), // Feb 29 + 2 days = March 2 in 2024
		},
		{
			name:      "month-end normalization: May 31 minus 1 month normalizes to May 1 (April has 30 days)",
			runStart:  time.Date(2024, time.May, 31, 0, 0, 0, 0, utc),
			retention: policy.Retention{Months: 1},
			expected:  time.Date(2024, time.May, 1, 0, 0, 0, 0, utc), // April 30 + 1 day = May 1
		},
		{
			name:      "month-end normalization: October 31 minus 1 month normalizes to October 1 (September has 30 days)",
			runStart:  time.Date(2024, time.October, 31, 0, 0, 0, 0, utc),
			retention: policy.Retention{Months: 1},
			expected:  time.Date(2024, time.October, 1, 0, 0, 0, 0, utc), // Sept 30 + 1 day = October 1
		},
		{
			name:      "zero duration returns exact runStart time",
			runStart:  time.Date(2026, time.November, 21, 12, 0, 0, 0, utc),
			retention: policy.Retention{},
			expected:  time.Date(2026, time.November, 21, 12, 0, 0, 0, utc),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotFunc := policy.Cutoff(tc.runStart, tc.retention)
			if !gotFunc.Equal(tc.expected) {
				t.Errorf("Cutoff() = %v; want %v", gotFunc, tc.expected)
			}

			gotMethod := tc.retention.Cutoff(tc.runStart)
			if !gotMethod.Equal(tc.expected) {
				t.Errorf("Retention.Cutoff() = %v; want %v", gotMethod, tc.expected)
			}

			// Ensure timezone is strictly preserved
			if gotFunc.Location() != tc.runStart.Location() {
				t.Errorf("Cutoff() location = %v; want %v", gotFunc.Location(), tc.runStart.Location())
			}
		})
	}
}

func TestRetention_IsZero(t *testing.T) {
	if !(policy.Retention{}).IsZero() {
		t.Errorf("Retention{}.IsZero() = false; want true")
	}
	if (policy.Retention{Years: 1}).IsZero() {
		t.Errorf("Retention{Years: 1}.IsZero() = true; want false")
	}
	if (policy.Retention{Months: 1}).IsZero() {
		t.Errorf("Retention{Months: 1}.IsZero() = true; want false")
	}
	if (policy.Retention{Days: 1}).IsZero() {
		t.Errorf("Retention{Days: 1}.IsZero() = true; want false")
	}
}
