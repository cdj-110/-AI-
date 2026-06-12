package model

import "time"

type Reading struct {
	DeviceKey string                 `json:"deviceKey"`
	Time      time.Time              `json:"time"`
	Metrics   map[string]interface{} `json:"metrics"`
}

type PointValue struct {
	DeviceKey string
	Metric    string
	Value     interface{}
}
