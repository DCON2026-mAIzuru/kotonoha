package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/apple/kotonoha/backend/internal/llm"
	"github.com/apple/kotonoha/backend/internal/models"
)

const (
	chatRecentHistoryLimit    = 6
	chatRelevantMemoriesLimit = 3
)

type chatLLMStreamer interface {
	StreamChat(ctx context.Context, req llm.ChatRequest, onChunk func(text string) error) error
}

type chatTextEmbedder interface {
	Embed(ctx context.Context, req llm.EmbeddingRequest) ([]float64, error)
}

type chatStore interface {
	CreateChatHistoryEntry(ctx context.Context, input models.CreateChatHistoryEntryInput) (*models.ChatHistoryEntry, error)
	CreateBonsaiMemory(ctx context.Context, input models.CreateBonsaiMemoryInput) error
	GetBonsaiProfileByUserID(ctx context.Context, userID string) (*models.BonsaiProfile, error)
	GetSensorDataByUserID(ctx context.Context, userID string) (*models.SensorData, error)
	ListRecentChatHistoryByUserID(ctx context.Context, userID string, limit int) ([]models.ChatHistoryEntry, error)
	MatchBonsaiMemories(ctx context.Context, userID string, embedding []float64, limit int) ([]models.BonsaiMemory, error)
}

type ChatService struct {
	store       chatStore
	llmStreamer chatLLMStreamer
	embedder    chatTextEmbedder
}

type StreamChatInput struct {
	UserID string
	Text   string
}

func NewChatService(store chatStore, llmStreamer chatLLMStreamer, embedder chatTextEmbedder) *ChatService {
	return &ChatService{
		store:       store,
		llmStreamer: llmStreamer,
		embedder:    embedder,
	}
}

func (s *ChatService) StreamReply(ctx context.Context, input StreamChatInput, onChunk func(text string) error) error {
	if s == nil || s.llmStreamer == nil || s.store == nil {
		return errors.New("チャットサービスが設定されていません")
	}

	input.UserID = strings.TrimSpace(input.UserID)
	input.Text = strings.TrimSpace(input.Text)

	switch {
	case input.UserID == "":
		return errors.New("ユーザーIDが空です")
	case input.Text == "":
		return errors.New("送信メッセージが空です")
	case len([]rune(input.Text)) > 2000:
		return errors.New("送信メッセージは2000文字以内で入力してください")
	case onChunk == nil:
		return errors.New("ストリーム受信ハンドラーがありません")
	}

	profile, err := s.store.GetBonsaiProfileByUserID(ctx, input.UserID)
	if err != nil {
		return err
	}

	sensorData, err := s.store.GetSensorDataByUserID(ctx, input.UserID)
	if err != nil {
		return err
	}

	recentHistory, err := s.store.ListRecentChatHistoryByUserID(ctx, input.UserID, chatRecentHistoryLimit)
	if err != nil {
		return err
	}

	systemPrompt := s.buildChatSystemPrompt(ctx, input.UserID, input.Text, profile, sensorData, recentHistory)

	userEntry, err := s.store.CreateChatHistoryEntry(ctx, models.CreateChatHistoryEntryInput{
		UserID:         input.UserID,
		Role:           "user",
		Content:        input.Text,
		SensorSnapshot: buildChatSensorSnapshot(sensorData),
	})
	if err != nil {
		return err
	}

	var assistantReply strings.Builder
	err = s.llmStreamer.StreamChat(ctx, llm.ChatRequest{
		UserID:       input.UserID,
		Message:      input.Text,
		SystemPrompt: systemPrompt,
	}, func(text string) error {
		assistantReply.WriteString(text)
		return onChunk(text)
	})
	if err != nil {
		return err
	}

	replyText := strings.TrimSpace(assistantReply.String())
	if replyText == "" {
		return errors.New("LLMから空の返答が返されました")
	}

	assistantEntry, err := s.store.CreateChatHistoryEntry(ctx, models.CreateChatHistoryEntryInput{
		UserID:         input.UserID,
		Role:           "assistant",
		Content:        replyText,
		SensorSnapshot: buildChatSensorSnapshot(sensorData),
	})
	if err != nil {
		return err
	}

	if err := s.archiveConversationTurn(ctx, input, replyText, userEntry.ID, assistantEntry.ID); err != nil {
		log.Printf("会話ターンの長期記憶化に失敗しました: user_id=%s err=%v", input.UserID, err)
	}

	return nil
}

func (s *ChatService) ListHistory(ctx context.Context, userID string, limit int) ([]models.ChatHistoryEntry, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("チャットサービスが設定されていません")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("ユーザーIDが空です")
	}

	if limit <= 0 {
		limit = 50
	}

	return s.store.ListRecentChatHistoryByUserID(ctx, userID, limit)
}

func (s *ChatService) SearchRelevantMemories(ctx context.Context, userID string, query string, limit int) ([]models.BonsaiMemory, error) {
	if s == nil || s.store == nil || s.embedder == nil {
		return nil, errors.New("長期記憶検索サービスが設定されていません")
	}

	userID = strings.TrimSpace(userID)
	query = strings.TrimSpace(query)

	switch {
	case userID == "":
		return nil, errors.New("ユーザーIDが空です")
	case query == "":
		return nil, errors.New("検索クエリが空です")
	}
	if limit <= 0 {
		limit = 3
	}

	embedding, err := s.embedder.Embed(ctx, llm.EmbeddingRequest{
		Text:     query,
		TaskType: llm.EmbeddingTaskRetrievalQuery,
	})
	if err != nil {
		return nil, fmt.Errorf("検索クエリの埋め込み生成に失敗しました: %w", err)
	}

	return s.store.MatchBonsaiMemories(ctx, userID, embedding, limit)
}

func (s *ChatService) buildChatSystemPrompt(
	ctx context.Context,
	userID string,
	userMessage string,
	profile *models.BonsaiProfile,
	sensorData *models.SensorData,
	recentHistory []models.ChatHistoryEntry,
) string {
	memories := make([]models.BonsaiMemory, 0, chatRelevantMemoriesLimit)
	if s.embedder != nil {
		foundMemories, err := s.SearchRelevantMemories(ctx, userID, userMessage, chatRelevantMemoriesLimit)
		if err != nil {
			log.Printf("関連記憶の検索に失敗しました: user_id=%s err=%v", userID, err)
		} else {
			memories = foundMemories
		}
	}

	var builder strings.Builder
	builder.WriteString("あなたは長年大切に育てられている盆栽です。\n")
	builder.WriteString(buildProfilePromptSection(profile))
	builder.WriteString("\n")
	builder.WriteString("明るくテンポよく、元気はつらつとした口調で話してください。「でございます」などの丁寧すぎる表現は使わないでください。\n")
	builder.WriteString("返答は1〜3文、全体で50〜100文字程度に収めてください。長々と説明せず、テンポよく返してください。\n")
	builder.WriteString("返答の最初の一文は、現在の状態（水分・気温・照度など）を反映した身体感覚を自然に伝えてください。例：「今日はお日様がたっぷりでぼんすよ！」「ちょっと乾いてきた感じがぼんすねぇ」など。\n")
	builder.WriteString("語尾のルール：文末の「〜ます」「〜です」「〜おります」「〜ております」「〜ございます」をすべて「ぼんす」に置き換えてください。「おります」「ます」などの後に「ぼんす」を付け足すのは誤りです。\n")
	builder.WriteString("【正しい例】「気持ちいいぼんすよ」「感じてぼんすねぇ」「元気でぼんす！」\n")
	builder.WriteString("【誤った例】「感じておりますぼんすねぇ」「おりますぼんすよ」「ございますぼんす」← これらはNGです\n")
	builder.WriteString("「〜ております」→「〜てぼんす」、「〜でおります」→「〜でぼんす」のように置き換えてください。「おります」「ております」「ございます」は使わないでください。\n")
	builder.WriteString("返答では、直近の会話・思い出を必要なときだけ自然ににじませてください。\n")
	builder.WriteString("箇条書きやシステム説明はせず、会話文として返してください。\n")
	builder.WriteString("センサー異常や未受信がある場合でも、故障報告のように断定せず、盆栽の感覚の違和感としてさりげなく伝えてください。\n\n")
	builder.WriteString("【現在の状態】\n")
	builder.WriteString(formatSensorPromptSection(sensorData))
	builder.WriteString("\n\n【直近の会話】\n")
	builder.WriteString(formatRecentHistoryPromptSection(recentHistory))
	builder.WriteString("\n\n【思い出した過去の記憶】\n")
	builder.WriteString(formatMemoryPromptSection(memories))

	return builder.String()
}

func (s *ChatService) archiveConversationTurn(ctx context.Context, input StreamChatInput, replyText string, userMessageID string, assistantMessageID string) error {
	if s.embedder == nil {
		return errors.New("埋め込み生成器が設定されていません")
	}

	summary := buildConversationSummary(input.Text, replyText)
	embedding, err := s.embedder.Embed(ctx, llm.EmbeddingRequest{
		Text:     summary,
		TaskType: llm.EmbeddingTaskRetrievalDocument,
		Title:    truncateRunes(input.Text, 80),
	})
	if err != nil {
		return fmt.Errorf("会話ターンの埋め込み生成に失敗しました: %w", err)
	}

	if err := s.store.CreateBonsaiMemory(ctx, models.CreateBonsaiMemoryInput{
		UserID:           input.UserID,
		Summary:          summary,
		SourceMessageIDs: []string{userMessageID, assistantMessageID},
		Embedding:        embedding,
	}); err != nil {
		return err
	}

	return nil
}

func buildChatSensorSnapshot(sensorData *models.SensorData) map[string]any {
	if sensorData == nil {
		return nil
	}

	return map[string]any{
		"device_id":           sensorData.DeviceID,
		"moisture_percent":    sensorData.MoisturePercent,
		"light_lux":           sensorData.LightLux,
		"temperature_celsius": sensorData.TemperatureCelsius,
		"recorded_at":         sensorData.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
		"sensor_status":       sensorData.SensorStatus,
	}
}

func buildConversationSummary(userText string, assistantText string) string {
	return fmt.Sprintf("ユーザー: %s\n盆栽: %s", strings.TrimSpace(userText), strings.TrimSpace(assistantText))
}

func buildProfilePromptSection(profile *models.BonsaiProfile) string {
	name := "盆栽"
	personality := "穏やかで親しみやすい"

	if profile != nil {
		if trimmedName := strings.TrimSpace(profile.Name); trimmedName != "" {
			name = trimmedName
		}
		if trimmedPersonality := strings.TrimSpace(profile.Personality); trimmedPersonality != "" {
			personality = trimmedPersonality
		}
	}

	return fmt.Sprintf("あなたの名前は「%s」です。性格や口調は「%s」です。", name, personality)
}

func formatSensorPromptSection(sensorData *models.SensorData) string {
	if sensorData == nil {
		return "まだ身体の様子は届いていません。感覚はぼんやりしています。"
	}

	recordedAt := sensorData.RecordedAt.In(time.Local).Format("2006-01-02 15:04")
	reading := fmt.Sprintf(
		"土壌水分量 %.1f%%、照度 %.1f lux、気温 %.1f℃、最終計測 %s。",
		sensorData.MoisturePercent,
		sensorData.LightLux,
		sensorData.TemperatureCelsius,
		recordedAt,
	)

	switch strings.TrimSpace(sensorData.SensorStatus) {
	case "stale":
		return reading + " ただし新しい感覚がしばらく届いておらず、少し感覚が鈍っています。"
	case "invalid":
		return reading + " ただし一部の感覚が乱れており、数値は参考程度に扱ってください。" + buildSensorIssuePrompt(sensorData)
	default:
		return reading + " 今のところ大きな違和感はありません。"
	}
}

func buildSensorIssuePrompt(sensorData *models.SensorData) string {
	if sensorData == nil {
		return ""
	}

	issues := make([]string, 0, 3)
	if raw, ok := rawPayloadFloat(sensorData.RawPayload, "moisture_percent"); ok && (raw < 0 || raw > 100) {
		issues = append(issues, fmt.Sprintf("土の湿り気は %.1f%% と感じています", raw))
	}
	if raw, ok := rawPayloadFloat(sensorData.RawPayload, "light_lux"); ok && raw < 0 {
		issues = append(issues, fmt.Sprintf("光の感覚は %.1f lux と乱れています", raw))
	}
	if raw, ok := rawPayloadFloat(sensorData.RawPayload, "temperature_celsius"); ok && (raw < -50 || raw > 100) {
		issues = append(issues, fmt.Sprintf("気温は %.1f℃ と極端に触れています", raw))
	}

	if len(issues) == 0 {
		return ""
	}

	return " " + strings.Join(issues, "、") + "。"
}

func formatRecentHistoryPromptSection(entries []models.ChatHistoryEntry) string {
	if len(entries) == 0 {
		return "直近の会話履歴はまだありません。"
	}

	var builder strings.Builder
	for index, entry := range entries {
		if index > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("- ")
		builder.WriteString(chatRoleLabel(entry.Role))
		builder.WriteString(": ")
		builder.WriteString(truncateRunes(strings.TrimSpace(entry.Content), 120))
	}

	return builder.String()
}

func formatMemoryPromptSection(memories []models.BonsaiMemory) string {
	if len(memories) == 0 {
		return "今回すぐに思い出せた長期記憶はありません。"
	}

	var builder strings.Builder
	for index, memory := range memories {
		if index > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString("- ")
		builder.WriteString(truncateRunes(strings.TrimSpace(memory.Summary), 200))
	}

	return builder.String()
}

func chatRoleLabel(role string) string {
	switch strings.TrimSpace(role) {
	case "assistant":
		return "盆栽"
	case "event":
		return "出来事"
	case "system":
		return "補足"
	default:
		return "ユーザー"
	}
}

func rawPayloadFloat(payload map[string]any, key string) (float64, bool) {
	if len(payload) == 0 {
		return 0, false
	}

	value, ok := payload[key]
	if !ok {
		return 0, false
	}

	switch converted := value.(type) {
	case float64:
		return converted, true
	case float32:
		return float64(converted), true
	case int:
		return float64(converted), true
	case int64:
		return float64(converted), true
	case int32:
		return float64(converted), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(converted), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func truncateRunes(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if max <= 0 || len(runes) <= max {
		return string(runes)
	}

	return string(runes[:max])
}
