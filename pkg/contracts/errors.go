// Package contracts defines the canonical data types used across zen-brain.
package contracts

import "fmt"

// ValidationError represents a contract validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("contracts: %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("contracts: %s", e.Message)
}
