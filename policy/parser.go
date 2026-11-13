package policy

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"
)

// Load reads and parses a YAML retention policy from an io.Reader.
// It enforces strict schema checks (rejecting unknown fields) and runs
// semantic validation.
func Load(r io.Reader) (*Policy, error) {
	// Read full buffer to ensure we can provide helpful error formatting
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy stream: %w", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("policy file is empty")
	}

	var p Policy
	dec := yaml.NewDecoder(bytes.NewReader(data), yaml.DisallowUnknownField())
	if err := dec.Decode(&p); err != nil {
		// Format error with line numbers and snippet for high-clarity DX
		formatted := yaml.FormatError(err, false, true)
		return nil, fmt.Errorf("policy YAML parsing failed:\n%s", formatted)
	}

	if err := p.Validate(); err != nil {
		return nil, err
	}

	return &p, nil
}

// LoadFile reads and parses a YAML retention policy from a file path on disk.
func LoadFile(path string) (*Policy, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open policy file %q: %w", path, err)
	}
	defer f.Close()

	p, err := Load(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return p, nil
}
