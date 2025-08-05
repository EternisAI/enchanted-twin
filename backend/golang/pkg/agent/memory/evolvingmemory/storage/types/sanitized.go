package types

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// SanitizedString is a type-safe wrapper for strings that will be used in ILIKE queries.
// It automatically sanitizes input to prevent SQL injection attacks.
type SanitizedString string

// NewSanitizedString creates a new SanitizedString from user input, applying sanitization.
func NewSanitizedString(input string) SanitizedString {
	return SanitizedString(sanitizeILIKEInput(input))
}

// String returns the sanitized string value.
func (s SanitizedString) String() string {
	return string(s)
}

// Value implements the driver.Valuer interface for database operations.
func (s SanitizedString) Value() (driver.Value, error) {
	if s == "" {
		return nil, nil
	}
	return string(s), nil
}

// Scan implements the sql.Scanner interface for database operations.
func (s *SanitizedString) Scan(value interface{}) error {
	if value == nil {
		*s = ""
		return nil
	}
	switch v := value.(type) {
	case string:
		*s = SanitizedString(v)
	case []byte:
		*s = SanitizedString(v)
	default:
		return fmt.Errorf("cannot scan %T into SanitizedString", value)
	}
	return nil
}

// NullableSanitizedString represents a nullable sanitized string for SQLC integration.
type NullableSanitizedString struct {
	String SanitizedString
	Valid  bool
}

// Value implements the driver.Valuer interface.
func (n NullableSanitizedString) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.String.Value()
}

// Scan implements the sql.Scanner interface.
func (n *NullableSanitizedString) Scan(value interface{}) error {
	if value == nil {
		n.String, n.Valid = "", false
		return nil
	}
	n.Valid = true
	return n.String.Scan(value)
}

// sanitizeILIKEInput sanitizes input for ILIKE queries to prevent SQL injection.
// This function should be used whenever passing user input to ILIKE queries.
func sanitizeILIKEInput(input string) string {
	// Remove potential SQL injection characters that could be used maliciously
	// Keep alphanumeric, spaces, hyphens, underscores, and basic punctuation
	cleaned := strings.ReplaceAll(input, "'", "''")     // Escape single quotes
	cleaned = strings.ReplaceAll(cleaned, "\\", "\\\\") // Escape backslashes
	cleaned = strings.ReplaceAll(cleaned, "%", "\\%")   // Escape LIKE wildcards
	cleaned = strings.ReplaceAll(cleaned, "_", "\\_")   // Escape LIKE wildcards
	return cleaned
}
