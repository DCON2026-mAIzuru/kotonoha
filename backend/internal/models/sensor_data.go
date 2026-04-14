package models

import "time"

type SensorData struct {
	UserID             string         `json:"user_id"`
	DeviceID           string         `json:"device_id"`
	MoisturePercent    float64        `json:"moisture_percent"`
	LightLux           float64        `json:"light_lux"`
	TemperatureCelsius float64        `json:"temperature_celsius"`
	RecordedAt         time.Time      `json:"recorded_at"`
	SensorStatus       string         `json:"sensor_status"`
	RawPayload         map[string]any `json:"raw_payload,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type UpsertSensorDataInput struct {
	UserID             string         `json:"-"`
	DeviceID           string         `json:"device_id"`
	MoisturePercent    float64        `json:"moisture_percent"`
	LightLux           float64        `json:"light_lux"`
	TemperatureCelsius float64        `json:"temperature_celsius"`
	RecordedAt         time.Time      `json:"recorded_at"`
	SensorStatus       string         `json:"-"`
	RawPayload         map[string]any `json:"-"`
}
