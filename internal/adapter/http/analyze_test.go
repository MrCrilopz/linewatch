package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"linewatch/internal/adapter/sqlite"
	"linewatch/internal/app"
	"linewatch/internal/domain"
)

type holdStore struct {
	*sqlite.DB
	hold chan struct{}
}

func (h holdStore) FinishAnalysis(id, status string, items []domain.Anomaly) error {
	<-h.hold
	return h.DB.FinishAnalysis(id, status, items)
}

func TestAnalysisOrderAndStates(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	dir := repoData(t)
	if err := db.Load(filepath.Join(dir, "readings.csv"), filepath.Join(dir, "events.csv")); err != nil {
		t.Fatal(err)
	}
	held := holdStore{DB: db, hold: make(chan struct{})}
	mux := NewMux(db, app.NewRunner(held, nil), "test-secret")

	before := get(mux, "/dashboard/summary")
	if before.Code != http.StatusOK {
		t.Fatalf("summary before %d", before.Code)
	}
	var empty domain.Summary
	if err := json.Unmarshal(before.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if empty.AnalysisStatus != "none" || empty.AnomalyCount != 0 || empty.MeterCount != 12 {
		t.Fatalf("before %+v", empty)
	}

	post := postJSON(mux, "/ai/analyze")
	if post.Code != http.StatusOK {
		t.Fatalf("start %d %s", post.Code, post.Body.String())
	}
	var started domain.Analysis
	if err := json.Unmarshal(post.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	if started.Status != "running" {
		t.Fatalf("status %s", started.Status)
	}
	busy := postJSON(mux, "/ai/analyze")
	if busy.Code != http.StatusConflict {
		t.Fatalf("busy %d", busy.Code)
	}
	close(held.hold)

	deadline := time.Now().Add(5 * time.Second)
	var done domain.Analysis
	for {
		rec := get(mux, "/ai/analysis/"+started.ID)
		if rec.Code != http.StatusOK {
			t.Fatalf("get %d %s", rec.Code, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &done); err != nil {
			t.Fatal(err)
		}
		if done.Status == "success" || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if done.Status != "success" || done.AnomalyCount != 4 || done.HighPriorityCount != 2 {
		t.Fatalf("done %+v", done)
	}
	ids, err := db.AnomalyMeters(started.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) < 2 || ids[0] != "M-109" || ids[1] != "M-112" {
		t.Fatalf("order %v", ids)
	}
	reason, err := db.Reason(started.ID, "M-109")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reason, "kWh") || !strings.Contains(reason, "%") {
		t.Fatalf("reason %q", reason)
	}
	quality, err := db.Reason(started.ID, "M-112")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(quality, "V") || !strings.Contains(quality, "A") {
		t.Fatalf("quality %q", quality)
	}
	if get(mux, "/ai/analysis/999").Code != http.StatusNotFound {
		t.Fatal("missing")
	}
	sumRec := get(mux, "/dashboard/summary")
	var sum domain.Summary
	if err := json.Unmarshal(sumRec.Body.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.AnalysisStatus != "success" || sum.AnomalyCount != 4 || sum.HighPriorityCount != 2 || sum.MeterCount != 12 || sum.Confidence <= 0 || sum.PeriodConsumptionKWh <= 0 {
		t.Fatalf("summary %+v", sum)
	}
	listRec := get(mux, "/anomalies")
	var list struct {
		Anomalies []domain.AnomalyView `json:"anomalies"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Anomalies) != 4 || list.Anomalies[0].MeterID != "M-109" || list.Anomalies[0].Type != "REAL_ANOMALY" {
		t.Fatalf("anomalies %+v", list.Anomalies)
	}
	detail := get(mux, "/anomalies/"+list.Anomalies[0].ID)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail %d %s", detail.Code, detail.Body.String())
	}
	var one domain.AnomalyView
	if err := json.Unmarshal(detail.Body.Bytes(), &one); err != nil {
		t.Fatal(err)
	}
	if one.BaselineKWh <= 0 || one.RecommendedAction == "" {
		t.Fatalf("detail %+v", one)
	}
	if get(mux, "/anomalies/999").Code != http.StatusNotFound {
		t.Fatal("missing anomaly")
	}
}

func postJSON(mux http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("Authorization", bearer())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}
