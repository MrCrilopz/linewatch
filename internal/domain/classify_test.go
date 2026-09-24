package domain

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestClassifyLabeledMeters(t *testing.T) {
	readings, events := loadCSV(t)
	want := map[string]Class{
		"M-109": {Type: RealAnomaly, Severity: High},
		"M-112": {Type: DataQuality, Severity: High},
		"M-104": {Type: Explainable, Severity: Medium},
		"M-106": {Type: FalsePositive, Severity: Low},
	}
	seen := map[string]bool{}
	for id, rows := range readings {
		class, ok := Classify(rows, events[id])
		if exp, labeled := want[id]; labeled {
			if !ok || class.Type != exp.Type || class.Severity != exp.Severity {
				t.Fatalf("%s got %+v ok=%v", id, class, ok)
			}
			if class.Confidence <= 0 || class.Confidence > 1 {
				t.Fatalf("%s confidence %v", id, class.Confidence)
			}
			seen[id] = true
			continue
		}
		if ok {
			t.Fatalf("%s classified %+v", id, class)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("labeled %d", len(seen))
	}
}

func loadCSV(t *testing.T) (map[string][]Reading, map[string][]Event) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "data")
	readings := map[string][]Reading{}
	rf, err := os.Open(filepath.Join(dir, "readings.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()
	rr := csv.NewReader(rf)
	header, err := rr.Read()
	if err != nil {
		t.Fatal(err)
	}
	_ = header
	for {
		rec, err := rr.Read()
		if err != nil {
			break
		}
		ts, err := time.Parse("2006-01-02 15:04:05", rec[1])
		if err != nil {
			t.Fatal(err)
		}
		kwh, _ := strconv.ParseFloat(rec[2], 64)
		v, _ := strconv.ParseFloat(rec[3], 64)
		i, _ := strconv.ParseFloat(rec[4], 64)
		pf, _ := strconv.ParseFloat(rec[5], 64)
		readings[rec[0]] = append(readings[rec[0]], Reading{
			MeterID: rec[0], Timestamp: ts.UTC(), ConsumptionKWh: kwh,
			VoltageV: v, CurrentA: i, PowerFactor: pf, Status: rec[6],
		})
	}
	events := map[string][]Event{}
	ef, err := os.Open(filepath.Join(dir, "events.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer ef.Close()
	er := csv.NewReader(ef)
	if _, err := er.Read(); err != nil {
		t.Fatal(err)
	}
	for {
		rec, err := er.Read()
		if err != nil {
			break
		}
		ts, err := time.Parse("2006-01-02 15:04", rec[1])
		if err != nil {
			t.Fatal(err)
		}
		events[rec[0]] = append(events[rec[0]], Event{
			MeterID: rec[0], Timestamp: ts.UTC(), Type: rec[2], Description: rec[3],
		})
	}
	return readings, events
}
