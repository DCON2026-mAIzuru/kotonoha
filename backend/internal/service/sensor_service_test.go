package service

import (
	"testing"
	"time"

	"github.com/apple/kotonoha/backend/internal/models"
)

func TestNormalizeSensorReadingMarksInvalidOutOfRangeReadings(t *testing.T) {
	t.Parallel()

	status, moisture, light, temperature, issues := normalizeSensorReading(IngestSensorInput{
		RecordedAt:         time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		MoisturePercent:    150,
		LightLux:           -20,
		TemperatureCelsius: -127,
	}, time.Date(2026, 4, 14, 12, 1, 0, 0, time.UTC))

	if status != sensorStatusInvalid {
		t.Fatalf("expected %q, got %q", sensorStatusInvalid, status)
	}
	if moisture != 100 || light != 0 || temperature != -50 {
		t.Fatalf("unexpected sanitized values: moisture=%v light=%v temperature=%v", moisture, light, temperature)
	}
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(issues))
	}
}

func TestShouldTriggerWateringEventSkipsInvalidReadings(t *testing.T) {
	t.Parallel()

	previous := &models.SensorData{
		MoisturePercent: 20,
		RecordedAt:      time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		SensorStatus:    sensorStatusNormal,
	}
	current := &models.SensorData{
		MoisturePercent: 45,
		RecordedAt:      time.Date(2026, 4, 14, 12, 2, 0, 0, time.UTC),
		SensorStatus:    sensorStatusInvalid,
	}

	if shouldTriggerWateringEvent(previous, current) {
		t.Fatal("watering event should not trigger for invalid sensor readings")
	}
}
