package domain

import (
	"math"
	"testing"
	"time"
)

func TestProfileVariation(t *testing.T) {
	day := func(date string, kwh float64) Reading {
		ts, err := time.Parse("2006-01-02", date)
		if err != nil {
			t.Fatal(err)
		}
		return Reading{Timestamp: ts, ConsumptionKWh: kwh}
	}
	flatBase, flatActual, flatVar, flatStatus := Profile([]Reading{
		day("2026-09-01", 10),
		day("2026-09-02", 10),
		day("2026-09-03", 10),
	}, nil)
	if flatBase != 30 || flatActual != 30 || flatVar != 0 || flatStatus != "ok" {
		t.Fatalf("flat base=%v actual=%v var=%v status=%s", flatBase, flatActual, flatVar, flatStatus)
	}

	eventAt, err := time.Parse("2006-01-02 15:04", "2026-09-02 00:00")
	if err != nil {
		t.Fatal(err)
	}
	base, actual, variation, status := Profile([]Reading{
		day("2026-09-01", 10),
		day("2026-09-02", 40),
		day("2026-09-03", 12),
	}, []Event{{Timestamp: eventAt}})
	if base != 33 || actual != 62 || status != "critical" {
		t.Fatalf("base=%v actual=%v status=%s", base, actual, status)
	}
	if math.Abs(variation-(29.0/33.0)) > 1e-9 {
		t.Fatalf("variation %v", variation)
	}
}
