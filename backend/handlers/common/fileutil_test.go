package common

import (
	"testing"
)

func TestIsValidIconURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/media/icons/app.png", true},
		{"/assets/icon.png", true},
		{"javascript:alert(1)", false},
		{"data:image/png;base64,abc", false},
		{"https://evil.com/icon.png", false},
		{"", false},
		{"/other/path", false},
		{"/media/", true},
		{"/assets/", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidIconURL(tt.input)
			if got != tt.want {
				t.Errorf("IsValidIconURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAllowedImageExt(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".png", true},
		{".PNG", true},
		{".jpg", true},
		{".jpeg", true},
		{".webp", true},
		{".gif", true},
		{".svg", false},
		{".exe", false},
		{".dmg", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			if got := AllowedImageExt(tt.ext); got != tt.want {
				t.Errorf("AllowedImageExt(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestAllowedBinaryExt(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".dmg", true},
		{".DMG", true},
		{".pkg", true},
		{".zip", true},
		{".exe", false},
		{".png", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			if got := AllowedBinaryExt(tt.ext); got != tt.want {
				t.Errorf("AllowedBinaryExt(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}
