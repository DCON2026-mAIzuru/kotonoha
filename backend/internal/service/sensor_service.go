package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/apple/kotonoha/backend/internal/llm"
	"github.com/apple/kotonoha/backend/internal/models"
	"github.com/apple/kotonoha/backend/internal/realtime"
	pushtransport "github.com/apple/kotonoha/backend/internal/webpush"
)

const (
	sensorStatusNormal      = "normal"
	sensorStatusStale       = "stale"
	sensorStatusInvalid     = "invalid"
	sensorEventRole         = "event"
	sensorEventNameWatering = "watering_detected"
	sensorEventSource       = "sensor_event"
	sensorEventWindow       = 3 * time.Minute
	sensorEventCooldown     = 2 * time.Hour
	sensorStaleThreshold    = 15 * time.Minute
	moistureRiseThreshold   = 20.0
)

var ErrSensorReadingOutOfOrder = errors.New("recorded_at が最新のセンサー時刻より古いため保存できません")

type sensorStore interface {
	GetSensorDataByUserID(ctx context.Context, userID string) (*models.SensorData, error)
	UpsertSensorData(ctx context.Context, input models.UpsertSensorDataInput) (*models.SensorData, error)
	GetBonsaiProfileByUserID(ctx context.Context, userID string) (*models.BonsaiProfile, error)
	ListPushSubscriptionsByUserID(ctx context.Context, userID string) ([]models.PushSubscription, error)
	GetLatestChatHistoryByRole(ctx context.Context, userID string, role string) (*models.ChatHistoryEntry, error)
	CreateChatHistoryEntry(ctx context.Context, input models.CreateChatHistoryEntryInput) (*models.ChatHistoryEntry, error)
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
}

type IngestSensorInput struct {
	UserID             string
	DeviceID           string    `json:"device_id"`
	RecordedAt         time.Time `json:"recorded_at"`
	MoisturePercent    float64   `json:"moisture_percent"`
	LightLux           float64   `json:"light_lux"`
	TemperatureCelsius float64   `json:"temperature_celsius"`
}

type IngestSensorResult struct {
	SensorData          *models.SensorData `json:"sensor_data"`
	EventTriggered      bool               `json:"event_triggered"`
	EventName           string             `json:"event_name,omitempty"`
	EventMessage        string             `json:"event_message,omitempty"`
	PushSentCount       int                `json:"push_sent_count,omitempty"`
	NotificationWarning string             `json:"notification_warning,omitempty"`
}

func NewSensorService(store sensorStore, llmStreamer chatLLMStreamer, notifier sensorNotifier, pushSender sensorPushSender) *SensorService {
	return &SensorService{
		store:      store,
		llm:        llmStreamer,
		notifier:   notifier,
		pushSender: pushSender,
	}
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
	case math.IsNaN(input.LightLux) || math.IsInf(input.LightLux, 0):
		return nil, errors.New("light_lux が不正です")
	case math.IsNaN(input.TemperatureCelsius) || math.IsInf(input.TemperatureCelsius, 0):
		return nil, errors.New("temperature_celsius が不正です")
	}

	previous, err := s.store.GetSensorDataByUserID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}

	recordedAt := input.RecordedAt.UTC()
	if previous != nil && recordedAt.Before(previous.RecordedAt) {
		return nil, ErrSensorReadingOutOfOrder
	}

	status, sanitizedMoisture, sanitizedLight, sanitizedTemperature, issues := normalizeSensorReading(input, time.Now().UTC())

	sensorData, err := s.store.UpsertSensorData(ctx, models.UpsertSensorDataInput{
		UserID:             input.UserID,
		DeviceID:           input.DeviceID,
		MoisturePercent:    sanitizedMoisture,
		LightLux:           sanitizedLight,
		TemperatureCelsius: sanitizedTemperature,
		RecordedAt:         recordedAt,
		SensorStatus:       status,
		RawPayload: map[string]any{
			"device_id":           input.DeviceID,
			"recorded_at":         recordedAt.Format(time.RFC3339),
			"moisture_percent":    input.MoisturePercent,
			"light_lux":           input.LightLux,
			"temperature_celsius": input.TemperatureCelsius,
			"sensor_issues":       issues,
		},
	})
	if err != nil {
		return nil, err
	}

	result := &IngestSensorResult{
		SensorData: sensorData,
	}

	if !shouldTriggerWateringEvent(previous, sensorData) {
		return result, nil
	}

	latestEvent, err := s.store.GetLatestChatHistoryByRole(ctx, input.UserID, sensorEventRole)
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

	eventMessage, err := s.generateWateringReaction(ctx, profile, previous, sensorData)
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
	result.EventName = sensorEventNameWatering
	result.EventMessage = eventMessage

	s.broadcastEvent(input.UserID, realtime.Message{
		Type:   "reply_done",
		Source: sensorEventSource,
		Event:  sensorEventNameWatering,
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
				Event:  sensorEventNameWatering,
				Tag:    sensorEventNameWatering,
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
			"event":                     sensorEventNameWatering,
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

func (s *SensorService) generateWateringReaction(
	ctx context.Context,
	profile *models.BonsaiProfile,
	previous *models.SensorData,
	current *models.SensorData,
) (string, error) {
	if s.llm == nil {
		return "", errors.New("イベント通知用のLLMが設定されていません")
	}

	var builder strings.Builder
	err := s.llm.StreamChat(ctx, llm.ChatRequest{
		UserID:       current.UserID,
		SystemPrompt: buildWateringSystemPrompt(profile),
		Message:      buildWateringUserPrompt(previous, current),
	}, func(text string) error {
		builder.WriteString(text)
		s.broadcastEvent(current.UserID, realtime.Message{
			Type:   "reply_chunk",
			Text:   text,
			Source: sensorEventSource,
			Event:  sensorEventNameWatering,
		})
		return nil
	})
	if err != nil {
		s.broadcastEvent(current.UserID, realtime.Message{
			Type:    "error",
			Message: "センサーイベントの生成に失敗しました",
			Source:  sensorEventSource,
			Event:   sensorEventNameWatering,
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
		"あなたは大切に育てられている盆栽です。名前は「%s」です。性格は「%s」です。ユーザーへ自分の身体感覚として自然に話してください。日本語で1〜2文、20〜60文字程度に収め、絵文字・箇条書き・引用符は使わないでください。",
		name,
		personality,
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
