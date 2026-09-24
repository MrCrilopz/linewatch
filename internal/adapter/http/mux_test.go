package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(stdhttp.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	NewMux().ServeHTTP(rec, req)

	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body %q", body["status"])
	}
}
