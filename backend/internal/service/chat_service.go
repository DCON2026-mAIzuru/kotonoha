package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

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
	DeleteBonsaiMemoriesByUserID(ctx context.Context, userID string) (int64, error)
	DeleteChatHistoryByUserID(ctx context.Context, userID string) (int64, error)
	GetBonsaiProfileByUserID(ctx context.Context, userID string) (*models.BonsaiProfile, error)
	GetSensorDataByUserID(ctx context.Context, userID string) (*models.SensorData, error)
	ListRecentChatHistoryByUserID(ctx context.Context, userID string, limit int) ([]models.ChatHistoryEntry, error)
	MatchBonsaiMemories(ctx context.Context, userID string, embedding []float64, limit int) ([]models.BonsaiMemory, error)
	UpdateBonsaiBondScore(ctx context.Context, userID string, score int) (*models.BonsaiProfile, error)
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

type StreamChatMeta struct {
	Typing TypingProfile `json:"typing"`
}

type ClearChatHistoryResult struct {
	DeletedHistoryCount int64 `json:"deleted_history_count"`
	DeletedMemoryCount  int64 `json:"deleted_memory_count"`
}

func NewChatService(store chatStore, llmStreamer chatLLMStreamer, embedder chatTextEmbedder) *ChatService {
	return &ChatService{
		store:       store,
		llmStreamer: llmStreamer,
		embedder:    embedder,
	}
}

func (s *ChatService) PrepareStreamMeta(ctx context.Context, userID string) (*StreamChatMeta, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("チャットサービスが設定されていません")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("ユーザーIDが空です")
	}

	profile, err := s.store.GetBonsaiProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	sensorData, err := s.store.GetSensorDataByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &StreamChatMeta{Typing: typingProfileFor(profile, sensorData)}, nil
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

	if profile != nil {
		nextScore := applyBondDelta(profile.BondScore, bondDeltaForEvent("talked"))
		if nextScore != profile.BondScore {
			if _, err := s.store.UpdateBonsaiBondScore(ctx, input.UserID, nextScore); err != nil {
				log.Printf("会話によるなじみ具合の更新に失敗しました: user_id=%s err=%v", input.UserID, err)
			}
		}
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

func (s *ChatService) ClearHistory(ctx context.Context, userID string) (*ClearChatHistoryResult, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("チャットサービスが設定されていません")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("ユーザーIDが空です")
	}

	deletedHistory, err := s.store.DeleteChatHistoryByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	deletedMemories, err := s.store.DeleteBonsaiMemoriesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &ClearChatHistoryResult{
		DeletedHistoryCount: deletedHistory,
		DeletedMemoryCount:  deletedMemories,
	}, nil
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
	builder.WriteString(buildBasePrinciplePrompt())
	builder.WriteString("\n\n---\n\n## この個体の樹格\n\n")
	builder.WriteString(buildProfilePromptSection(profile))
	if backstorySection := buildBackstoryPromptSection(profile); backstorySection != "" {
		builder.WriteString("\n")
		builder.WriteString(backstorySection)
	}
	builder.WriteString("\n\n---\n\n## 今日の性格の出方\n\n")
	builder.WriteString(buildPersonalityStatePromptSection(profile, sensorData))
	builder.WriteString("\n\n---\n\n## 今日の身体\n\n")
	builder.WriteString(buildBodyStatePromptSection(sensorData))
	builder.WriteString("\n\n---\n\n## 育ちの記録\n\n")
	builder.WriteString(buildGrowthStatePromptSection(profile, sensorData))
	builder.WriteString("\n\n---\n\n## 手入れの記録\n\n")
	builder.WriteString(buildCareHistoryPromptSection(recentHistory))
	builder.WriteString("\n\n---\n\n## なじみ具合\n\n")
	builder.WriteString(buildBondStatePromptSection(profile, sensorData))
	builder.WriteString("\n\n---\n\n## 根に残る会話の欠片\n\n")
	builder.WriteString(formatMemoryPromptSection(memories))
	builder.WriteString("\n\n---\n\n## 直近の会話\n\n")
	builder.WriteString(formatRecentHistoryPromptSection(recentHistory))
	builder.WriteString("\n\n---\n\n## 今日の話し方\n\n")
	builder.WriteString(buildToneModePromptSection(profile))

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

func buildBackstoryPromptSection(profile *models.BonsaiProfile) string {
	if profile == nil {
		return ""
	}

	backstory := strings.TrimSpace(profile.Backstory)
	if backstory == "" {
		return ""
	}

	return fmt.Sprintf(
		"【盆栽の来歴】\n%s\nこの来歴は最初から持っている過去の記憶です。現在の状態やユーザーの話題に合うときだけ、短く自然に思い出してください。",
		truncateRunes(backstory, 1000),
	)
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

func truncateRunes(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if max <= 0 || len(runes) <= max {
		return string(runes)
	}

	return string(runes[:max])
}
