package pathutil

import (
	"testing"
)

func TestSafePath(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name    string
		parts   []string
		wantErr bool
	}{
		{"simple file", []string{"file.txt"}, false},
		{"nested path", []string{"a", "b", "c.txt"}, false},
		{"traversal blocked", []string{"..", "etc", "passwd"}, true},
		{"dot-dot in middle", []string{"a", "..", "..", "outside"}, true},
		{"clean stays inside", []string{"a", "..", "b"}, false},
		{"root itself", []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SafePath(root, tt.parts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafePath(%q, %v) error = %v, wantErr %v", root, tt.parts, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizePathSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"simple filename", "file.txt", "file.txt", false},
		{"strips directory", "a/b/file.txt", "file.txt", false},
		{"rejects dot", ".", "", true},
		{"rejects dotdot", "..", "", true},
		{"rejects slash", "/", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizePathSegment(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizePathSegment(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("SanitizePathSegment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal filename", "report.pdf", "report.pdf"},
		{"strips unsafe chars", "file<>name.txt", "filename.txt"},
		{"preserves hyphens and underscores", "my_file-2.txt", "my_file-2.txt"},
		{"all unsafe returns download", "///", "download"},
		{"empty returns download", "", "download"},
		{"unicode stripped", "résumé.pdf", "rsum.pdf"},
		{"spaces stripped", "my file.txt", "myfile.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
