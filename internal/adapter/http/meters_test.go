package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"linewatch/internal/adapter/sqlite"
)

func TestMeterFilterAndMissing(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	dir := repoData(t)
	if err := db.Load(filepath.Join(dir, "readings.csv"), filepath.Join(dir, "events.csv")); err != nil {
		t.Fatal(err)
	}
	mux := NewMux(db)

	rec := get(mux, "/meters?status=ok&q=M-109&sort=consumption")
	if rec.Code != http.StatusOK {
		t.Fatalf("filter status %d %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Meters []struct {
			ID string `json:"meter_id"`
		} `json:"meters"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Meters) != 1 || list.Meters[0].ID != "M-109" {
		t.Fatalf("filter %+v", list.Meters)
	}

	missing := get(mux, "/meters/M-999")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing %d", missing.Code)
	}
	var errBody map[string]string
	if err := json.Unmarshal(missing.Body.Bytes(), &errBody); err != nil {
		t.Fatal(err)
	}
	if errBody["error"] == "" {
		t.Fatal("empty error")
	}

	bad := get(mux, "/meters/not-a-meter")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad id %d", bad.Code)
	}

	series := get(mux, "/meters/M-109/readings")
	if series.Code != http.StatusOK {
		t.Fatalf("readings %d", series.Code)
	}
	var body struct {
		Readings []struct {
			KWh float64 `json:"consumption_kwh"`
		} `json:"readings"`
	}
	if err := json.Unmarshal(series.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Readings) != 336 {
		t.Fatalf("readings %d", len(body.Readings))
	}
}

func get(mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func repoData(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "data")
}
