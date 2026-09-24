package domain

import "time"

type Meter struct {
	ID string
}

type Reading struct {
	MeterID        string
	Timestamp      time.Time
	ConsumptionKWh float64
	VoltageV       float64
	CurrentA       float64
	PowerFactor    float64
	Status         string
}

type Event struct {
	MeterID     string
	Timestamp   time.Time
	Type        string
	Description string
}
