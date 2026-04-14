package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/apple/kotonoha/backend/internal/models"
)

func (s *PostgresStore) GetSensorDataByUserID(ctx context.Context, userID string) (*models.SensorData, error) {
	const query = `
		select
			user_id,
			device_id,
			moisture_percent,
			light_lux,
			temperature_celsius,
			recorded_at,
			sensor_status,
			raw_payload,
			created_at,
			updated_at
		from public.sensor_data
		where user_id = $1
	`

	var sensorData models.SensorData
	var raw []byte
	if err := s.pool.QueryRow(ctx, query, userID).Scan(
		&sensorData.UserID,
		&sensorData.DeviceID,
		&sensorData.MoisturePercent,
		&sensorData.LightLux,
		&sensorData.TemperatureCelsius,
		&sensorData.RecordedAt,
		&sensorData.SensorStatus,
		&raw,
		&sensorData.CreatedAt,
		&sensorData.UpdatedAt,
	); err != nil {
		if errors.Is(err, ErrNoRows()) {
			return nil, nil
		}

		return nil, fmt.Errorf("センサー状態の取得に失敗しました: %w", err)
	}

	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &sensorData.RawPayload); err != nil {
			return nil, fmt.Errorf("センサー raw_payload の変換に失敗しました: %w", err)
		}
	}

	return &sensorData, nil
}

func (s *PostgresStore) UpsertSensorData(ctx context.Context, input models.UpsertSensorDataInput) (*models.SensorData, error) {
	payload, err := json.Marshal(input.RawPayload)
	if err != nil {
		return nil, fmt.Errorf("センサー raw_payload の変換に失敗しました: %w", err)
	}

	const query = `
		insert into public.sensor_data (
			user_id,
			device_id,
			moisture_percent,
			light_lux,
			temperature_celsius,
			recorded_at,
			sensor_status,
			raw_payload
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (user_id)
		do update set
			device_id = excluded.device_id,
			moisture_percent = excluded.moisture_percent,
			light_lux = excluded.light_lux,
			temperature_celsius = excluded.temperature_celsius,
			recorded_at = excluded.recorded_at,
			sensor_status = excluded.sensor_status,
			raw_payload = excluded.raw_payload
		returning
			user_id,
			device_id,
			moisture_percent,
			light_lux,
			temperature_celsius,
			recorded_at,
			sensor_status,
			raw_payload,
			created_at,
			updated_at
	`

	var sensorData models.SensorData
	var raw []byte
	if err := s.pool.QueryRow(
		ctx,
		query,
		input.UserID,
		input.DeviceID,
		input.MoisturePercent,
		input.LightLux,
		input.TemperatureCelsius,
		input.RecordedAt,
		input.SensorStatus,
		payload,
	).Scan(
		&sensorData.UserID,
		&sensorData.DeviceID,
		&sensorData.MoisturePercent,
		&sensorData.LightLux,
		&sensorData.TemperatureCelsius,
		&sensorData.RecordedAt,
		&sensorData.SensorStatus,
		&raw,
		&sensorData.CreatedAt,
		&sensorData.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("センサー状態の保存に失敗しました: %w", err)
	}

	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &sensorData.RawPayload); err != nil {
			return nil, fmt.Errorf("センサー raw_payload の読み取りに失敗しました: %w", err)
		}
	}

	return &sensorData, nil
}
