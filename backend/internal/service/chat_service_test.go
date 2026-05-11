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
	diagnosis       *models.BonsaiDiagnosis
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

func (s *chatServiceTestStore) GetLatestBonsaiDiagnosis(context.Context, string) (*models.BonsaiDiagnosis, error) {
	return s.diagnosis, nil
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
	callCount   int
}

func (s *chatServiceTestStreamer) StreamChat(_ context.Context, req llm.ChatRequest, onChunk func(text string) error) error {
	s.callCount++
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
		diagnosis: &models.BonsaiDiagnosis{
			HealthScore:   72.4,
			Status:        "warning",
			StatusLabel:   "注意",
			Severity:      "medium",
			SeverityLabel: "中",
			Detail:        "古葉側に黄変・茶変が見られ、弱りの兆候が出ています。",
			Ratios: models.DiagnosisRatios{
				Total:  models.DiagnosisColorRatios{Green: 82.1, Yellow: 12.4, BrownRed: 5.5},
				Top:    models.DiagnosisColorRatios{Green: 94.0, Yellow: 4.1, BrownRed: 1.9},
				Bottom: models.DiagnosisColorRatios{Green: 68.5, Yellow: 21.0, BrownRed: 10.5},
			},
			CreatedAt: time.Date(2026, 4, 30, 16, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
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
		"## 画像から見える葉の様子",
		"latest_image_diagnosis:",
		"health_score: 72.4 / 100",
		"status: 注意",
		"leaf_color_total: green 82.1%, yellow 12.4%, brown 5.5%",
		"写真から見える葉色の手がかり",
		"## なじみ具合",
		"bond_score: 92",
		"ぼんす語尾",
		"## 返答の優先順位",
		"センサー値、画像診断、会話履歴がずれる時",
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

func TestStreamReplyStartsSpecialDCONReplyThenGeneratesContinuation(t *testing.T) {
	t.Parallel()

	store := &chatServiceTestStore{
		profile: &models.BonsaiProfile{
			UserID:    "user-1",
			BondScore: 50,
		},
		sensorData: &models.SensorData{
			UserID:             "user-1",
			DeviceID:           "sensor-1",
			MoisturePercent:    45,
			LightLux:           1000,
			TemperatureCelsius: 24,
			RecordedAt:         time.Date(2026, 5, 5, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60)),
			SensorStatus:       sensorStatusNormal,
		},
	}
	streamer := &chatServiceTestStreamer{reply: " 緊張しているぶん、準備してきた根はちゃんと踏んばれるよ。"}
	service := NewChatService(store, streamer, &chatServiceTestEmbedder{})

	var streamed strings.Builder
	err := service.StreamReply(context.Background(), StreamChatInput{
		UserID: "user-1",
		Text:   "DCON本戦緊張するよ",
	}, func(text string) error {
		streamed.WriteString(text)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamReply returned error: %v", err)
	}

	const want = "頑張って！君たちならうまくいくよ！ 緊張しているぶん、準備してきた根はちゃんと踏んばれるよ。"
	if got := streamed.String(); got != want {
		t.Fatalf("streamed reply = %q, want %q", got, want)
	}
	if streamer.callCount != 1 {
		t.Fatalf("expected LLM streamer to be called once, got %d calls", streamer.callCount)
	}
	if !strings.Contains(streamer.lastRequest.SystemPrompt, "同じ文を繰り返さず") {
		t.Fatalf("expected continuation guidance in prompt: %s", streamer.lastRequest.SystemPrompt)
	}
	if len(store.createdHistory) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(store.createdHistory))
	}
	if got := store.createdHistory[1].Content; got != want {
		t.Fatalf("assistant history = %q, want %q", got, want)
	}
}

func TestBuildResponsePriorityPromptSectionWarnsOnUnhealthyDiagnosis(t *testing.T) {
	t.Parallel()

	prompt := buildResponsePriorityPromptSection(&models.BonsaiDiagnosis{
		HealthScore:   41.2,
		Status:        "warning",
		StatusLabel:   "不健康",
		Severity:      "high",
		SeverityLabel: "高",
	})

	for _, want := range []string{
		"「大丈夫」「安心している」「落ち着いている」だけで終えない",
		"根元の水分は足りているけれど、葉色はまだ弱って見える",
		"光・水・根の状態を見直す",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q\nprompt=%s", want, prompt)
		}
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
