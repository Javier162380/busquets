package dto

import (
	"errors"
	"fmt"
)

// Category represents the type of error for handling at different layers.
type Category int

const (
	CategoryNotFound Category = iota
	CategoryValidation
	CategoryConflict
	CategoryUnavailable
	CategoryInternal
)

// String returns a human-readable name for the category.
func (c Category) String() string {
	switch c {
	case CategoryNotFound:
		return "Not Found"
	case CategoryValidation:
		return "Validation Error"
	case CategoryConflict:
		return "Conflict"
	case CategoryUnavailable:
		return "Service Unavailable"
	case CategoryInternal:
		return "Internal Error"
	default:
		return "Unknown Error"
	}
}

// Error is the standard error type for the application.
type Error struct {
	Category Category
	Op       string // Operation that failed (e.g., "GetPlan", "EnableConnector")
	Message  string // User-friendly message
	Err      error  // Underlying error (optional)
}

func (e *Error) Error() string {
	if e.Err != nil {
		if e.Op != "" {
			return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
		}
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	if e.Op != "" {
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

// Is reports whether target matches this error's base message.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Message == t.Message
}

// Sentinel errors - base errors without operation context.
var (
	ErrNotFound               = &Error{Category: CategoryNotFound, Message: "not found"} //nolint:gci // No need
	ErrInvalidDateFormat      = &Error{Category: CategoryValidation, Message: "invalid datetime format"}
	ErrInvalidNumber          = &Error{Category: CategoryValidation, Message: "invalid number value"}
	ErrNoValue                = &Error{Category: CategoryValidation, Message: "no value provided"}
	ErrConnectorDisabled      = &Error{Category: CategoryUnavailable, Message: "connector not initialized"}
	ErrNoConnectorEnabled     = &Error{Category: CategoryUnavailable, Message: "no connector enabled"}
	ErrNoSummarizerConfigured = &Error{Category: CategoryUnavailable, Message: "no summarizer configured"}
	ErrConnectorResponseEmpty = &Error{Category: CategoryUnavailable, Message: "connector returned no response"}
)

// GetCategory extracts the error category, defaulting to Internal.
func GetCategory(err error) Category {
	if err == nil {
		return CategoryInternal
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Category
	}
	return CategoryInternal
}

// IsNotFound checks if error is a not-found error.
func IsNotFound(err error) bool {
	return GetCategory(err) == CategoryNotFound
}
