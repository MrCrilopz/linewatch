package app

import (
	"errors"
	"log"
	"sort"

	"linewatch/internal/domain"
)

var ErrBusy = errors.New("analysis running")

type Catalog interface {
	Meters() ([]domain.Meter, error)
	Readings(id string) ([]domain.Reading, error)
	Events(id string) ([]domain.Event, error)
	CreateAnalysis() (domain.Analysis, error)
	FinishAnalysis(id, status string, items []domain.Anomaly) error
	Analysis(id string) (domain.Analysis, error)
	AnomalyMeters(analysisID string) ([]string, error)
}

type Explainer interface {
	Explain(domain.Evidence) (string, string)
}

type Runner struct {
	Store   Catalog
	Explain Explainer
	busy    chan struct{}
}

func NewRunner(store Catalog, explain Explainer) *Runner {
	return &Runner{Store: store, Explain: explain, busy: make(chan struct{}, 1)}
}

func (r *Runner) Start(lang string) (domain.Analysis, error) {
	select {
	case r.busy <- struct{}{}:
	default:
		return domain.Analysis{}, ErrBusy
	}
	row, err := r.Store.CreateAnalysis()
	if err != nil {
		<-r.busy
		return domain.Analysis{}, err
	}
	log.Printf("analysis id=%s status=running", row.ID)
	go r.finish(row.ID, lang)
	return row, nil
}

func (r *Runner) finish(id, lang string) {
	defer func() { <-r.busy }()
	items, err := r.detect(lang)
	status := "success"
	if err != nil {
		status = "error"
		items = nil
		log.Printf("analysis id=%s status=error", id)
	}
	if err := r.Store.FinishAnalysis(id, status, items); err != nil {
		log.Printf("analysis id=%s status=error", id)
		return
	}
	if status == "success" {
		log.Printf("analysis id=%s status=success anomalies=%d", id, len(items))
	}
}

func (r *Runner) detect(lang string) ([]domain.Anomaly, error) {
	meters, err := r.Store.Meters()
	if err != nil {
		return nil, err
	}
	var items []domain.Anomaly
	for _, meter := range meters {
		readings, err := r.Store.Readings(meter.ID)
		if err != nil {
			return nil, err
		}
		events, err := r.Store.Events(meter.ID)
		if err != nil {
			return nil, err
		}
		class, ok := domain.Classify(readings, events)
		if !ok {
			continue
		}
		ev := domain.EvidenceFor(readings, events, class)
		ev.MeterID = meter.ID
		ev.Language = lang
		reason, action := domain.Template(ev)
		if r.Explain != nil {
			reason, action = r.Explain.Explain(ev)
		}
		items = append(items, domain.Anomaly{
			MeterID: meter.ID,
			Class:   class,
			Reason:  reason,
			Action:  action,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if rank(items[i].Class.Severity) != rank(items[j].Class.Severity) {
			return rank(items[i].Class.Severity) > rank(items[j].Class.Severity)
		}
		if items[i].Class.Excess != items[j].Class.Excess {
			return items[i].Class.Excess > items[j].Class.Excess
		}
		return items[i].MeterID < items[j].MeterID
	})
	return items, nil
}

func rank(severity string) int {
	switch severity {
	case domain.High:
		return 3
	case domain.Medium:
		return 2
	default:
		return 1
	}
}
