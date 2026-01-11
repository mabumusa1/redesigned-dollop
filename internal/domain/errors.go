package domain

import "fmt"

// ValidationError represents a field validation failure.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: field '%s' %s", e.Field, e.Message)
}

// NewValidationError creates a new ValidationError with the given field and message.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// IsValidationError checks if the given error is a ValidationError.
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// AsValidationError attempts to extract a ValidationError from the given error.
// Returns nil if the error is not a ValidationError.
func AsValidationError(err error) *ValidationError {
	if ve, ok := err.(*ValidationError); ok {
		return ve
	}
	return nil
}
