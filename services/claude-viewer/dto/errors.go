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
	// NotFound errors (404).
	ErrNotFound = &Error{Category: CategoryNotFound, Message: "not found"}

	// Validation errors (400).
	ErrValidation        = &Error{Category: CategoryValidation, Message: "validation failed"}
	ErrInvalidFormat     = &Error{Category: CategoryValidation, Message: "invalid format"}
	ErrInvalidDateFormat = &Error{Category: CategoryValidation, Message: "invalid datetime format"}
	ErrInvalidNumber     = &Error{Category: CategoryValidation, Message: "invalid number value"}
	ErrInvalidToken      = &Error{Category: CategoryValidation, Message: "invalid pagination token"}
	ErrNoValue           = &Error{Category: CategoryValidation, Message: "no value provided"}

	// Conflict errors (409).
	ErrConflict = &Error{Category: CategoryConflict, Message: "conflict detected"}

	// Unavailable errors (503).
	ErrUnavailable        = &Error{Category: CategoryUnavailable, Message: "service unavailable"}
	ErrConnectorDisabled  = &Error{Category: CategoryUnavailable, Message: "connector not initialized"}
	ErrNoConnectorEnabled = &Error{Category: CategoryUnavailable, Message: "no connector enabled"}

	// Internal errors (500).
	ErrInternal = &Error{Category: CategoryInternal, Message: "internal error"}
)

// E creates a new error with operation context.
// Usage: dto.E(dto.ErrPlanNotFound, "GetPlanByFileName", err).
func E(sentinel *Error, op string, cause error) *Error {
	return &Error{
		Category: sentinel.Category,
		Op:       op,
		Message:  sentinel.Message,
		Err:      cause,
	}
}

// Wrap wraps an error with a sentinel error type.
// Usage: dto.Wrap(dto.ErrInternal, err).
func Wrap(sentinel *Error, cause error) *Error {
	return &Error{
		Category: sentinel.Category,
		Message:  sentinel.Message,
		Err:      cause,
	}
}

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

// IsValidation checks if error is a validation error.
func IsValidation(err error) bool {
	return GetCategory(err) == CategoryValidation
}

// IsConflict checks if error is a conflict error.
func IsConflict(err error) bool {
	return GetCategory(err) == CategoryConflict
}

// IsUnavailable checks if error is an unavailable error.
func IsUnavailable(err error) bool {
	return GetCategory(err) == CategoryUnavailable
}

// IsInternal checks if error is an internal error.
func IsInternal(err error) bool {
	return GetCategory(err) == CategoryInternal
}
