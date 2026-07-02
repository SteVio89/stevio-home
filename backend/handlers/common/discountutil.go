package common

import (
	"fmt"
	"time"
)

// ValidateDiscountParams checks that discountType is "percent" or "fixed" and
// discountValue is positive (and at most 100 for percent).
func ValidateDiscountParams(discountType string, discountValue int) error {
	if (discountType != "percent" && discountType != "fixed") || discountValue <= 0 {
		return fmt.Errorf("invalid discount type or value")
	}
	if discountType == "percent" && discountValue > 100 {
		return fmt.Errorf("percent discount cannot exceed 100")
	}
	return nil
}

// ParseOptionalTime parses a nullable time string pointer using the given format.
// Returns (nil, nil) when s is nil or empty, (*time.Time, nil) on success, or
// (nil, error) on parse failure.
func ParseOptionalTime(s *string, format string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(format, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
