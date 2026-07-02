package validate

import (
	"errors"
	"testing"
)

func TestFieldError(t *testing.T) {
	fe := &FieldError{Field: "email", Message: "required", cause: ErrRequired}

	if got := fe.Error(); got != "email: required" {
		t.Errorf("Error() = %q, want %q", got, "email: required")
	}
	if !errors.Is(fe, ErrRequired) {
		t.Error("expected FieldError to wrap ErrRequired")
	}
}

func TestErrorsCollector(t *testing.T) {
	t.Run("empty has no errors", func(t *testing.T) {
		var errs Errors
		if errs.HasErrors() {
			t.Error("expected no errors")
		}
		if errs.Err() != nil {
			t.Error("Err() should return nil when empty")
		}
	})

	t.Run("nil error is ignored", func(t *testing.T) {
		var errs Errors
		errs.Add("name", nil)
		if errs.HasErrors() {
			t.Error("nil error should be ignored")
		}
	})

	t.Run("collects multiple errors", func(t *testing.T) {
		var errs Errors
		errs.Add("name", Slug(""))
		errs.Add("email", Email("bad"))
		if !errs.HasErrors() {
			t.Fatal("expected errors")
		}
		if errs.Err() == nil {
			t.Fatal("Err() should return self when errors exist")
		}
		fields := errs.Fields()
		if len(fields) != 2 {
			t.Fatalf("expected 2 field errors, got %d", len(fields))
		}
		if fields[0].Field != "name" {
			t.Errorf("first field = %q, want %q", fields[0].Field, "name")
		}
		if fields[1].Field != "email" {
			t.Errorf("second field = %q, want %q", fields[1].Field, "email")
		}
	})

	t.Run("errors.Is on collected errors", func(t *testing.T) {
		var errs Errors
		errs.Add("name", Slug(""))
		errs.Add("slug", Slug("BAD!"))

		err := errs.Err()
		if !errors.Is(err, ErrRequired) {
			t.Error("expected errors.Is to find ErrRequired")
		}
		if !errors.Is(err, ErrInvalid) {
			t.Error("expected errors.Is to find ErrInvalid")
		}
		if errors.Is(err, ErrTooLong) {
			t.Error("should not match ErrTooLong")
		}
	})

	t.Run("Error() joins messages", func(t *testing.T) {
		var errs Errors
		errs.Add("a", Slug(""))
		errs.Add("b", Slug(""))
		got := errs.Error()
		if got == "" {
			t.Error("Error() should not be empty")
		}
	})
}
