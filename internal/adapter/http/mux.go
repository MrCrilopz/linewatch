package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"linewatch/internal/app"
	"linewatch/internal/domain"
)

var meterID = regexp.MustCompile(`^M-[0-9]+$`)

type Store interface {
	Meters() ([]domain.Meter, error)
	Meter(id string) (domain.Meter, error)
	Readings(id string) ([]domain.Reading, error)
	Summary() (domain.Summary, error)
	Anomalies(lang string) ([]domain.AnomalyView, error)
	Anomaly(id, lang string) (domain.AnomalyView, error)
}

func NewMux(store Store, runner *app.Runner, secret string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login", login(secret))
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /ai/analyze", func(w http.ResponseWriter, r *http.Request) {
		startAnalysis(runner, w, r)
	})
	mux.HandleFunc("GET /ai/analysis/{id}", func(w http.ResponseWriter, r *http.Request) {
		showAnalysis(runner, w, r)
	})
	mux.HandleFunc("GET /dashboard/summary", func(w http.ResponseWriter, r *http.Request) {
		dashboard(store, w)
	})
	mux.HandleFunc("GET /anomalies", func(w http.ResponseWriter, r *http.Request) {
		listAnomalies(store, w, r)
	})
	mux.HandleFunc("GET /anomalies/{id}", func(w http.ResponseWriter, r *http.Request) {
		anomaly(store, w, r)
	})
	mux.HandleFunc("GET /meters", func(w http.ResponseWriter, r *http.Request) {
		listMeters(store, w, r)
	})
	mux.HandleFunc("GET /meters/{meterId}/readings", func(w http.ResponseWriter, r *http.Request) {
		meterReadings(store, w, r)
	})
	mux.HandleFunc("GET /meters/{meterId}", func(w http.ResponseWriter, r *http.Request) {
		meter(store, w, r)
	})
	return guard(secret, mux)
}

func startAnalysis(runner *app.Runner, w http.ResponseWriter, r *http.Request) {
	if runner == nil {
		writeError(w, http.StatusInternalServerError, "analysis")
		return
	}
	row, err := runner.Start(langOf(r))
	if errors.Is(err, app.ErrBusy) {
		writeError(w, http.StatusConflict, "analysis running")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "analysis")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func showAnalysis(runner *app.Runner, w http.ResponseWriter, r *http.Request) {
	if runner == nil {
		writeError(w, http.StatusInternalServerError, "analysis")
		return
	}
	id := r.PathValue("id")
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid analysis id")
		return
	}
	row, err := runner.Store.Analysis(id)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "analysis")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func dashboard(store Store, w http.ResponseWriter) {
	if store == nil {
		writeError(w, http.StatusInternalServerError, "store")
		return
	}
	sum, err := store.Summary()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func listAnomalies(store Store, w http.ResponseWriter, r *http.Request) {
	if store == nil {
		writeError(w, http.StatusInternalServerError, "store")
		return
	}
	items, err := store.Anomalies(langOf(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"anomalies": items})
}

func anomaly(store Store, w http.ResponseWriter, r *http.Request) {
	if store == nil {
		writeError(w, http.StatusInternalServerError, "store")
		return
	}
	id := r.PathValue("id")
	if !regexp.MustCompile(`^[0-9]+$`).MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid anomaly id")
		return
	}
	item, err := store.Anomaly(id, langOf(r))
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func langOf(r *http.Request) string {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Accept-Language")), "en") {
		return "en"
	}
	return "es"
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func listMeters(store Store, w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	q := r.URL.Query().Get("q")
	order := r.URL.Query().Get("sort")
	if status == "" {
		status = "all"
	}
	if order == "" {
		order = "consumption"
	}
	switch status {
	case "all", "ok", "alert", "critical":
	default:
		writeError(w, http.StatusBadRequest, "invalid query")
		return
	}
	switch order {
	case "consumption", "variation", "severity":
	default:
		writeError(w, http.StatusBadRequest, "invalid query")
		return
	}
	meters, err := store.Meters()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store")
		return
	}
	out := make([]domain.Meter, 0, len(meters))
	for _, m := range meters {
		if q != "" && !strings.Contains(m.ID, q) {
			continue
		}
		if status != "all" && m.Status != status {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if order == "variation" && out[i].Variation != out[j].Variation {
			return out[i].Variation > out[j].Variation
		}
		if order == "severity" && statusRank(out[i].Status) != statusRank(out[j].Status) {
			return statusRank(out[i].Status) > statusRank(out[j].Status)
		}
		if order == "consumption" && out[i].ConsumptionKWh != out[j].ConsumptionKWh {
			return out[i].ConsumptionKWh > out[j].ConsumptionKWh
		}
		return out[i].ID < out[j].ID
	})
	writeJSON(w, http.StatusOK, map[string]any{"meters": out})
}

func statusRank(status string) int {
	switch status {
	case "critical":
		return 3
	case "alert":
		return 2
	default:
		return 1
	}
}

func meter(store Store, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("meterId")
	if !meterID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid meter id")
		return
	}
	m, err := store.Meter(id)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func meterReadings(store Store, w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("meterId")
	if !meterID.MatchString(id) {
		writeError(w, http.StatusBadRequest, "invalid meter id")
		return
	}
	rows, err := store.Readings(id)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"readings": rows})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
