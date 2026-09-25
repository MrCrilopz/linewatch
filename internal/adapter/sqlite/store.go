package sqlite

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	_ "modernc.org/sqlite"

	"linewatch/internal/domain"
)

const (
	timeLayout    = "2006-01-02 15:04:05"
	timeLayoutMin = "2006-01-02 15:04"
)

type DB struct {
	sql *sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`
		PRAGMA foreign_keys = ON;
		CREATE TABLE IF NOT EXISTS meters (
			meter_id TEXT PRIMARY KEY
		);
		CREATE TABLE IF NOT EXISTS readings (
			id INTEGER PRIMARY KEY,
			meter_id TEXT NOT NULL REFERENCES meters(meter_id),
			timestamp TEXT NOT NULL,
			consumption_kwh REAL NOT NULL,
			voltage_v REAL NOT NULL,
			current_a REAL NOT NULL,
			power_factor REAL NOT NULL,
			status TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY,
			meter_id TEXT NOT NULL REFERENCES meters(meter_id),
			event_timestamp TEXT NOT NULL,
			event_type TEXT NOT NULL,
			description TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS analysis (
			id INTEGER PRIMARY KEY,
			started_at TEXT NOT NULL,
			finished_at TEXT,
			status TEXT NOT NULL,
			anomaly_count INTEGER NOT NULL DEFAULT 0,
			high_priority_count INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS anomalies (
			id INTEGER PRIMARY KEY,
			analysis_id INTEGER NOT NULL REFERENCES analysis(id),
			meter_id TEXT NOT NULL,
			type TEXT NOT NULL,
			severity TEXT NOT NULL,
			confidence REAL NOT NULL,
			excess REAL NOT NULL,
			reason TEXT NOT NULL,
			recommended_action TEXT NOT NULL,
			position INTEGER NOT NULL
		);
	`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`UPDATE analysis SET status = 'error', finished_at = started_at WHERE status = 'running'`); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{sql: db}, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) Load(readingsPath, eventsPath string) error {
	readings, err := readReadings(readingsPath)
	if err != nil {
		return err
	}
	events, err := readEvents(eventsPath)
	if err != nil {
		return err
	}
	tx, err := db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM readings; DELETE FROM events; DELETE FROM meters;`); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, row := range readings {
		seen[row.MeterID] = struct{}{}
	}
	for _, row := range events {
		seen[row.MeterID] = struct{}{}
	}
	for id := range seen {
		if _, err := tx.Exec(`INSERT INTO meters(meter_id) VALUES (?)`, id); err != nil {
			return err
		}
	}
	for _, row := range readings {
		if _, err := tx.Exec(
			`INSERT INTO readings(meter_id, timestamp, consumption_kwh, voltage_v, current_a, power_factor, status) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			row.MeterID, row.Timestamp.Format(timeLayout), row.ConsumptionKWh, row.VoltageV, row.CurrentA, row.PowerFactor, row.Status,
		); err != nil {
			return err
		}
	}
	for _, row := range events {
		if _, err := tx.Exec(
			`INSERT INTO events(meter_id, event_timestamp, event_type, description) VALUES (?, ?, ?, ?)`,
			row.MeterID, row.Timestamp.Format(timeLayout), row.Type, row.Description,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) Meters() ([]domain.Meter, error) {
	rows, err := db.sql.Query(`SELECT meter_id FROM meters ORDER BY meter_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Meter
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		m, err := db.Meter(id)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (db *DB) Meter(id string) (domain.Meter, error) {
	var exists int
	err := db.sql.QueryRow(`SELECT COUNT(*) FROM meters WHERE meter_id = ?`, id).Scan(&exists)
	if err != nil {
		return domain.Meter{}, err
	}
	if exists == 0 {
		return domain.Meter{}, domain.ErrNotFound
	}
	readings, err := db.consumption(id)
	if err != nil {
		return domain.Meter{}, err
	}
	events, err := db.Events(id)
	if err != nil {
		return domain.Meter{}, err
	}
	baseline, actual, variation, status := domain.Profile(readings, events)
	return domain.Meter{
		ID:             id,
		ConsumptionKWh: actual,
		BaselineKWh:    baseline,
		Variation:      variation,
		Status:         status,
	}, nil
}

func (db *DB) consumption(id string) ([]domain.Reading, error) {
	rows, err := db.sql.Query(`SELECT timestamp, consumption_kwh FROM readings WHERE meter_id = ? ORDER BY timestamp`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Reading
	for rows.Next() {
		var raw string
		var rec domain.Reading
		if err := rows.Scan(&raw, &rec.ConsumptionKWh); err != nil {
			return nil, err
		}
		rec.Timestamp, err = parseTime(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (db *DB) Events(id string) ([]domain.Event, error) {
	rows, err := db.sql.Query(`SELECT event_timestamp, event_type, description FROM events WHERE meter_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Event
	for rows.Next() {
		var raw string
		var ev domain.Event
		if err := rows.Scan(&raw, &ev.Type, &ev.Description); err != nil {
			return nil, err
		}
		ev.Timestamp, err = parseTime(raw)
		if err != nil {
			return nil, err
		}
		ev.MeterID = id
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (db *DB) Readings(id string) ([]domain.Reading, error) {
	var exists int
	if err := db.sql.QueryRow(`SELECT COUNT(*) FROM meters WHERE meter_id = ?`, id).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, domain.ErrNotFound
	}
	rows, err := db.sql.Query(`
		SELECT timestamp, consumption_kwh, voltage_v, current_a, power_factor, status
		FROM readings
		WHERE meter_id = ?
		ORDER BY timestamp
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Reading
	for rows.Next() {
		var raw string
		var rec domain.Reading
		if err := rows.Scan(&raw, &rec.ConsumptionKWh, &rec.VoltageV, &rec.CurrentA, &rec.PowerFactor, &rec.Status); err != nil {
			return nil, err
		}
		rec.Timestamp, err = parseTime(raw)
		if err != nil {
			return nil, err
		}
		rec.MeterID = id
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (db *DB) Counts() (meters, readings, events int, err error) {
	err = db.sql.QueryRow(`SELECT (SELECT COUNT(*) FROM meters), (SELECT COUNT(*) FROM readings), (SELECT COUNT(*) FROM events)`).Scan(&meters, &readings, &events)
	return meters, readings, events, err
}

func readReadings(path string) ([]domain.Reading, error) {
	rows, err := openCSV(path)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Reading, 0, len(rows))
	for i, row := range rows {
		rec, err := parseReading(row)
		if err != nil {
			return nil, fmt.Errorf("readings row %d: %w", i+2, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

func readEvents(path string) ([]domain.Event, error) {
	rows, err := openCSV(path)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Event, 0, len(rows))
	for i, row := range rows {
		rec, err := parseEvent(row)
		if err != nil {
			return nil, fmt.Errorf("events row %d: %w", i+2, err)
		}
		out = append(out, rec)
	}
	return out, nil
}

func openCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	var rows []map[string]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		row := make(map[string]string, len(header))
		for i, name := range header {
			if i < len(rec) {
				row[name] = rec[i]
			}
		}
		rows = append(rows, row)
	}
}

func parseReading(row map[string]string) (domain.Reading, error) {
	ts, err := parseTime(row["timestamp"])
	if err != nil {
		return domain.Reading{}, fmt.Errorf("timestamp")
	}
	kwh, err := parseNonNegative(row["consumption_kwh"])
	if err != nil {
		return domain.Reading{}, fmt.Errorf("consumption_kwh")
	}
	voltage, err := parseRange(row["voltage_v"], 0, 1000, false)
	if err != nil {
		return domain.Reading{}, fmt.Errorf("voltage_v")
	}
	current, err := parseNonNegative(row["current_a"])
	if err != nil {
		return domain.Reading{}, fmt.Errorf("current_a")
	}
	pf, err := parseRange(row["power_factor"], 0, 1, true)
	if err != nil {
		return domain.Reading{}, fmt.Errorf("power_factor")
	}
	if row["meter_id"] == "" || row["status"] == "" {
		return domain.Reading{}, fmt.Errorf("meter_id")
	}
	return domain.Reading{
		MeterID:        row["meter_id"],
		Timestamp:      ts.UTC(),
		ConsumptionKWh: kwh,
		VoltageV:       voltage,
		CurrentA:       current,
		PowerFactor:    pf,
		Status:         row["status"],
	}, nil
}

func parseEvent(row map[string]string) (domain.Event, error) {
	ts, err := parseTime(row["event_timestamp"])
	if err != nil {
		return domain.Event{}, fmt.Errorf("event_timestamp")
	}
	if row["meter_id"] == "" || row["event_type"] == "" {
		return domain.Event{}, fmt.Errorf("meter_id")
	}
	return domain.Event{
		MeterID:     row["meter_id"],
		Timestamp:   ts.UTC(),
		Type:        row["event_type"],
		Description: row["description"],
	}, nil
}

func parseTime(raw string) (time.Time, error) {
	if ts, err := time.Parse(timeLayout, raw); err == nil {
		return ts, nil
	}
	return time.Parse(timeLayoutMin, raw)
}

func parseNonNegative(raw string) (float64, error) {
	return parseRange(raw, 0, 1e9, true)
}

func parseRange(raw string, min, max float64, includeMin bool) (float64, error) {
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if includeMin {
		if n < min || n > max {
			return 0, fmt.Errorf("range")
		}
	} else if n <= min || n > max {
		return 0, fmt.Errorf("range")
	}
	return n, nil
}
