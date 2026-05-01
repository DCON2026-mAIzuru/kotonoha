package llm

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
)

type stubStreamer struct {
	err    error
	chunks []string
}

func (s stubStreamer) StreamChat(ctx context.Context, req ChatRequest, onChunk func(string) error) error {
	for _, chunk := range s.chunks {
		if err := onChunk(chunk); err != nil {
			return err
		}
	}
	return s.err
}

func TestResolveGeminiModelPriorityUsesConfiguredOrder(t *testing.T) {
	t.Parallel()

	got := resolveGeminiModelPriority("", " gemini-3-flash-preview, gemini-2.5-flash , gemini-2.5-flash-lite ")
	want := []string{"gemini-3-flash-preview", "gemini-2.5-flash", "gemini-2.5-flash-lite"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveGeminiModelPriority() = %v, want %v", got, want)
	}
}

func TestResolveGeminiModelPriorityDeduplicatesModels(t *testing.T) {
	t.Parallel()

	got := resolveGeminiModelPriority("gemini-2.5-flash", "")
	want := []string{"gemini-2.5-flash", "gemini-3-flash-preview", "gemini-2.5-flash-lite"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveGeminiModelPriority() = %v, want %v", got, want)
	}
}

func TestResolveGeminiModelPriorityAcceptsCommaSeparatedGeminiModel(t *testing.T) {
	t.Parallel()

	got := resolveGeminiModelPriority(" gemini-3-flash-preview, gemini-2.5-flash ", "")
	want := []string{"gemini-3-flash-preview", "gemini-2.5-flash", "gemini-2.5-flash-lite"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveGeminiModelPriority() = %v, want %v", got, want)
	}
}

func TestFailoverStreamerFallsBackOnRetryableErrors(t *testing.T) {
	t.Parallel()

	streamer := &failoverStreamer{
		streamers: []namedStreamer{
			{
				model: "gemini-3-flash-preview",
				streamer: stubStreamer{err: &GeminiAPIError{
					StatusCode: http.StatusTooManyRequests,
					Status:     "RESOURCE_EXHAUSTED",
					Message:    "rate limit hit",
				}},
			},
			{
				model:    "gemini-2.5-flash",
				streamer: stubStreamer{chunks: []string{"ok"}},
			},
		},
	}

	var chunks []string
	err := streamer.StreamChat(context.Background(), ChatRequest{Message: "hello"}, func(text string) error {
		chunks = append(chunks, text)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}
	if !reflect.DeepEqual(chunks, []string{"ok"}) {
		t.Fatalf("chunks = %v, want [ok]", chunks)
	}
}

func TestFailoverStreamerDoesNotFallbackAfterPartialEmission(t *testing.T) {
	t.Parallel()

	retryableErr := &GeminiAPIError{
		StatusCode: http.StatusServiceUnavailable,
		Status:     "UNAVAILABLE",
		Message:    "high demand",
	}
	streamer := &failoverStreamer{
		streamers: []namedStreamer{
			{
				model:    "gemini-3-flash-preview",
				streamer: stubStreamer{chunks: []string{"par"}, err: retryableErr},
			},
			{
				model:    "gemini-2.5-flash",
				streamer: stubStreamer{chunks: []string{"ok"}},
			},
		},
	}

	var chunks []string
	err := streamer.StreamChat(context.Background(), ChatRequest{Message: "hello"}, func(text string) error {
		chunks = append(chunks, text)
		return nil
	})
	if !errors.Is(err, retryableErr) {
		t.Fatalf("StreamChat() error = %v, want %v", err, retryableErr)
	}
	if !reflect.DeepEqual(chunks, []string{"par"}) {
		t.Fatalf("chunks = %v, want [par]", chunks)
	}
}
