package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/apple/kotonoha/backend/internal/llm"
	"github.com/apple/kotonoha/backend/internal/models"
)

type chatServiceTestStore struct {
	profile         *models.BonsaiProfile
	sensorData      *models.SensorData
	recentHistory   []models.ChatHistoryEntry
	memories        []models.BonsaiMemory
	createdHistory  []models.CreateChatHistoryEntryInput
	createdMemories []models.CreateBonsaiMemoryInput
}

func (s *chatServiceTestStore) CreateChatHistoryEntry(_ context.Context, input models.CreateChatHistoryEntryInput) (*models.ChatHistoryEntry, error) {
	s.createdHistory = append(s.createdHistory, input)
	return &models.ChatHistoryEntry{
		ID:        "history-" + input.Role,
		UserID:    input.UserID,
		Role:      input.Role,
		Content:   input.Content,
		CreatedAt: time.Now(),
	}, nil
}

func (s *chatServiceTestStore) CreateBonsaiMemory(_ context.Context, input models.CreateBonsaiMemoryInput) error {
	s.createdMemories = append(s.createdMemories, input)
	return nil
}

func (s *chatServiceTestStore) DeleteBonsaiMemoriesByUserID(context.Context, string) (int64, error) {
	return int64(len(s.createdMemories)), nil
}

func (s *chatServiceTestStore) DeleteChatHistoryByUserID(context.Context, string) (int64, error) {
	return int64(len(s.createdHistory)), nil
}

func (s *chatServiceTestStore) GetBonsaiProfileByUserID(context.Context, string) (*models.BonsaiProfile, error) {
	return s.profile, nil
}

func (s *chatServiceTestStore) GetSensorDataByUserID(context.Context, string) (*models.SensorData, error) {
	return s.sensorData, nil
}

func (s *chatServiceTestStore) ListRecentChatHistoryByUserID(context.Context, string, int) ([]models.ChatHistoryEntry, error) {
	return s.recentHistory, nil
}

func (s *chatServiceTestStore) MatchBonsaiMemories(context.Context, string, []float64, int) ([]models.BonsaiMemory, error) {
	return s.memories, nil
}

func (s *chatServiceTestStore) UpdateBonsaiBondScore(_ context.Context, _ string, score int) (*models.BonsaiProfile, error) {
	if s.profile != nil {
		s.profile.BondScore = score
	}
	return s.profile, nil
}

type chatServiceTestStreamer struct {
	lastRequest llm.ChatRequest
	reply       string
}

func (s *chatServiceTestStreamer) StreamChat(_ context.Context, req llm.ChatRequest, onChunk func(text string) error) error {
	s.lastRequest = req
	return onChunk(s.reply)
}

type chatServiceTestEmbedder struct{}

func (e *chatServiceTestEmbedder) Embed(context.Context, llm.EmbeddingRequest) ([]float64, error) {
	return []float64{0.1, 0.2, 0.3}, nil
}

func TestStreamReplyBuildsDynamicPrompt(t *testing.T) {
	t.Parallel()

	store := &chatServiceTestStore{
		profile: &models.BonsaiProfile{
			UserID:      "user-1",
			Name:        "言の葉",
			Personality: "古風なおじいちゃん",
			Backstory:   "前の持ち主は朝に水をくれる人だった。昔は別の庭で育っていた。",
			ToneMode:    ToneModeGentle,
			BondScore:   92,
		},
		sensorData: &models.SensorData{
			UserID:             "user-1",
			DeviceID:           "sensor-1",
			MoisturePercent:    40,
			LightLux:           1200,
			TemperatureCelsius: -50,
			RecordedAt:         time.Date(2026, 4, 14, 10, 30, 0, 0, time.FixedZone("JST", 9*60*60)),
			SensorStatus:       sensorStatusInvalid,
			RawPayload: map[string]any{
				"temperature_celsius": -127.0,
			},
		},
		recentHistory: []models.ChatHistoryEntry{
			{Role: "assistant", Content: "根元は少し静かです。"},
			{Role: "event", Content: "お水をもらえて助かりました。"},
		},
		memories: []models.BonsaiMemory{
			{Summary: "ユーザー: 先週は日当たりのよい窓辺に移してくれた\n盆栽: 葉がのびのびしました"},
		},
	}
	streamer := &chatServiceTestStreamer{reply: "こんにちは。今日は少し感覚が不思議ですが、声をかけてもらえて嬉しいです。"}

	service := NewChatService(store, streamer, &chatServiceTestEmbedder{})

	err := service.StreamReply(context.Background(), StreamChatInput{
		UserID: "user-1",
		Text:   "今日はどんな気分？",
	}, func(string) error {
		return nil
	})
	if err != nil {
		t.Fatalf("StreamReply returned error: %v", err)
	}

	prompt := streamer.lastRequest.SystemPrompt
	for _, want := range []string{
		"言の葉",
		"古風なおじいちゃん",
		"bonsai_dna:",
		"trust_pattern: slow_to_trust",
		"## 今日の性格の出方",
		"personality_state:",
		"custom_personality_hint: 古風なおじいちゃん",
		"## 会話の見立て",
		"conversation_intent: question",
		"relationship_phase: early_contact",
		"直近で使いすぎている表現:",
		"- 少し",
		"- 根元",
		"前の持ち主は朝に水をくれる人だった",
		"昔は別の庭で育っていた",
		"## 今日の身体",
		"## なじみ具合",
		"bond_score: 92",
		"ぼんす語尾",
		"根元は少し静かです。",
		"お水をもらえて助かりました。",
		"先週は日当たりのよい窓辺に移してくれた",
		"身体の感覚に乱れがあり",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("system prompt does not contain %q\nprompt=%s", want, prompt)
		}
	}

	if len(store.createdHistory) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(store.createdHistory))
	}
	if len(store.createdMemories) != 1 {
		t.Fatalf("expected 1 archived memory, got %d", len(store.createdMemories))
	}
}

func TestBuildConversationContextPromptSection(t *testing.T) {
	t.Parallel()

	prompt := buildConversationContextPromptSection("ただいま。今日つかれた", []models.ChatHistoryEntry{
		{Role: "assistant", Content: "根元は少し静かだよ。"},
		{Role: "user", Content: "おはよう"},
		{Role: "assistant", Content: "葉の先が少し起きた。"},
	})

	for _, want := range []string{
		"conversation_intent: user_low_energy",
		"relationship_phase: ongoing",
		"直近で使いすぎている表現:",
		"- 少し",
		"- 根元",
		"- 葉の先",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q\nprompt=%s", want, prompt)
		}
	}
}

func TestBuildConversationContextPromptSectionFirstContact(t *testing.T) {
	t.Parallel()

	prompt := buildConversationContextPromptSection("今日カレー食べた", nil)

	for _, want := range []string{
		"conversation_intent: light_chat",
		"relationship_phase: first_contact",
		"初回として扱う",
		"直近で強く避けるべき繰り返し表現はない",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q\nprompt=%s", want, prompt)
		}
	}
}

func TestFormatSensorPromptSectionIncludesMoistureState(t *testing.T) {
	t.Parallel()

	prompt := buildBodyStatePromptSection(&models.SensorData{
		MoisturePercent:    12,
		LightLux:           800,
		TemperatureCelsius: 22,
		RecordedAt:         time.Date(2026, 4, 18, 14, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
		SensorStatus:       sensorStatusNormal,
	})

	for _, want := range []string{
		"根元はかなり乾いていて",
		"葉に届く光はやわらかい",
		"鉢の温度は落ち着いている",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q\nprompt=%s", want, prompt)
		}
	}
}

func TestFormatSensorPromptSectionIncludesHighTensionForWetState(t *testing.T) {
	t.Parallel()

	prompt := buildBodyStatePromptSection(&models.SensorData{
		MoisturePercent:    92,
		LightLux:           1400,
		TemperatureCelsius: 24,
		RecordedAt:         time.Date(2026, 4, 18, 14, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
		SensorStatus:       sensorStatusNormal,
	})

	for _, want := range []string{
		"根元には水がたっぷり巡っていて",
		"葉に届く光はやわらかい",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q\nprompt=%s", want, prompt)
		}
	}
}
