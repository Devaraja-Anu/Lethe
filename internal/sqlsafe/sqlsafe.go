package sqlsafe

import (
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"
)

var identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func Validate(s string) error {
	if s == "" {
		return errors.New("identifier is empty")
	}

	if len(s) > 63 {
		return errors.New("identifier exceeds 63 bytes")
	}

	if !identifierRegex.MatchString(s) {
		return errors.New("identifier contains invalid characters")
	}

	return nil
}

func Quote(s string) (string, error) {
	if err := Validate(s); err != nil {
		return "", err
	}

	return pgx.Identifier{s}.Sanitize(), nil
}

func QuoteQualified(parts ...string) (string, error) {
	if len(parts) == 0 {
		return "", errors.New("identifier parts cannot be empty")
	}

	for _, part := range parts {
		if err := Validate(part); err != nil {
			return "", err
		}
	}

	return pgx.Identifier(parts).Sanitize(), nil
}
