package domain

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Meter struct {
	ID             string  `json:"meter_id"`
	ConsumptionKWh float64 `json:"consumption_kwh"`
	Status         string  `json:"status"`
}

type Reading struct {
	MeterID        string    `json:"meter_id"`
	Timestamp      time.Time `json:"timestamp"`
	ConsumptionKWh float64   `json:"consumption_kwh"`
	VoltageV       float64   `json:"voltage_v"`
	CurrentA       float64   `json:"current_a"`
	PowerFactor    float64   `json:"power_factor"`
	Status         string    `json:"status"`
}

type Event struct {
	MeterID     string
	Timestamp   time.Time
	Type        string
	Description string
}
