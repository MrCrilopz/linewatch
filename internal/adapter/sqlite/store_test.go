package sqlite

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadCounts(t *testing.T) {
	db := openTemp(t)
	dir := repoData(t)
	if err := db.Load(filepath.Join(dir, "readings.csv"), filepath.Join(dir, "events.csv")); err != nil {
		t.Fatal(err)
	}
	meters, readings, events, err := db.Counts()
	if err != nil {
		t.Fatal(err)
	}
	if meters != 12 || readings != 4032 || events != 4 {
		t.Fatalf("counts meters=%d readings=%d events=%d", meters, readings, events)
	}
}

func TestLoadRejectsPowerFactor(t *testing.T) {
	db := openTemp(t)
	dir := t.TempDir()
	readings := filepath.Join(dir, "readings.csv")
	events := filepath.Join(dir, "events.csv")
	if err := os.WriteFile(readings, []byte("meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status\nM-101,2026-09-01 00:00,20,220,40,1.4,OK\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(events, []byte("meter_id,event_timestamp,event_type,description\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := db.Load(readings, events); err == nil {
		t.Fatal("expected error")
	}
}

func openTemp(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func repoData(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "data")
}
