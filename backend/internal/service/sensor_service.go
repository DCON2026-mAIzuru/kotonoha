package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/apple/kotonoha/backend/internal/llm"
	"github.com/apple/kotonoha/backend/internal/models"
	"github.com/apple/kotonoha/backend/internal/realtime"
	pushtransport "github.com/apple/kotonoha/backend/internal/webpush"
)

const (
	sensorStatusNormal         = "normal"
	sensorStatusStale          = "stale"
	sensorStatusInvalid        = "invalid"
	sensorEventRole            = "event"
	sensorEventNameWatering    = "watering_detected"
	sensorEventNameLowMoisture = "low_moisture_detected"
	sensorEventSource          = "sensor_event"
	sensorEventWindow          = 3 * time.Minute
	sensorEventCooldown        = 2 * time.Hour
	sensorStaleThreshold       = 15 * time.Minute
	moistureRiseThreshold      = 20.0
	lowMoistureThreshold       = 20.0
)

var ErrSensorReadingOutOfOrder = errors.New("recorded_at が最新のセンサー時刻より古いため保存できません")

type sensorStore interface {
	GetSensorDataByUserID(ctx context.Context, userID string) (*models.SensorData, error)
	UpsertSensorData(ctx context.Context, input models.UpsertSensorDataInput) (*models.SensorData, error)
	GetBonsaiProfileByUserID(ctx context.Context, userID string) (*models.BonsaiProfile, error)
	ListPushSubscriptionsByUserID(ctx context.Context, userID string) ([]models.PushSubscription, error)
	GetLatestChatHistoryEventByName(ctx context.Context, userID string, eventName string) (*models.ChatHistoryEntry, error)
	CreateChatHistoryEntry(ctx context.Context, input models.CreateChatHistoryEntryInput) (*models.ChatHistoryEntry, error)
	UpdateBonsaiBondScore(ctx context.Context, userID string, score int) (*models.BonsaiProfile, error)
}

type sensorNotifier interface {
	Broadcast(userID string, message realtime.Message)
}

type sensorPushSender interface {
	Send(ctx context.Context, subscription models.PushSubscription, payload pushtransport.Payload) error
}

type SensorService struct {
	store      sensorStore
	llm        chatLLMStreamer
	notifier   sensorNotifier
	pushSender sensorPushSender
	mu         sync.RWMutex
	controls   map[string]moistureControlState
}

type IngestSensorInput struct {
	UserID              string
	DeviceID            string    `json:"device_id"`
	RecordedAt          time.Time `json:"recorded_at"`
	MoisturePercent     float64   `json:"moisture_percent"`
	AirHumidityPercent  *float64  `json:"air_humidity_percent"`
	LightLux            float64   `json:"light_lux"`
	TemperatureCelsius  float64   `json:"temperature_celsius"`
	PressurePascal      *float64  `json:"pressure_pascal"`
	SoilMoisturePercent *float64  `json:"soil_moisture_percent"`
	SoilECUsCm          *float64  `json:"soil_ec_us_cm"`
	SoilPHValue         *float64  `json:"soil_ph_value"`
}

type IngestSensorResult struct {
	SensorData          *models.SensorData `json:"sensor_data"`
	EventTriggered      bool               `json:"event_triggered"`
	EventName           string             `json:"event_name,omitempty"`
	EventMessage        string             `json:"event_message,omitempty"`
	PushSentCount       int                `json:"push_sent_count,omitempty"`
	NotificationWarning string             `json:"notification_warning,omitempty"`
}

type MoistureControl struct {
	Paused      bool       `json:"paused"`
	ManualValue *float64   `json:"manual_value"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

type UpdateMoistureControlInput struct {
	Paused      bool
	ManualValue *float64
}

type moistureControlState struct {
	Paused      bool
	ManualValue *float64
	UpdatedAt   time.Time
}

func NewSensorService(store sensorStore, llmStreamer chatLLMStreamer, notifier sensorNotifier, pushSender sensorPushSender) *SensorService {
	return &SensorService{
		store:      store,
		llm:        llmStreamer,
		notifier:   notifier,
		pushSender: pushSender,
		controls:   make(map[string]moistureControlState),
	}
}

func (s *SensorService) GetCurrent(ctx context.Context, userID string) (*models.SensorData, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("センサーサービスが設定されていません")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("センサーユーザーIDが空です")
	}

	sensorData, err := s.store.GetSensorDataByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if sensorData == nil {
		return nil, nil
	}

	if sensorData.SensorStatus != sensorStatusInvalid {
		sensorData.SensorStatus = sensorStatusFor(sensorData.RecordedAt.UTC(), time.Now().UTC(), false)
	}

	return s.applyMoistureControl(userID, sensorData), nil
}

func (s *SensorService) GetMoistureControl(_ context.Context, userID string) (*MoistureControl, error) {
	if s == nil {
		return nil, errors.New("センサーサービスが設定されていません")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("センサーユーザーIDが空です")
	}

	s.mu.RLock()
	state, ok := s.controls[userID]
	s.mu.RUnlock()
	if !ok {
		return &MoistureControl{Paused: false}, nil
	}

	return moistureControlFromState(state), nil
}

func (s *SensorService) UpdateMoistureControl(ctx context.Context, userID string, input UpdateMoistureControlInput) (*MoistureControl, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("センサーサービスが設定されていません")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("センサーユーザーIDが空です")
	}
	if input.ManualValue != nil {
		switch {
		case math.IsNaN(*input.ManualValue) || math.IsInf(*input.ManualValue, 0):
			return nil, errors.New("manual_value が不正です")
		case *input.ManualValue < 0 || *input.ManualValue > 100:
			return nil, errors.New("manual_value は 0 から 100 の範囲で指定してください")
		}
	}

	if !input.Paused {
		s.mu.Lock()
		delete(s.controls, userID)
		s.mu.Unlock()
		return &MoistureControl{Paused: false}, nil
	}

	current, err := s.store.GetSensorDataByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	manualValue := cloneFloat64Pointer(input.ManualValue)
	if manualValue == nil && current != nil {
		manualValue = float64Pointer(current.MoisturePercent)
	}

	state := moistureControlState{
		Paused:      true,
		ManualValue: manualValue,
		UpdatedAt:   time.Now().UTC(),
	}

	s.mu.Lock()
	s.controls[userID] = state
	s.mu.Unlock()

	if current != nil && manualValue != nil {
		status := current.SensorStatus
		if status != sensorStatusInvalid {
			status = sensorStatusFor(current.RecordedAt.UTC(), time.Now().UTC(), false)
		}

		rawPayload := cloneRawPayload(current.RawPayload)
		rawPayload["moisture_control_paused"] = true
		rawPayload["manual_moisture_percent"] = *manualValue

		if _, err := s.store.UpsertSensorData(ctx, models.UpsertSensorDataInput{
			UserID:             userID,
			DeviceID:           current.DeviceID,
			MoisturePercent:    *manualValue,
			LightLux:           current.LightLux,
			TemperatureCelsius: current.TemperatureCelsius,
			RecordedAt:         current.RecordedAt.UTC(),
			SensorStatus:       status,
			RawPayload:         rawPayload,
		}); err != nil {
			return nil, err
		}
	}

	return moistureControlFromState(state), nil
}

func (s *SensorService) Ingest(ctx context.Context, input IngestSensorInput) (*IngestSensorResult, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("センサーサービスが設定されていません")
	}

	input.UserID = strings.TrimSpace(input.UserID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)

	switch {
	case input.UserID == "":
		return nil, errors.New("センサーユーザーIDが空です")
	case input.DeviceID == "":
		return nil, errors.New("device_id がありません")
	case input.RecordedAt.IsZero():
		return nil, errors.New("recorded_at がありません")
	case input.RecordedAt.After(time.Now().UTC().Add(2 * time.Minute)):
		return nil, errors.New("recorded_at が未来時刻すぎます")
	case math.IsNaN(input.MoisturePercent) || math.IsInf(input.MoisturePercent, 0):
		return nil, errors.New("moisture_percent が不正です")
	case invalidOptionalFloat(input.AirHumidityPercent):
		return nil, errors.New("air_humidity_percent が不正です")
	case math.IsNaN(input.LightLux) || math.IsInf(input.LightLux, 0):
		return nil, errors.New("light_lux が不正です")
	case math.IsNaN(input.TemperatureCelsius) || math.IsInf(input.TemperatureCelsius, 0):
		return nil, errors.New("temperature_celsius が不正です")
	case invalidOptionalFloat(input.PressurePascal):
		return nil, errors.New("pressure_pascal が不正です")
	case invalidOptionalFloat(input.SoilMoisturePercent):
		return nil, errors.New("soil_moisture_percent が不正です")
	case invalidOptionalFloat(input.SoilECUsCm):
		return nil, errors.New("soil_ec_us_cm が不正です")
	case invalidOptionalFloat(input.SoilPHValue):
		return nil, errors.New("soil_ph_value が不正です")
	}

	previous, err := s.store.GetSensorDataByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	recordedAt := input.RecordedAt.UTC()
	if previous != nil && recordedAt.Before(previous.RecordedAt) {
		return nil, ErrSensorReadingOutOfOrder
	}

	rawMoisturePercent := input.MoisturePercent
	effectiveMoisturePercent := canonicalMoisturePercent(input)
	input.MoisturePercent = effectiveMoisturePercent
	control := s.moistureControlForUser(input.UserID)
	if control.Paused && control.ManualValue != nil {
		input.MoisturePercent = *control.ManualValue
	}

	status, sanitizedMoisture, sanitizedLight, sanitizedTemperature, issues := normalizeSensorReading(input, time.Now().UTC())

	rawPayload := map[string]any{
		"device_id":           input.DeviceID,
		"recorded_at":         recordedAt.Format(time.RFC3339),
		"moisture_percent":    effectiveMoisturePercent,
		"light_lux":           input.LightLux,
		"temperature_celsius": input.TemperatureCelsius,
		"sensor_issues":       issues,
	}
	addOptionalRawPayloadFloat(rawPayload, "air_humidity_percent", airHumidityPercent(input, rawMoisturePercent))
	addOptionalRawPayloadFloat(rawPayload, "pressure_pascal", input.PressurePascal)
	addOptionalRawPayloadFloat(rawPayload, "soil_moisture_percent", input.SoilMoisturePercent)
	addOptionalRawPayloadFloat(rawPayload, "soil_ec_us_cm", input.SoilECUsCm)
	addOptionalRawPayloadFloat(rawPayload, "soil_ph_value", input.SoilPHValue)
	if control.Paused {
		rawPayload["moisture_control_paused"] = true
		if control.ManualValue != nil {
			rawPayload["manual_moisture_percent"] = *control.ManualValue
		}
	}

	sensorData, err := s.store.UpsertSensorData(ctx, models.UpsertSensorDataInput{
		UserID:             input.UserID,
		DeviceID:           input.DeviceID,
		MoisturePercent:    sanitizedMoisture,
		LightLux:           sanitizedLight,
		TemperatureCelsius: sanitizedTemperature,
		RecordedAt:         recordedAt,
		SensorStatus:       status,
		RawPayload:         rawPayload,
	})
	if err != nil {
		return nil, err
	}

	result := &IngestSensorResult{
		SensorData: sensorData,
	}

	eventName := detectSensorEvent(previous, sensorData)
	if eventName == "" {
		return result, nil
	}

	latestEvent, err := s.store.GetLatestChatHistoryEventByName(ctx, input.UserID, eventName)
	if err != nil {
		return nil, err
	}
	if latestEvent != nil && time.Since(latestEvent.CreatedAt) < sensorEventCooldown {
		return result, nil
	}

	profile, err := s.store.GetBonsaiProfileByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if profile != nil {
		nextScore := applyBondDelta(profile.BondScore, bondDeltaForEvent(eventName))
		if nextScore != profile.BondScore {
			updatedProfile, updateErr := s.store.UpdateBonsaiBondScore(ctx, input.UserID, nextScore)
			if updateErr != nil {
				log.Printf("センサーイベントによるなじみ具合の更新に失敗しました: user_id=%s event=%s err=%v", input.UserID, eventName, updateErr)
			} else {
				profile = updatedProfile
			}
		}
	}

	eventMessage, err := s.generateSensorEventReaction(ctx, eventName, profile, previous, sensorData)
	if err != nil {
		result.NotificationWarning = err.Error()
		return result, nil
	}

	eventMessage = strings.TrimSpace(eventMessage)
	if eventMessage == "" {
		result.NotificationWarning = "センサーイベントの通知文が空でした"
		return result, nil
	}

	result.EventTriggered = true
	result.EventName = eventName
	result.EventMessage = eventMessage

	s.broadcastEvent(input.UserID, realtime.Message{
		Type:   "reply_done",
		Source: sensorEventSource,
		Event:  eventName,
	})

	pushSentCount := 0
	subscriptions, err := s.store.ListPushSubscriptionsByUserID(ctx, input.UserID)
	if err != nil {
		log.Printf("Push購読一覧の取得に失敗しました: user_id=%s err=%v", input.UserID, err)
		result.NotificationWarning = "Push購読一覧の取得に失敗しました"
	} else {
		title := "盆栽からのお知らせ"
		if profile != nil && strings.TrimSpace(profile.Name) != "" {
			title = fmt.Sprintf("%s からのお知らせ", strings.TrimSpace(profile.Name))
		}

		for _, subscription := range subscriptions {
			if s.pushSender == nil {
				continue
			}

			err := s.pushSender.Send(ctx, subscription, pushtransport.Payload{
				Title:  title,
				Body:   eventMessage,
				Source: sensorEventSource,
				Event:  eventName,
				Tag:    eventName,
			})
			if err != nil {
				log.Printf("Push通知の送信に失敗しました: subscription_id=%s err=%v", subscription.ID, err)
				if result.NotificationWarning == "" {
					result.NotificationWarning = "一部のPush通知送信に失敗しました"
				}
				continue
			}
			pushSentCount++
		}
	}
	result.PushSentCount = pushSentCount

	_, err = s.store.CreateChatHistoryEntry(ctx, models.CreateChatHistoryEntryInput{
		UserID:  input.UserID,
		Role:    sensorEventRole,
		Content: eventMessage,
		SensorSnapshot: map[string]any{
			"event":                     eventName,
			"device_id":                 sensorData.DeviceID,
			"recorded_at":               sensorData.RecordedAt.Format(time.RFC3339),
			"moisture_percent":          sensorData.MoisturePercent,
			"previous_moisture_percent": previous.MoisturePercent,
			"moisture_delta":            sensorData.MoisturePercent - previous.MoisturePercent,
			"light_lux":                 sensorData.LightLux,
			"temperature_celsius":       sensorData.TemperatureCelsius,
		},
	})
	if err != nil {
		log.Printf("センサーイベント履歴の保存に失敗しました: user_id=%s err=%v", input.UserID, err)
		if result.NotificationWarning == "" {
			result.NotificationWarning = "イベント履歴の保存に失敗しました"
		}
	}

	return result, nil
}

func (s *SensorService) applyMoistureControl(userID string, sensorData *models.SensorData) *models.SensorData {
	if sensorData == nil {
		return nil
	}

	control := s.moistureControlForUser(userID)
	if !control.Paused || control.ManualValue == nil {
		return sensorData
	}

	cloned := *sensorData
	cloned.MoisturePercent = *control.ManualValue
	cloned.RawPayload = cloneRawPayload(sensorData.RawPayload)
	cloned.RawPayload["moisture_control_paused"] = true
	cloned.RawPayload["manual_moisture_percent"] = *control.ManualValue
	return &cloned
}

func (s *SensorService) moistureControlForUser(userID string) moistureControlState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.controls[strings.TrimSpace(userID)]
	if !ok {
		return moistureControlState{}
	}

	return moistureControlState{
		Paused:      state.Paused,
		ManualValue: cloneFloat64Pointer(state.ManualValue),
		UpdatedAt:   state.UpdatedAt,
	}
}

func moistureControlFromState(state moistureControlState) *MoistureControl {
	if !state.Paused {
		return &MoistureControl{Paused: false}
	}

	updatedAt := state.UpdatedAt
	return &MoistureControl{
		Paused:      true,
		ManualValue: cloneFloat64Pointer(state.ManualValue),
		UpdatedAt:   &updatedAt,
	}
}

func float64Pointer(value float64) *float64 {
	return &value
}

func cloneFloat64Pointer(value *float64) *float64 {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func cloneRawPayload(rawPayload map[string]any) map[string]any {
	if len(rawPayload) == 0 {
		return make(map[string]any)
	}

	cloned := make(map[string]any, len(rawPayload))
	for key, value := range rawPayload {
		cloned[key] = value
	}
	return cloned
}

func (s *SensorService) generateSensorEventReaction(
	ctx context.Context,
	eventName string,
	profile *models.BonsaiProfile,
	previous *models.SensorData,
	current *models.SensorData,
) (string, error) {
	if s.llm == nil {
		return "", errors.New("イベント通知用のLLMが設定されていません")
	}

	systemPrompt, userPrompt, err := buildSensorEventPrompts(eventName, profile, previous, current)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	err = s.llm.StreamChat(ctx, llm.ChatRequest{
		UserID:       current.UserID,
		SystemPrompt: systemPrompt,
		Message:      userPrompt,
	}, func(text string) error {
		builder.WriteString(text)
		s.broadcastEvent(current.UserID, realtime.Message{
			Type:   "reply_chunk",
			Text:   text,
			Source: sensorEventSource,
			Event:  eventName,
		})
		return nil
	})
	if err != nil {
		s.broadcastEvent(current.UserID, realtime.Message{
			Type:    "error",
			Message: "センサーイベントの生成に失敗しました",
			Source:  sensorEventSource,
			Event:   eventName,
		})
		return "", fmt.Errorf("センサーイベントの生成に失敗しました: %w", err)
	}

	return builder.String(), nil
}

func (s *SensorService) broadcastEvent(userID string, message realtime.Message) {
	if s.notifier == nil {
		return
	}

	s.notifier.Broadcast(userID, message)
}

func detectSensorEvent(previous *models.SensorData, current *models.SensorData) string {
	switch {
	case shouldTriggerWateringEvent(previous, current):
		return sensorEventNameWatering
	case shouldTriggerLowMoistureEvent(previous, current):
		return sensorEventNameLowMoisture
	default:
		return ""
	}
}

func shouldTriggerWateringEvent(previous *models.SensorData, current *models.SensorData) bool {
	if previous == nil || current == nil {
		return false
	}

	if previous.SensorStatus != sensorStatusNormal || current.SensorStatus != sensorStatusNormal {
		return false
	}

	if current.RecordedAt.Before(previous.RecordedAt) {
		return false
	}

	return current.RecordedAt.Sub(previous.RecordedAt) <= sensorEventWindow &&
		(current.MoisturePercent-previous.MoisturePercent) >= moistureRiseThreshold
}

func shouldTriggerLowMoistureEvent(previous *models.SensorData, current *models.SensorData) bool {
	if previous == nil || current == nil {
		return false
	}

	if previous.SensorStatus != sensorStatusNormal || current.SensorStatus != sensorStatusNormal {
		return false
	}

	if current.RecordedAt.Before(previous.RecordedAt) {
		return false
	}

	return previous.MoisturePercent >= lowMoistureThreshold &&
		current.MoisturePercent < lowMoistureThreshold
}

func sensorStatusFor(recordedAt time.Time, now time.Time, hasInvalidReading bool) string {
	if hasInvalidReading {
		return sensorStatusInvalid
	}

	if now.Sub(recordedAt) > sensorStaleThreshold {
		return sensorStatusStale
	}

	return sensorStatusNormal
}

func normalizeSensorReading(input IngestSensorInput, now time.Time) (string, float64, float64, float64, []string) {
	issues := make([]string, 0, 3)

	moisture := input.MoisturePercent
	if moisture < 0 || moisture > 100 {
		issues = append(issues, fmt.Sprintf("moisture_percent=%.2f", moisture))
		moisture = clampFloat(moisture, 0, 100)
	}

	light := input.LightLux
	if light < 0 {
		issues = append(issues, fmt.Sprintf("light_lux=%.2f", light))
		light = 0
	}

	temperature := input.TemperatureCelsius
	if temperature < -50 || temperature > 100 {
		issues = append(issues, fmt.Sprintf("temperature_celsius=%.2f", temperature))
		temperature = clampFloat(temperature, -50, 100)
	}

	return sensorStatusFor(input.RecordedAt.UTC(), now, len(issues) > 0), moisture, light, temperature, issues
}

func canonicalMoisturePercent(input IngestSensorInput) float64 {
	if input.SoilMoisturePercent != nil {
		return *input.SoilMoisturePercent
	}

	return input.MoisturePercent
}

func airHumidityPercent(input IngestSensorInput, rawMoisturePercent float64) *float64 {
	if input.AirHumidityPercent != nil {
		return input.AirHumidityPercent
	}
	if input.SoilMoisturePercent != nil {
		return &rawMoisturePercent
	}

	return nil
}

func invalidOptionalFloat(value *float64) bool {
	return value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0))
}

func addOptionalRawPayloadFloat(payload map[string]any, key string, value *float64) {
	if value != nil {
		payload[key] = *value
	}
}

func clampFloat(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}

	return value
}

func buildWateringSystemPrompt(profile *models.BonsaiProfile) string {
	name := "盆栽"
	personality := "穏やかでやわらかい口調"

	if profile != nil {
		if trimmedName := strings.TrimSpace(profile.Name); trimmedName != "" {
			name = trimmedName
		}
		if trimmedPersonality := strings.TrimSpace(profile.Personality); trimmedPersonality != "" {
			personality = trimmedPersonality
		}
	}

	return fmt.Sprintf(
		"あなたは大切に育てられている盆栽です。名前は「%s」です。性格は「%s」です。日本語で1〜2文、20〜60文字程度に収め、絵文字・箇条書き・引用符は使わないでください。最初の一文は水やりされた直後の身体感覚（ほっとした感じ、根元がほどける感じなど）を自然に表現してください。感謝を大げさにせず、世話が届いたことを小さく返してください。%s",
		name,
		personality,
		sensorEventBonsuInstruction(profile),
	)
}

func buildWateringUserPrompt(previous *models.SensorData, current *models.SensorData) string {
	return fmt.Sprintf(
		"センサーイベントです。土壌水分量が %.1f%% から %.1f%% に %.1f ポイント上がりました。照度は %.1f lux、気温は %.1f ℃ です。ユーザーがお水をくれた可能性が高いので、盆栽としてほっとした気持ちや感謝をこめた自発メッセージを一言だけ返してください。",
		previous.MoisturePercent,
		current.MoisturePercent,
		current.MoisturePercent-previous.MoisturePercent,
		current.LightLux,
		current.TemperatureCelsius,
	)
}

func buildLowMoistureSystemPrompt(profile *models.BonsaiProfile) string {
	name := "盆栽"
	personality := "穏やかでやわらかい口調"

	if profile != nil {
		if trimmedName := strings.TrimSpace(profile.Name); trimmedName != "" {
			name = trimmedName
		}
		if trimmedPersonality := strings.TrimSpace(profile.Personality); trimmedPersonality != "" {
			personality = trimmedPersonality
		}
	}

	return fmt.Sprintf(
		"あなたは大切に育てられている盆栽です。名前は「%s」です。性格は「%s」です。少し切実で素直な口調でユーザーへ自分の乾き具合を伝えてください。日本語で1〜2文、20〜60文字程度に収め、絵文字・箇条書き・引用符は使わないでください。最初の一文は乾きやしんどさを自然に表現してください。責めず、次の小さな世話につなげてください。乾きが強い時は「ぼんす」語尾を使わないでください。",
		name,
		personality,
	)
}

func sensorEventBonsuInstruction(profile *models.BonsaiProfile) string {
	if profile != nil && profile.BondScore >= 90 && normalizeToneMode(profile.ToneMode) != ToneModeClassical {
		return "深くなじんだ相手への親しい癖として、「ぼんすよ」「ぼんすね」を一文だけ使ってよい。全文をぼんす語尾にしないでください。"
	}

	return "「ぼんす」語尾はまだ使わないでください。"
}

func buildLowMoistureUserPrompt(previous *models.SensorData, current *models.SensorData) string {
	return fmt.Sprintf(
		"センサーイベントです。土壌水分量が %.1f%% から %.1f%% に下がり、20%% を下回りました。照度は %.1f lux、気温は %.1f ℃ です。盆栽として水がほしいことや乾いてきた感覚を、自発メッセージとして一言だけ返してください。",
		previous.MoisturePercent,
		current.MoisturePercent,
		current.LightLux,
		current.TemperatureCelsius,
	)
}

func buildSensorEventPrompts(
	eventName string,
	profile *models.BonsaiProfile,
	previous *models.SensorData,
	current *models.SensorData,
) (string, string, error) {
	switch eventName {
	case sensorEventNameWatering:
		return buildWateringSystemPrompt(profile), buildWateringUserPrompt(previous, current), nil
	case sensorEventNameLowMoisture:
		return buildLowMoistureSystemPrompt(profile), buildLowMoistureUserPrompt(previous, current), nil
	default:
		return "", "", fmt.Errorf("未対応のセンサーイベントです: %s", eventName)
	}
}
