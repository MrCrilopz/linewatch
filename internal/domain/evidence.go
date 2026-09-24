package domain

import (
	"fmt"
	"time"
)

type Analysis struct {
	ID                string     `json:"id"`
	Status            string     `json:"status"`
	AnomalyCount      int        `json:"anomaly_count"`
	HighPriorityCount int        `json:"high_priority_count"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
}

type Anomaly struct {
	MeterID string
	Class   Class
	Reason  string
	Action  string
}

type AnomalyView struct {
	ID                string   `json:"id"`
	MeterID           string   `json:"meter_id"`
	Anomaly           bool     `json:"anomaly"`
	Type              string   `json:"type"`
	Severity          string   `json:"severity"`
	Confidence        float64  `json:"confidence"`
	Reason            string   `json:"reason"`
	RecommendedAction string   `json:"recommended_action"`
	BaselineKWh       float64  `json:"baseline_kwh,omitempty"`
	ActualKWh         float64  `json:"actual_kwh,omitempty"`
	VariationPct      float64  `json:"variation_pct,omitempty"`
	EventType         string   `json:"event_type,omitempty"`
	EventDescription  string   `json:"event_description,omitempty"`
	VoltageV          float64  `json:"voltage_v,omitempty"`
	CurrentA          float64  `json:"current_a,omitempty"`
	PowerFactor       float64  `json:"power_factor,omitempty"`
	Signals           []string `json:"signals,omitempty"`
}

type Summary struct {
	MeterCount           int        `json:"meter_count"`
	PeriodConsumptionKWh float64    `json:"period_consumption_kwh"`
	AnomalyCount         int        `json:"anomaly_count"`
	HighPriorityCount    int        `json:"high_priority_count"`
	Confidence           float64    `json:"confidence"`
	AnalysisStatus       string     `json:"analysis_status"`
	LastAnalysisAt       *time.Time `json:"last_analysis_at,omitempty"`
}

type Evidence struct {
	MeterID          string
	Type             string
	Severity         string
	Confidence       float64
	BaselineKWh      float64
	ActualKWh        float64
	VariationPct     float64
	EventType        string
	EventDescription string
	CurrentA         float64
	VoltageV         float64
	PowerFactor      float64
	Signals          []string
}

func EvidenceFor(readings []Reading, events []Event, class Class) Evidence {
	days := map[string]float64{}
	for _, row := range readings {
		days[row.Timestamp.UTC().Format("2006-01-02")] += row.ConsumptionKWh
	}
	skip := map[string]struct{}{}
	for _, ev := range events {
		skip[ev.Timestamp.UTC().Format("2006-01-02")] = struct{}{}
	}
	sample := make([]float64, 0, len(days))
	for day, sum := range days {
		if _, out := skip[day]; out {
			continue
		}
		sample = append(sample, sum)
	}
	base := median(sample)
	if len(sample) == 0 {
		all := make([]float64, 0, len(days))
		for _, sum := range days {
			all = append(all, sum)
		}
		base = median(all)
	}
	var actualDay string
	var actual float64
	for day, sum := range days {
		rel := 0.0
		if base != 0 {
			rel = abs((sum - base) / base)
		}
		if actualDay == "" || rel > absRel(actual, base) {
			actualDay = day
			actual = sum
		}
	}
	current, voltage, pf := medianCurrent(readings, actualDay), medianVoltage(readings, actualDay), medianPF(readings, actualDay)
	if class.Type == DataQuality {
		current, voltage, pf = worstElectrical(readings)
	}
	ev := Evidence{
		Type:        class.Type,
		Severity:    class.Severity,
		Confidence:  class.Confidence,
		BaselineKWh: base,
		ActualKWh:   actual,
		Signals:     class.Signals,
		CurrentA:    current,
		VoltageV:    voltage,
		PowerFactor: pf,
	}
	if base != 0 {
		ev.VariationPct = (actual - base) / base * 100
	}
	for _, event := range events {
		if event.Timestamp.UTC().Format("2006-01-02") == actualDay {
			ev.EventType = event.Type
			ev.EventDescription = event.Description
		}
	}
	if ev.EventType == "" && len(events) == 1 {
		ev.EventType = events[0].Type
		ev.EventDescription = events[0].Description
	}
	return ev
}

func absRel(actual, base float64) float64 {
	if base == 0 {
		return 0
	}
	return abs((actual - base) / base)
}

func worstElectrical(readings []Reading) (current, voltage, pf float64) {
	worst := -1.0
	for _, row := range readings {
		watts := row.VoltageV * row.CurrentA * row.PowerFactor
		gap := 1.0
		if watts != 0 {
			ratio := row.ConsumptionKWh / (watts / 1000)
			gap = abs(ratio - 1)
		}
		if gap > worst {
			worst = gap
			current, voltage, pf = row.CurrentA, row.VoltageV, row.PowerFactor
		}
	}
	return current, voltage, pf
}

func medianCurrent(readings []Reading, day string) float64 {
	var values []float64
	for _, row := range readings {
		if row.Timestamp.UTC().Format("2006-01-02") == day {
			values = append(values, row.CurrentA)
		}
	}
	return median(values)
}

func medianVoltage(readings []Reading, day string) float64 {
	var values []float64
	for _, row := range readings {
		if row.Timestamp.UTC().Format("2006-01-02") == day {
			values = append(values, row.VoltageV)
		}
	}
	return median(values)
}

func medianPF(readings []Reading, day string) float64 {
	var values []float64
	for _, row := range readings {
		if row.Timestamp.UTC().Format("2006-01-02") == day {
			values = append(values, row.PowerFactor)
		}
	}
	return median(values)
}

func Template(ev Evidence) (string, string) {
	switch ev.Type {
	case RealAnomaly:
		return fmt.Sprintf("Consumo %.1f%% por encima del baseline %.1f kWh sin evento conocido. Corriente %.1f A.", ev.VariationPct, ev.BaselineKWh, ev.CurrentA),
			"Investigar medidor e instalación."
	case Explainable:
		return fmt.Sprintf("Consumo %.1f%% por encima del baseline %.1f kWh con evento %s.", ev.VariationPct, ev.BaselineKWh, ev.EventType),
			"Validar la operación."
	case FalsePositive:
		return fmt.Sprintf("Consumo %.1f%% frente al baseline %.1f kWh durante %s.", ev.VariationPct, ev.BaselineKWh, ev.EventType),
			"No escalar."
	default:
		return fmt.Sprintf("Consumo %.1f kWh cerca del baseline %.1f kWh. Tensión %.1f V, corriente %.1f A y factor de potencia %.2f no cierran.", ev.ActualKWh, ev.BaselineKWh, ev.VoltageV, ev.CurrentA, ev.PowerFactor),
			"Validar las lecturas eléctricas."
	}
}
