package sqlsafe_test

import (
	"strings"
	"testing"

	"github.com/devaraja-anu/lethe/internal/sqlsafe"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		// --- Valid Identifiers ---
		{
			name:    "valid standard lowercase",
			input:   "users",
			wantErr: false,
		},
		{
			name:    "valid snake_case",
			input:   "user_sessions",
			wantErr: false,
		},
		{
			name:    "valid camelCase",
			input:   "userSessions",
			wantErr: false,
		},
		{
			name:    "valid PascalCase",
			input:   "UserSessions",
			wantErr: false,
		},
		{
			name:    "valid with leading underscore",
			input:   "_private_id",
			wantErr: false,
		},
		{
			name:    "valid with trailing and internal digits",
			input:   "table_v2_migration_01",
			wantErr: false,
		},
		{
			name:    "valid exactly 63 bytes (Postgres NAMEDATALEN - 1 boundary)",
			input:   strings.Repeat("a", 63),
			wantErr: false,
		},

		// --- Empty & Length Boundaries ---
		{
			name:    "invalid empty string",
			input:   "",
			wantErr: true,
			errMsg:  "identifier is empty",
		},
		{
			name:    "invalid exceeds 63 bytes (64 bytes boundary)",
			input:   strings.Repeat("a", 64),
			wantErr: true,
			errMsg:  "identifier exceeds 63 bytes",
		},
		{
			name:    "invalid huge buffer attempt (10KB)",
			input:   strings.Repeat("a", 10240),
			wantErr: true,
			errMsg:  "identifier exceeds 63 bytes",
		},

		// --- Leading Characters ---
		{
			name:    "invalid leading digit",
			input:   "1users",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid only digits",
			input:   "12345",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},

		// --- Whitespace & Control Characters ---
		{
			name:    "invalid leading space",
			input:   " users",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid trailing space",
			input:   "users ",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid internal space",
			input:   "user sessions",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid tab character",
			input:   "user\tsessions",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid newline character",
			input:   "users\n",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid carriage return",
			input:   "users\r",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid null byte prefix",
			input:   "\x00users",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid null byte suffix",
			input:   "users\x00",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},

		// --- SQL Injection & Dangerous Delimiters ---
		{
			name:    "hostile stacked query injection",
			input:   "users; DROP TABLE lethe_audit; --",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "hostile double quote breakout",
			input:   `users" OR "1"="1`,
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "hostile single quote breakout",
			input:   "users' OR '1'='1",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "hostile SQL line comment delimiter",
			input:   "users--",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "hostile SQL block comment delimiter",
			input:   "users/*comment*/",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "hostile backtick syntax attempt",
			input:   "`users`",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid raw double-quoted string",
			input:   `"users"`,
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid dot in single identifier (must use QuoteQualified)",
			input:   "public.users",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},

		// --- Punctuation & Special Symbols ---
		{
			name:    "invalid hyphen (kebab-case)",
			input:   "user-sessions",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid dollar sign",
			input:   "user$id",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid ampersand",
			input:   "user&id",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid slash",
			input:   "users/sessions",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},

		// --- Non-ASCII & Unicode Homoglyphs ---
		{
			name:    "invalid unicode cyrillic homoglyph (looks like 'a')",
			input:   "users\u0430",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid accented latin character",
			input:   "r\u00e9sum\u00e9",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
		{
			name:    "invalid emoji",
			input:   "users\U0001F680",
			wantErr: true,
			errMsg:  "identifier contains invalid characters",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := sqlsafe.Validate(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate(%q) error = %v, wantErr = %v", tc.input, err, tc.wantErr)
			}
			if tc.wantErr && tc.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("Validate(%q) error = %q, expected substring %q", tc.input, err, tc.errMsg)
				}
			}
		})
	}
}

func TestQuote(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "quotes simple identifier",
			input:   "users",
			want:    `"users"`,
			wantErr: false,
		},
		{
			name:    "quotes identifier with underscores and numbers",
			input:   "user_session_1",
			want:    `"user_session_1"`,
			wantErr: false,
		},
		{
			name:    "quotes identifier with leading underscore",
			input:   "_hidden_col",
			want:    `"_hidden_col"`,
			wantErr: false,
		},
		{
			name:      "fails and returns empty for injection attempt",
			input:     "users; DROP TABLE users; --",
			want:      "",
			wantErr:   true,
			errSubstr: "identifier contains invalid characters",
		},
		{
			name:      "fails and returns empty for empty string",
			input:     "",
			want:      "",
			wantErr:   true,
			errSubstr: "identifier is empty",
		},
		{
			name:      "fails and returns empty for over-length string",
			input:     strings.Repeat("col", 22), // 66 bytes
			want:      "",
			wantErr:   true,
			errSubstr: "identifier exceeds 63 bytes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sqlsafe.Quote(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Quote(%q) error = %v, wantErr = %v", tc.input, err, tc.wantErr)
			}
			if tc.wantErr && tc.errSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("Quote(%q) error = %q, expected substring %q", tc.input, err, tc.errSubstr)
				}
			}
			if got != tc.want {
				t.Errorf("Quote(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestQuoteQualified(t *testing.T) {
	tests := []struct {
		name      string
		parts     []string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "quotes schema and table",
			parts:   []string{"public", "users"},
			want:    `"public"."users"`,
			wantErr: false,
		},
		{
			name:    "quotes single part",
			parts:   []string{"users"},
			want:    `"users"`,
			wantErr: false,
		},
		{
			name:    "quotes 3-level qualification (catalog.schema.table)",
			parts:   []string{"app_db", "public", "users"},
			want:    `"app_db"."public"."users"`,
			wantErr: false,
		},
		{
			name:      "fails when parts slice is empty",
			parts:     []string{},
			want:      "",
			wantErr:   true,
			errSubstr: "identifier parts cannot be empty",
		},
		{
			name:      "fails when one part is empty",
			parts:     []string{"public", ""},
			want:      "",
			wantErr:   true,
			errSubstr: "identifier is empty",
		},
		{
			name:      "fails when second part contains injection",
			parts:     []string{"public", "users; DROP TABLE users;"},
			want:      "",
			wantErr:   true,
			errSubstr: "identifier contains invalid characters",
		},
		{
			name:      "fails when first part exceeds 63 bytes",
			parts:     []string{strings.Repeat("s", 64), "users"},
			want:      "",
			wantErr:   true,
			errSubstr: "identifier exceeds 63 bytes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sqlsafe.QuoteQualified(tc.parts...)
			if (err != nil) != tc.wantErr {
				t.Fatalf("QuoteQualified(%v) error = %v, wantErr = %v", tc.parts, err, tc.wantErr)
			}
			if tc.wantErr && tc.errSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("QuoteQualified(%v) error = %q, expected substring %q", tc.parts, err, tc.errSubstr)
				}
			}
			if got != tc.want {
				t.Errorf("QuoteQualified(%v) = %q, want %q", tc.parts, got, tc.want)
			}
		})
	}
}
