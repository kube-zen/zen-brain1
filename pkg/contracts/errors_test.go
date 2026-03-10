// Package contracts tests ValidationError formatting.
package contracts

import (
	"strings"
	"testing"
)

func TestValidationError_ErrorWithField(t *testing.T) {
	err := &ValidationError{
		Field:   "work_type",
		Message: "invalid value",
	}

	msg := err.Error()

	if !strings.Contains(msg, "contracts") {
		t.Error("Error message missing 'contracts' prefix")
	}
	if !strings.Contains(msg, "work_type") {
		t.Error("Error message missing field name")
	}
	if !strings.Contains(msg, "invalid value") {
		t.Error("Error message missing error message")
	}
	if msg != "contracts: work_type: invalid value" {
		t.Errorf("Error() = %q, want 'contracts: work_type: invalid value'", msg)
	}
}

func TestValidationError_ErrorWithoutField(t *testing.T) {
	err := &ValidationError{
		Message: "work item is nil",
	}

	msg := err.Error()

	if !strings.Contains(msg, "contracts") {
		t.Error("Error message missing 'contracts' prefix")
	}
	if !strings.Contains(msg, "work item is nil") {
		t.Error("Error message missing error message")
	}
	if msg != "contracts: work item is nil" {
		t.Errorf("Error() = %q, want 'contracts: work item is nil'", msg)
	}
}

func TestValidationError_ErrorWithEmptyField(t *testing.T) {
	err := &ValidationError{
		Field:   "",
		Message: "validation failed",
	}

	msg := err.Error()

	// Empty field should be handled like no field
	expected := "contracts: validation failed"
	if msg != expected {
		t.Errorf("Error() = %q, want %q", msg, expected)
	}
}

func TestValidationError_ErrorWithEmptyMessage(t *testing.T) {
	err := &ValidationError{
		Field:   "id",
		Message: "",
	}

	msg := err.Error()

	if msg != "contracts: id: " {
		t.Errorf("Error() = %q, want 'contracts: id: '", msg)
	}
}

func TestValidationError_ErrorWithBothEmpty(t *testing.T) {
	err := &ValidationError{
		Field:   "",
		Message: "",
	}

	msg := err.Error()

	if msg != "contracts: " {
		t.Errorf("Error() = %q, want 'contracts: '", msg)
	}

	// Verify it implements error interface correctly
	var _ error = err
}

func TestValidationError_ErrorWithComplexField(t *testing.T) {
	err := &ValidationError{
		Field:   "execution_constraints.max_cost_usd",
		Message: "must be >= 0",
	}

	msg := err.Error()

	if msg != "contracts: execution_constraints.max_cost_usd: must be >= 0" {
		t.Errorf("Error() = %q, want 'contracts: execution_constraints.max_cost_usd: must be >= 0'", msg)
	}
}

func TestValidationError_ErrorWithSpecialCharacters(t *testing.T) {
	err := &ValidationError{
		Field:   "tags.sred",
		Message: "invalid SRED tag \"SRED-INVALID\"",
	}

	msg := err.Error()

	if !strings.Contains(msg, "SRED-INVALID") {
		t.Error("Error message missing quoted value")
	}
}

func TestValidationError_IsError(t *testing.T) {
	err := &ValidationError{
		Field:   "title",
		Message: "required",
	}

	// Verify it can be used as error type
	var e error = err
	if e == nil {
		t.Error("ValidationError should be non-nil when assigned to error type")
	}
	if e.Error() != "contracts: title: required" {
		t.Errorf("As error, Error() = %q", e.Error())
	}
}

func TestValidationError_PointerReceiver(t *testing.T) {
	err := &ValidationError{
		Field:   "priority",
		Message: "invalid priority",
	}

	// Verify the error type has pointer receiver
	// (this test mainly ensures the type is correct)
	if err.Error() == "" {
		t.Error("ValidationError.Error() should not return empty string")
	}
}

func TestValidationError_InErrorChain(t *testing.T) {
	baseErr := &ValidationError{
		Field:   "work_item_id",
		Message: "required",
	}

	// Common pattern: wrap error with additional context
	msg := "failed to validate: " + baseErr.Error()

	expected := "failed to validate: contracts: work_item_id: required"
	if msg != expected {
		t.Errorf("Wrapped error = %q, want %q", msg, expected)
	}
}
