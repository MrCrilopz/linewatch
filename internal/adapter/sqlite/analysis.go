package sqlite

import (
	"database/sql"
	"errors"
	"strconv"
	"time"

	"linewatch/internal/domain"
)

func (db *DB) CreateAnalysis() (domain.Analysis, error) {
	now := time.Now().UTC()
	res, err := db.sql.Exec(
		`INSERT INTO analysis(started_at, status) VALUES (?, 'running')`,
		now.Format(time.RFC3339),
	)
	if err != nil {
		return domain.Analysis{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Analysis{}, err
	}
	return domain.Analysis{ID: strconv.FormatInt(id, 10), Status: "running", StartedAt: now}, nil
}

func (db *DB) Analysis(id string) (domain.Analysis, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return domain.Analysis{}, domain.ErrNotFound
	}
	var row domain.Analysis
	var started, finished sql.NullString
	var rowID int64
	err = db.sql.QueryRow(
		`SELECT id, status, anomaly_count, high_priority_count, started_at, finished_at FROM analysis WHERE id = ?`,
		n,
	).Scan(&rowID, &row.Status, &row.AnomalyCount, &row.HighPriorityCount, &started, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Analysis{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Analysis{}, err
	}
	row.ID = strconv.FormatInt(rowID, 10)
	row.StartedAt, _ = time.Parse(time.RFC3339, started.String)
	if finished.Valid && finished.String != "" {
		ts, _ := time.Parse(time.RFC3339, finished.String)
		row.FinishedAt = &ts
	}
	return row, nil
}

func (db *DB) FinishAnalysis(id, status string, items []domain.Anomaly) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	high := 0
	for _, item := range items {
		if item.Class.Severity == domain.High {
			high++
		}
	}
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM anomalies WHERE analysis_id = ?`, n); err != nil {
		return err
	}
	for i, item := range items {
		if _, err := tx.Exec(
			`INSERT INTO anomalies(analysis_id, meter_id, type, severity, confidence, excess, reason, recommended_action, position) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			n, item.MeterID, item.Class.Type, item.Class.Severity, item.Class.Confidence, item.Class.Excess, item.Reason, item.Action, i,
		); err != nil {
			return err
		}
	}
	finished := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(
		`UPDATE analysis SET status = ?, finished_at = ?, anomaly_count = ?, high_priority_count = ? WHERE id = ?`,
		status, finished, len(items), high, n,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) Reason(analysisID, meterID string) (string, error) {
	n, err := strconv.ParseInt(analysisID, 10, 64)
	if err != nil {
		return "", err
	}
	var reason string
	err = db.sql.QueryRow(`SELECT reason FROM anomalies WHERE analysis_id = ? AND meter_id = ?`, n, meterID).Scan(&reason)
	return reason, err
}

func (db *DB) Summary() (domain.Summary, error) {
	var sum domain.Summary
	sum.AnalysisStatus = "none"
	if err := db.sql.QueryRow(`SELECT COUNT(*) FROM meters`).Scan(&sum.MeterCount); err != nil {
		return domain.Summary{}, err
	}
	if err := db.sql.QueryRow(`SELECT COALESCE(SUM(consumption_kwh), 0) FROM readings`).Scan(&sum.PeriodConsumptionKWh); err != nil {
		return domain.Summary{}, err
	}
	row, err := db.latestAnalysis()
	if errors.Is(err, domain.ErrNotFound) {
		return sum, nil
	}
	if err != nil {
		return domain.Summary{}, err
	}
	sum.AnalysisStatus = row.Status
	sum.AnomalyCount = row.AnomalyCount
	sum.HighPriorityCount = row.HighPriorityCount
	when := row.StartedAt
	if row.FinishedAt != nil {
		when = *row.FinishedAt
	}
	sum.LastAnalysisAt = &when
	if row.Status == "success" {
		if err := db.sql.QueryRow(`SELECT COALESCE(AVG(confidence), 0) FROM anomalies WHERE analysis_id = ?`, row.ID).Scan(&sum.Confidence); err != nil {
			return domain.Summary{}, err
		}
	}
	return sum, nil
}

func (db *DB) Anomalies() ([]domain.AnomalyView, error) {
	row, err := db.latestAnalysis()
	if errors.Is(err, domain.ErrNotFound) || (err == nil && row.Status != "success") {
		return []domain.AnomalyView{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := db.sql.Query(`
		SELECT id, meter_id, type, severity, confidence, reason, recommended_action
		FROM anomalies WHERE analysis_id = ? ORDER BY position
	`, row.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AnomalyView
	for rows.Next() {
		var item domain.AnomalyView
		var id int64
		if err := rows.Scan(&id, &item.MeterID, &item.Type, &item.Severity, &item.Confidence, &item.Reason, &item.RecommendedAction); err != nil {
			return nil, err
		}
		item.ID = strconv.FormatInt(id, 10)
		item.Anomaly = true
		out = append(out, item)
	}
	if out == nil {
		out = []domain.AnomalyView{}
	}
	return out, rows.Err()
}

func (db *DB) Anomaly(id string) (domain.AnomalyView, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return domain.AnomalyView{}, domain.ErrNotFound
	}
	var item domain.AnomalyView
	err = db.sql.QueryRow(`
		SELECT id, meter_id, type, severity, confidence, reason, recommended_action
		FROM anomalies WHERE id = ?
	`, n).Scan(&n, &item.MeterID, &item.Type, &item.Severity, &item.Confidence, &item.Reason, &item.RecommendedAction)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AnomalyView{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.AnomalyView{}, err
	}
	item.ID = strconv.FormatInt(n, 10)
	item.Anomaly = true
	readings, err := db.Readings(item.MeterID)
	if err != nil {
		return domain.AnomalyView{}, err
	}
	events, err := db.Events(item.MeterID)
	if err != nil {
		return domain.AnomalyView{}, err
	}
	class, ok := domain.Classify(readings, events)
	if ok {
		ev := domain.EvidenceFor(readings, events, class)
		item.BaselineKWh = ev.BaselineKWh
		item.ActualKWh = ev.ActualKWh
		item.VariationPct = ev.VariationPct
		item.EventType = ev.EventType
		item.EventDescription = ev.EventDescription
		item.VoltageV = ev.VoltageV
		item.CurrentA = ev.CurrentA
		item.PowerFactor = ev.PowerFactor
		item.Signals = ev.Signals
	}
	return item, nil
}

func (db *DB) latestAnalysis() (domain.Analysis, error) {
	var rowID int64
	err := db.sql.QueryRow(`SELECT id FROM analysis ORDER BY id DESC LIMIT 1`).Scan(&rowID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Analysis{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Analysis{}, err
	}
	return db.Analysis(strconv.FormatInt(rowID, 10))
}

func (db *DB) AnomalyMeters(analysisID string) ([]string, error) {
	n, err := strconv.ParseInt(analysisID, 10, 64)
	if err != nil {
		return nil, err
	}
	rows, err := db.sql.Query(`SELECT meter_id, severity FROM anomalies WHERE analysis_id = ? ORDER BY position`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id, severity string
		if err := rows.Scan(&id, &severity); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
