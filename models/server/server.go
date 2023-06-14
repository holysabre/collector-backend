package server

import (
	model_ipmi "collector-agent/models/ipmi"
	"time"
)

type Server struct {
	ID           uint64                `json:"id"`
	CabinetID    uint64                `json:"cabinet_id"`
	Connection   model_ipmi.Connection `json:"connection"`
	PowerStatus  string                `json:"power_status"`
	PowerReading int                   `json:"power_reading"`
	Time         time.Time             `json:"time"`
}
