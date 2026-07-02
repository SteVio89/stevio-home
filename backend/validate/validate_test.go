package validate

import (
	"errors"
	"strings"
	"testing"
)

func TestEmail(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid", "user@example.com", nil},
		{"valid subdomain", "user@mail.example.com", nil},
		{"missing @", "userexample.com", ErrInvalid},
		{"empty local", "@example.com", ErrInvalid},
		{"no domain dot", "user@localhost", ErrInvalid},
		{"leading domain dot", "user@.example.com", ErrInvalid},
		{"trailing domain dot", "user@example.com.", ErrInvalid},
		{"empty string", "", ErrInvalid},
		{"@ only", "@", ErrInvalid},
		{"too long", strings.Repeat("a", 243) + "@example.com", ErrTooLong},
		{"contains CR", "user\r@example.com", ErrInvalid},
		{"contains LF", "user\n@example.com", ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Email(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Email(%q) = %v, want %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"valid", "my-slug", nil},
		{"valid no hyphen", "slug", nil},
		{"valid with numbers", "slug-123", nil},
		{"empty", "", ErrRequired},
		{"leading hyphen", "-slug", ErrInvalid},
		{"trailing hyphen", "slug-", ErrInvalid},
		{"uppercase", "My-Slug", ErrInvalid},
		{"spaces", "my slug", ErrInvalid},
		{"special chars", "slug!", ErrInvalid},
		{"consecutive hyphens", "my--slug", ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Slug(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Slug(%q) = %v, want %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
