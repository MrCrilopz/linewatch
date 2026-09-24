package domain

import "sort"

func Profile(readings []Reading, events []Event) (baseline, actual, variation float64, status string) {
	days := map[string]float64{}
	for _, row := range readings {
		days[row.Timestamp.UTC().Format("2006-01-02")] += row.ConsumptionKWh
		actual += row.ConsumptionKWh
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
	if len(sample) == 0 {
		return actual, actual, 0, "ok"
	}
	sort.Float64s(sample)
	mid := len(sample) / 2
	daily := sample[mid]
	if len(sample)%2 == 0 {
		daily = (sample[mid-1] + sample[mid]) / 2
	}
	baseline = daily * float64(len(days))
	if baseline == 0 {
		return baseline, actual, 0, "ok"
	}
	variation = (actual - baseline) / baseline
	switch {
	case variation >= 0.75 || variation <= -0.75:
		status = "critical"
	case variation >= 0.15 || variation <= -0.15:
		status = "alert"
	default:
		status = "ok"
	}
	return baseline, actual, variation, status
}
