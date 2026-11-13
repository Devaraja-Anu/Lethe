package policy

// Action represents the retention action to execute on expired records.
type Action string

const (
	// ActionDelete removes expired records (and cascade children in the same transaction).
	ActionDelete Action = "delete"

	// ActionAnonymize redacts personal data fields in place in an atomic update.
	ActionAnonymize Action = "anonymize"
)

// Default safety and performance values applied if omitted.
const (
	DefaultBatchSize         = 500
	DefaultMaxRowsPerRun     = 50000
	DefaultMaxPercentOfTable = 20
	MaxBatchSize             = 10000
)

// Policy represents the top-level declarative data retention policy configuration.
type Policy struct {
	// Version specifies the schema version. Currently, only version 1 is supported.
	Version int `yaml:"version"`

	// Defaults defines global performance limits and circuit breakers.
	Defaults Defaults `yaml:"defaults"`

	// Entities lists the data retention rules for individual database entities.
	Entities []EntityPolicy `yaml:"entities"`
}

// Defaults defines global batch execution settings and safety circuit breakers.
type Defaults struct {
	// BatchSize is the number of rows processed per transaction batch (default: 500).
	BatchSize int `yaml:"batch_size"`

	// MaxRowsPerRun is the maximum total rows an entity run is allowed to delete or anonymize before aborting (default: 50000).
	MaxRowsPerRun int `yaml:"max_rows_per_run"`

	// MaxPercentOfTable aborts a run if planned rows exceed this percentage of total table rows (default: 20).
	MaxPercentOfTable int `yaml:"max_percent_of_table"`
}

// EntityPolicy defines the data retention rules, lifecycle, and safety boundaries
// for a single logical data unit (such as user accounts or support tickets).
type EntityPolicy struct {
	// Name is the unique logical identifier for this entity.
	// It is used in audit logs, advisory lock hashing, and the lethe_holds table.
	Name string `yaml:"name"`

	// Table is the PostgreSQL target table where retention rules are applied.
	Table string `yaml:"table"`

	// PrimaryKey is the single-column primary key of the table (e.g. "id").
	PrimaryKey string `yaml:"primary_key"`

	// Trigger is the TIMESTAMPTZ column evaluated against the cutoff date
	// (e.g. "last_activity_at", "closed_at").
	Trigger string `yaml:"trigger"`

	// Retention is the calendar retention window duration (e.g. "3y", "6m", "90d").
	Retention string `yaml:"retention"`

	// Action specifies what to do with expired records: ActionDelete or ActionAnonymize.
	Action Action `yaml:"action"`

	// BatchSize optionally overrides the global defaults.batch_size for this specific entity.
	BatchSize *int `yaml:"batch_size,omitempty"`

	// Children lists dependent tables that must be deleted in the same transaction
	// before the parent row is deleted (exclusive to ActionDelete).
	Children []Child `yaml:"children,omitempty"`

	// HoldIf is an optional trusted SQL boolean predicate. Rows evaluating to TRUE
	// are exempted from deletion/anonymization (e.g. "kyc_verified = true").
	HoldIf string `yaml:"hold_if,omitempty"`

	// Anonymize maps column names to replacement expressions (e.g. "email" -> "redacted-{id}@invalid").
	// Required when Action is ActionAnonymize.
	Anonymize map[string]*string `yaml:"anonymize,omitempty"`

	// AnonymizedMarker is the TIMESTAMPTZ column set to now() when a row is anonymized.
	// Required for ActionAnonymize to guarantee idempotent, non-repeating runs.
	AnonymizedMarker string `yaml:"anonymized_marker,omitempty"`
}

// Entity is an alias for EntityPolicy.
type Entity = EntityPolicy

// Child represents a dependent child table that must have its referencing
// rows deleted before the parent row is deleted in the same transaction.
type Child struct {
	// Table is the name of the dependent child table (e.g., "user_sessions").
	Table string `yaml:"table"`

	// ForeignKeyColumn is the column in the child table that references
	// the parent table's primary key (e.g., "user_id").
	ForeignKeyColumn string `yaml:"fk"`
}
