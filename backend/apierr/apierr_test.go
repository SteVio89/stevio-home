package apierr

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, ErrNotFound())

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body APIError
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != "not_found" {
		t.Errorf("code = %q, want %q", body.Code, "not_found")
	}
}

func TestJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, 200, map[string]string{"ok": "true"})

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["ok"] != "true" {
		t.Errorf("body = %v, want ok=true", body)
	}
}

func TestAPIErrorError(t *testing.T) {
	msg := ErrNotFound().Error()
	if msg != "not_found: Resource not found" {
		t.Errorf("Error() = %q", msg)
	}
}
