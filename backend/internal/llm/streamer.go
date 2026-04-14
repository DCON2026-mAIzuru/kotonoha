package llm

import (
	"context"
	"strings"
)

type ChatRequest struct {
	UserID       string
	Message      string
	SystemPrompt string
}

type Streamer interface {
	StreamChat(ctx context.Context, req ChatRequest, onChunk func(text string) error) error
}

func NewStreamer(geminiAPIKey string, geminiModel string) Streamer {
	if strings.TrimSpace(geminiAPIKey) == "" {
		return NewFallbackStreamer()
	}

	model := strings.TrimSpace(geminiModel)
	if model == "" {
		model = "gemini-2.5-flash"
	}

	return NewGeminiStreamer(geminiAPIKey, model, nil)
}
