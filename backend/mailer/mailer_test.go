package mailer

import (
	"strings"
	"testing"
)

func TestSanitizeHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean string", "Hello World", "Hello World"},
		{"strips CR", "line\rone", "lineone"},
		{"strips LF", "line\none", "lineone"},
		{"strips CRLF", "line\r\none", "lineone"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeHeader(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeHeader(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildMessage(t *testing.T) {
	msg := string(buildMessage("from@test.com", "to@test.com", "Subject", "Body text"))

	checks := []struct {
		name     string
		contains string
	}{
		{"from header", "From: from@test.com\r\n"},
		{"to header", "To: to@test.com\r\n"},
		{"subject header", "Subject: Subject\r\n"},
		{"mime version", "MIME-Version: 1.0\r\n"},
		{"content type", "Content-Type: text/plain; charset=utf-8\r\n"},
		{"body", "Body text"},
	}
	for _, c := range checks {
		if !strings.Contains(msg, c.contains) {
			t.Errorf("buildMessage missing %s: %q not in %q", c.name, c.contains, msg)
		}
	}
}

func TestBuildMessage_HeaderInjection(t *testing.T) {
	msg := string(buildMessage(
		"from@test.com",
		"to@test.com",
		"Subject\r\nBcc: attacker@evil.com",
		"body",
	))

	// The CRLF is stripped, so "Bcc:" cannot appear as a separate header line.
	if strings.Contains(msg, "\r\nBcc:") || strings.Contains(msg, "\nBcc:") {
		t.Error("buildMessage should prevent header injection via CRLF in subject")
	}
}
