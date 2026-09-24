package domain

import "sort"

const (
	RealAnomaly   = "REAL_ANOMALY"
	Explainable   = "EXPLAINABLE"
	FalsePositive = "FALSE_POSITIVE"
	DataQuality   = "DATA_QUALITY"

	High   = "HIGH"
	Medium = "MEDIUM"
	Low    = "LOW"
)

type Class struct {
	Type       string
	Severity   string
	Confidence float64
	Signals    []string
}

func Classify(readings []Reading, events []Event) (Class, bool) {
	days := map[string]float64{}
	for _, row := range readings {
		days[row.Timestamp.UTC().Format("2006-01-02")] += row.ConsumptionKWh
	}
	if len(days) == 0 {
		return Class{}, false
	}
	all := make([]float64, 0, len(days))
	for _, sum := range days {
		all = append(all, sum)
	}
	level := median(all)
	if level == 0 {
		return Class{}, false
	}
	stable := true
	for _, sum := range days {
		if abs((sum-level)/level) > 0.15 {
			stable = false
			break
		}
	}
	if stable && powerBroken(readings) {
		return Class{
			Type:       DataQuality,
			Severity:   High,
			Confidence: 1,
			Signals:    []string{"consumption_near_baseline", "electrical_inconsistency"},
		}, true
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
	base := level
	if len(sample) > 0 {
		base = median(sample)
	}
	if base == 0 {
		return Class{}, false
	}

	drops := map[string]struct{}{}
	rises := map[string]struct{}{}
	spikes := map[string]struct{}{}
	for day, sum := range days {
		rel := (sum - base) / base
		switch {
		case rel <= -0.25:
			drops[day] = struct{}{}
		case rel >= 1:
			spikes[day] = struct{}{}
			rises[day] = struct{}{}
		case rel >= 0.25:
			rises[day] = struct{}{}
		}
	}

	if len(drops) > 0 && overlaps(drops, events, "SCHEDULED_OUTAGE") {
		return Class{
			Type:       FalsePositive,
			Severity:   Low,
			Confidence: 1,
			Signals:    []string{"consumption_change", "scheduled_outage"},
		}, true
	}
	if len(rises) > 0 && overlaps(rises, events, "OPERATIONAL_CHANGE") {
		return Class{
			Type:       Explainable,
			Severity:   Medium,
			Confidence: 1,
			Signals:    []string{"consumption_increase", "operational_change"},
		}, true
	}
	if len(spikes) > 0 && !explains(events) && electricalShift(readings, spikes) {
		return Class{
			Type:       RealAnomaly,
			Severity:   High,
			Confidence: 1,
			Signals:    []string{"consumption_above_baseline", "no_explaining_event", "electrical_change"},
		}, true
	}
	return Class{}, false
}

func overlaps(days map[string]struct{}, events []Event, kind string) bool {
	for _, ev := range events {
		if ev.Type != kind {
			continue
		}
		day := ev.Timestamp.UTC().Format("2006-01-02")
		if _, ok := days[day]; ok {
			return true
		}
		for got := range days {
			if got >= day {
				return true
			}
		}
	}
	return false
}

func explains(events []Event) bool {
	for _, ev := range events {
		if ev.Type == "SCHEDULED_OUTAGE" || ev.Type == "OPERATIONAL_CHANGE" {
			return true
		}
	}
	return false
}

func electricalShift(readings []Reading, spikes map[string]struct{}) bool {
	var baseI, spikeI []float64
	for _, row := range readings {
		day := row.Timestamp.UTC().Format("2006-01-02")
		if _, ok := spikes[day]; ok {
			spikeI = append(spikeI, row.CurrentA)
			continue
		}
		baseI = append(baseI, row.CurrentA)
	}
	if len(baseI) == 0 || len(spikeI) == 0 {
		return false
	}
	return median(spikeI) >= median(baseI)*1.5
}

func powerBroken(readings []Reading) bool {
	for _, row := range readings {
		watts := row.VoltageV * row.CurrentA * row.PowerFactor
		if watts == 0 {
			return true
		}
		ratio := row.ConsumptionKWh / (watts / 1000)
		if ratio < 0.6 || ratio > 1.8 {
			return true
		}
	}
	return false
}

func median(values []float64) float64 {
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2
}

func abs(n float64) float64 {
	if n < 0 {
		return -n
	}
	return n
}
