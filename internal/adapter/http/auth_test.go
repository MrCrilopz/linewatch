package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogin(t *testing.T) {
	h := NewMux(nil, nil, "test-secret")
	bad := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{"username":"demo","password":"nope"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, bad)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login %d", rec.Code)
	}
	ok := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{"username":"demo","password":"linewatch-demo"}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, ok)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["token"] == "" {
		t.Fatal(body)
	}
	open := httptest.NewRequest(http.MethodGet, "/dashboard/summary", nil)
	open.Header.Set("Authorization", "Bearer secret-token-value")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, open)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("open %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret-token-value") {
		t.Fatal(rec.Body.String())
	}
	health := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, health)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("X-Frame-Options") != "DENY" || rec.Header().Get("Referrer-Policy") != "no-referrer" || rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal(rec.Header())
	}
}
