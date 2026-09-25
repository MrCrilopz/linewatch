package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, open)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("open %d", rec.Code)
	}
}
