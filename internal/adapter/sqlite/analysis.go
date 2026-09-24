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
