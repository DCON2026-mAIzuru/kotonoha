package service

import (
	"context"
	"testing"
	"time"

	"github.com/apple/kotonoha/backend/internal/llm"
	"github.com/apple/kotonoha/backend/internal/models"
	"github.com/apple/kotonoha/backend/internal/realtime"
	pushtransport "github.com/apple/kotonoha/backend/internal/webpush"
)

type sensorServiceTestStore struct {
	previous             *models.SensorData
	profile              *models.BonsaiProfile
	subscriptions        []models.PushSubscription
	latestEventsByName   map[string]*models.ChatHistoryEntry
	lastQueriedEventName string
	createdHistory       []models.CreateChatHistoryEntryInput
}

func (s *sensorServiceTestStore) GetSensorDataByUserID(context.Context, string) (*models.SensorData, error) {
	return s.previous, nil
}

func (s *sensorServiceTestStore) UpsertSensorData(_ context.Context, input models.UpsertSensorDataInput) (*models.SensorData, error) {
	s.previous = &models.SensorData{
		UserID:             input.UserID,
		DeviceID:           input.DeviceID,
		MoisturePercent:    input.MoisturePercent,
		LightLux:           input.LightLux,
		TemperatureCelsius: input.TemperatureCelsius,
		RecordedAt:         input.RecordedAt,
		SensorStatus:       input.SensorStatus,
		RawPayload:         input.RawPayload,
	}
	return s.previous, nil
}

func (s *sensorServiceTestStore) GetBonsaiProfileByUserID(context.Context, string) (*models.BonsaiProfile, error) {
	return s.profile, nil
}

func (s *sensorServiceTestStore) ListPushSubscriptionsByUserID(context.Context, string) ([]models.PushSubscription, error) {
	return s.subscriptions, nil
}

func (s *sensorServiceTestStore) GetLatestChatHistoryEventByName(_ context.Context, _ string, eventName string) (*models.ChatHistoryEntry, error) {
	s.lastQueriedEventName = eventName
	if s.latestEventsByName == nil {
		return nil, nil
	}
	return s.latestEventsByName[eventName], nil
}

func (s *sensorServiceTestStore) CreateChatHistoryEntry(_ context.Context, input models.CreateChatHistoryEntryInput) (*models.ChatHistoryEntry, error) {
	s.createdHistory = append(s.createdHistory, input)
	return &models.ChatHistoryEntry{
		ID:             "event-1",
		UserID:         input.UserID,
		Role:           input.Role,
		Content:        input.Content,
		SensorSnapshot: input.SensorSnapshot,
		CreatedAt:      time.Now().UTC(),
	}, nil
}

func (s *sensorServiceTestStore) UpdateBonsaiBondScore(_ context.Context, _ string, score int) (*models.BonsaiProfile, error) {
	if s.profile != nil {
		s.profile.BondScore = score
	}
	return s.profile, nil
}

type sensorServiceTestStreamer struct {
	reply       string
	lastRequest llm.ChatRequest
}

func (s *sensorServiceTestStreamer) StreamChat(_ context.Context, req llm.ChatRequest, onChunk func(text string) error) error {
	s.lastRequest = req
	return onChunk(s.reply)
}

type sensorServiceTestNotifier struct {
	messages []realtime.Message
}

func (n *sensorServiceTestNotifier) Broadcast(_ string, message realtime.Message) {
	n.messages = append(n.messages, message)
}

type sensorServiceTestPushSender struct {
	payloads []pushtransport.Payload
}

func (s *sensorServiceTestPushSender) Send(_ context.Context, _ models.PushSubscription, payload pushtransport.Payload) error {
	s.payloads = append(s.payloads, payload)
	return nil
}

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

func TestIngestUsesSoilMoistureAsCanonicalMoisture(t *testing.T) {
	t.Parallel()

	store := &sensorServiceTestStore{}
	service := NewSensorService(store, nil, nil, nil)
	soilMoisture := 28.4
	airHumidity := 61.2
	pressure := 1008.6
	ec := 520.0
	ph := 6.7

	result, err := service.Ingest(context.Background(), IngestSensorInput{
		UserID:              "user-1",
		DeviceID:            "esp32-demo-001",
		RecordedAt:          time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		MoisturePercent:     airHumidity,
		LightLux:            1200,
		TemperatureCelsius:  24.1,
		PressurePascal:      &pressure,
		SoilMoisturePercent: &soilMoisture,
		SoilECUsCm:          &ec,
		SoilPHValue:         &ph,
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}
	if result.SensorData.MoisturePercent != soilMoisture {
		t.Fatalf("expected canonical moisture to use soil value %v, got %v", soilMoisture, result.SensorData.MoisturePercent)
	}
	if got := result.SensorData.RawPayload["air_humidity_percent"]; got != airHumidity {
		t.Fatalf("expected air humidity to be preserved, got %#v", got)
	}
	if got := result.SensorData.RawPayload["soil_moisture_percent"]; got != soilMoisture {
		t.Fatalf("expected soil moisture raw payload, got %#v", got)
	}
	if got := result.SensorData.RawPayload["soil_ec_us_cm"]; got != ec {
		t.Fatalf("expected soil EC raw payload, got %#v", got)
	}
	if got := result.SensorData.RawPayload["soil_ph_value"]; got != ph {
		t.Fatalf("expected soil pH raw payload, got %#v", got)
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

func TestShouldTriggerLowMoistureEventOnThresholdCrossing(t *testing.T) {
	t.Parallel()

	previous := &models.SensorData{
		MoisturePercent: 24,
		RecordedAt:      time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		SensorStatus:    sensorStatusNormal,
	}
	current := &models.SensorData{
		MoisturePercent: 19.5,
		RecordedAt:      time.Date(2026, 4, 14, 13, 0, 0, 0, time.UTC),
		SensorStatus:    sensorStatusNormal,
	}

	if !shouldTriggerLowMoistureEvent(previous, current) {
		t.Fatal("low moisture event should trigger when moisture crosses below 20%")
	}
}

func TestShouldTriggerLowMoistureEventSkipsRepeatedDryReadings(t *testing.T) {
	t.Parallel()

	previous := &models.SensorData{
		MoisturePercent: 18,
		RecordedAt:      time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		SensorStatus:    sensorStatusNormal,
	}
	current := &models.SensorData{
		MoisturePercent: 15,
		RecordedAt:      time.Date(2026, 4, 14, 13, 0, 0, 0, time.UTC),
		SensorStatus:    sensorStatusNormal,
	}

	if shouldTriggerLowMoistureEvent(previous, current) {
		t.Fatal("low moisture event should not retrigger while already below 20%")
	}
}

func TestIngestTriggersLowMoisturePushNotification(t *testing.T) {
	now := time.Now().UTC()
	store := &sensorServiceTestStore{
		previous: &models.SensorData{
			UserID:             "user-1",
			DeviceID:           "sensor-1",
			MoisturePercent:    26,
			LightLux:           900,
			TemperatureCelsius: 22,
			RecordedAt:         now.Add(-30 * time.Minute),
			SensorStatus:       sensorStatusNormal,
		},
		profile: &models.BonsaiProfile{
			UserID:      "user-1",
			Name:        "言の葉",
			Personality: "素直で少し甘えん坊",
		},
		subscriptions: []models.PushSubscription{{ID: "sub-1"}},
		latestEventsByName: map[string]*models.ChatHistoryEntry{
			sensorEventNameWatering: {
				ID:        "event-watering",
				UserID:    "user-1",
				Role:      sensorEventRole,
				CreatedAt: now.Add(-30 * time.Minute),
			},
		},
	}
	streamer := &sensorServiceTestStreamer{reply: "のどが乾いてきて、ひとくち水がほしいぼんすよ"}
	notifier := &sensorServiceTestNotifier{}
	pushSender := &sensorServiceTestPushSender{}

	service := NewSensorService(store, streamer, notifier, pushSender)

	result, err := service.Ingest(context.Background(), IngestSensorInput{
		UserID:             "user-1",
		DeviceID:           "sensor-1",
		RecordedAt:         now.Add(-1 * time.Minute),
		MoisturePercent:    19,
		LightLux:           850,
		TemperatureCelsius: 21,
	})
	if err != nil {
		t.Fatalf("Ingest returned error: %v", err)
	}

	if !result.EventTriggered {
		t.Fatal("expected low moisture event to trigger")
	}
	if result.EventName != sensorEventNameLowMoisture {
		t.Fatalf("expected event %q, got %q", sensorEventNameLowMoisture, result.EventName)
	}
	if result.PushSentCount != 1 {
		t.Fatalf("expected 1 push notification, got %d", result.PushSentCount)
	}
	if store.lastQueriedEventName != sensorEventNameLowMoisture {
		t.Fatalf("expected cooldown lookup for %q, got %q", sensorEventNameLowMoisture, store.lastQueriedEventName)
	}
	if len(pushSender.payloads) != 1 || pushSender.payloads[0].Event != sensorEventNameLowMoisture {
		t.Fatalf("expected low moisture push payload, got %#v", pushSender.payloads)
	}
	if len(store.createdHistory) != 1 {
		t.Fatalf("expected 1 created history entry, got %d", len(store.createdHistory))
	}
	if got := store.createdHistory[0].SensorSnapshot["event"]; got != sensorEventNameLowMoisture {
		t.Fatalf("expected history event %q, got %#v", sensorEventNameLowMoisture, got)
	}
	if len(notifier.messages) != 2 {
		t.Fatalf("expected 2 realtime messages, got %d", len(notifier.messages))
	}
	if notifier.messages[0].Event != sensorEventNameLowMoisture || notifier.messages[1].Event != sensorEventNameLowMoisture {
		t.Fatalf("expected realtime messages for %q, got %#v", sensorEventNameLowMoisture, notifier.messages)
	}
	if streamer.lastRequest.Message == "" {
		t.Fatal("expected LLM prompt to be built for low moisture event")
	}
}
