package validate

import (
	"fmt"
	"strings"
)

// FieldError associates a validation error with a specific field name.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	cause   error
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (e *FieldError) Unwrap() error {
	return e.cause
}

// Errors collects multiple field validation errors.
type Errors struct {
	errs []FieldError
}

// Add wraps a non-nil error as a FieldError and appends it to the collection.
func (e *Errors) Add(field string, err error) {
	if err == nil {
		return
	}
	e.errs = append(e.errs, FieldError{
		Field:   field,
		Message: err.Error(),
		cause:   err,
	})
}

// HasErrors reports whether any errors have been collected.
func (e *Errors) HasErrors() bool {
	return len(e.errs) > 0
}

// Error returns a joined string of all field errors.
func (e *Errors) Error() string {
	msgs := make([]string, len(e.errs))
	for i, fe := range e.errs {
		msgs[i] = fe.Error()
	}
	return strings.Join(msgs, "; ")
}

// Unwrap returns the underlying errors for use with errors.Is.
func (e *Errors) Unwrap() []error {
	out := make([]error, len(e.errs))
	for i := range e.errs {
		out[i] = &e.errs[i]
	}
	return out
}

// Err returns nil if no errors have been collected, or self if there are errors.
func (e *Errors) Err() error {
	if !e.HasErrors() {
		return nil
	}
	return e
}

// Fields returns the structured list of field errors for JSON serialization.
func (e *Errors) Fields() []FieldError {
	return e.errs
}
