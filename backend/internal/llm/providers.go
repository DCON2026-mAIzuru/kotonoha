package llm

import (
	"context"
	"errors"
	"strings"
	"time"
)

type StreamerConfig struct {
	Provider            string
	BaseURL             string
	APIKey              string
	Model               string
	MaxTokens           int
	Timeout             time.Duration
	GeminiAPIKey        string
	GeminiModel         string
	GeminiModelPriority string
}

func NewConfiguredStreamer(cfg StreamerConfig) (Streamer, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "gemini":
		if strings.TrimSpace(cfg.GeminiAPIKey) == "" {
			return nil, errors.New("Gemini を使うには GEMINI_API_KEY が必要です。Colab を使う場合は LLM_PROVIDER=openai-compatible を設定してください")
		}
		return NewStreamer(cfg.GeminiAPIKey, cfg.GeminiModel, cfg.GeminiModelPriority), nil
	case "openai-compatible":
		return NewOpenAICompatibleStreamer(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.MaxTokens, cfg.Timeout)
	case "disabled":
		return disabledStreamer{}, nil
	case "fallback":
		return NewFallbackStreamer(), nil
	default:
		return nil, errors.New("LLM_PROVIDER は gemini、openai-compatible、disabled、fallback のいずれかを設定してください")
	}
}

type disabledStreamer struct{}

func (disabledStreamer) StreamChat(context.Context, ChatRequest, func(string) error) error {
	return errors.New("LLMが未接続です。Colabの復旧と接続設定が必要です")
}

// Embeddings are independent of the chat model. Without a configured embedding
// provider, history is still saved but no artificial vectors enter the database.
func NewConfiguredEmbedder(provider, geminiAPIKey, geminiModel string) (Embedder, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		if strings.TrimSpace(geminiAPIKey) == "" {
			provider = "disabled"
		} else {
			provider = "gemini"
		}
	}
	switch provider {
	case "disabled":
		return nil, nil
	case "gemini":
		if strings.TrimSpace(geminiAPIKey) == "" {
			return nil, errors.New("EMBEDDING_PROVIDER=gemini には GEMINI_API_KEY が必要です")
		}
		return NewEmbedder(geminiAPIKey, geminiModel), nil
	case "fallback":
		return NewFallbackEmbedder(), nil
	default:
		return nil, errors.New("EMBEDDING_PROVIDER は gemini、disabled、fallback のいずれかを設定してください")
	}
}
