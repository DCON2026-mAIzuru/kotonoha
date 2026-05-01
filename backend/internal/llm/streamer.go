package llm

import (
	"context"
	"errors"
	"log"
	"net/http"
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

type failoverStreamer struct {
	streamers []namedStreamer
}

type namedStreamer struct {
	model    string
	streamer Streamer
}

func NewStreamer(geminiAPIKey string, geminiModel string, geminiModelPriority string) Streamer {
	if strings.TrimSpace(geminiAPIKey) == "" {
		return NewFallbackStreamer()
	}

	models := resolveGeminiModelPriority(geminiModel, geminiModelPriority)
	streamers := make([]namedStreamer, 0, len(models))
	for _, model := range models {
		streamers = append(streamers, namedStreamer{
			model:    model,
			streamer: NewGeminiStreamer(geminiAPIKey, model, nil),
		})
	}

	if len(streamers) == 1 {
		return streamers[0].streamer
	}

	return &failoverStreamer{streamers: streamers}
}

func (s *failoverStreamer) StreamChat(ctx context.Context, req ChatRequest, onChunk func(text string) error) error {
	var lastErr error

	for index, candidate := range s.streamers {
		emitted := false
		err := candidate.streamer.StreamChat(ctx, req, func(text string) error {
			emitted = true
			return onChunk(text)
		})
		if err == nil {
			if index > 0 {
				log.Printf("Geminiフォールバックに成功しました: model=%s", candidate.model)
			}
			return nil
		}

		lastErr = err
		if emitted || !shouldFailoverGeminiError(err) || index == len(s.streamers)-1 {
			return err
		}

		log.Printf("Geminiモデルを切り替えます: from=%s reason=%v", candidate.model, err)
	}

	return lastErr
}

func resolveGeminiModelPriority(geminiModel string, geminiModelPriority string) []string {
	defaults := []string{
		"gemini-3-flash-preview",
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
	}

	var raw []string
	if strings.TrimSpace(geminiModelPriority) != "" {
		raw = strings.Split(geminiModelPriority, ",")
	} else if strings.TrimSpace(geminiModel) != "" {
		raw = append(raw, strings.Split(geminiModel, ",")...)
		raw = append(raw, defaults...)
	} else {
		raw = defaults
	}

	seen := make(map[string]struct{}, len(raw))
	models := make([]string, 0, len(raw))
	for _, candidate := range raw {
		model := strings.TrimSpace(candidate)
		if model == "" {
			continue
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}

	if len(models) == 0 {
		return defaults
	}

	return models
}

func shouldFailoverGeminiError(err error) bool {
	var apiErr *GeminiAPIError
	if !errors.As(err, &apiErr) {
		return false
	}

	return apiErr.StatusCode == http.StatusTooManyRequests ||
		apiErr.StatusCode == http.StatusServiceUnavailable ||
		apiErr.Status == "RESOURCE_EXHAUSTED" ||
		apiErr.Status == "UNAVAILABLE"
}
