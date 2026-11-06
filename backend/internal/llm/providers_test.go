package llm

import (
	"context"
	"testing"
)

func TestDisabledStreamerReturnsErrorWithoutEmittingReply(t *testing.T) {
	t.Parallel()
	streamer, err := NewConfiguredStreamer(StreamerConfig{Provider: "disabled"})
	if err != nil {
		t.Fatal(err)
	}
	emitted := false
	err = streamer.StreamChat(context.Background(), ChatRequest{Message: "こんにちは"}, func(string) error {
		emitted = true
		return nil
	})
	if err == nil || err.Error() != "LLMが未接続です。Colabの復旧と接続設定が必要です" {
		t.Fatalf("disabled model error = %v", err)
	}
	if emitted {
		t.Fatal("disabled model must not emit a simulated response")
	}
}

func TestConfiguredProvidersDoNotSilentlyUseFallbacks(t *testing.T) {
	t.Parallel()
	if _, err := NewConfiguredStreamer(StreamerConfig{}); err == nil {
		t.Fatal("missing Gemini key must not select a fixed response")
	}
	if _, err := NewConfiguredStreamer(StreamerConfig{Provider: "openai-compatible", BaseURL: "https://host/v1"}); err == nil {
		t.Fatal("missing Colab model must fail")
	}
	if _, err := NewConfiguredStreamer(StreamerConfig{Provider: "typo"}); err == nil {
		t.Fatal("unknown provider must fail")
	}
	embedder, err := NewConfiguredEmbedder(context.Background(), EmbedderConfig{})
	if err != nil || embedder != nil {
		t.Fatalf("unconfigured embeddings must be disabled, got %T, %v", embedder, err)
	}
	if _, err := NewConfiguredEmbedder(context.Background(), EmbedderConfig{Provider: "gemini"}); err == nil {
		t.Fatal("explicit Gemini embeddings require a key")
	}
	embedder, err = NewConfiguredEmbedder(context.Background(), EmbedderConfig{Provider: "disabled", GeminiAPIKey: "existing-gemini-key"})
	if err != nil || embedder != nil {
		t.Fatal("explicit disabled must override existing Gemini credentials")
	}
	streamer, err := NewConfiguredStreamer(StreamerConfig{Provider: "fallback"})
	if _, ok := streamer.(*FallbackStreamer); err != nil || !ok {
		t.Fatal("explicit development fallback is unavailable")
	}
}
