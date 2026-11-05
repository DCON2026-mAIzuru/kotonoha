package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestOpenAICompatibleStreamer(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer server-only-secret" {
			t.Error("missing server authorization")
		}
		var body struct {
			Model     string `json:"model"`
			Stream    bool   `json:"stream"`
			MaxTokens int    `json:"max_tokens"`
			Messages  []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body.Model != "configured-model-id" || !body.Stream || body.MaxTokens != 256 || len(body.Messages) != 2 {
			t.Errorf("unexpected payload: %+v", body)
		}
		if len(body.Messages) == 2 && (body.Messages[0].Role != "system" || body.Messages[0].Content != "盆栽として話す" || body.Messages[1].Content != "こんにちは") {
			t.Error("system prompt or user message was not preserved")
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		fmt.Fprint(w, ": keepalive\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"reasoning_content\":\"private reasoning\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"おはよう\"}}]}\n\n")
		w.(http.Flusher).Flush()
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ございます\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"completion_tokens\":2}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	streamer, err := NewOpenAICompatibleStreamer(server.URL+"/v1/", "server-only-secret", "configured-model-id", 256, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var chunks []string
	err = streamer.StreamChat(context.Background(), ChatRequest{Message: "こんにちは", SystemPrompt: "盆栽として話す"}, func(text string) error {
		chunks = append(chunks, text)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(chunks, []string{"おはよう", "ございます"}) {
		t.Fatalf("chunks = %q", chunks)
	}
}

func TestChatCompletionSSERejectsFailures(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"empty":     "data: [DONE]\n\n",
		"truncated": "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n",
		"malformed": "data: not json\n\n",
		"upstream":  "data: {\"error\":{\"message\":\"private upstream detail\"}}\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			err := scanChatCompletionSSE(strings.NewReader(input), func(string) error { return nil })
			if err == nil {
				t.Fatal("expected failure, got success")
			}
			if strings.Contains(err.Error(), "private upstream detail") {
				t.Fatal("upstream details were exposed")
			}
		})
	}
	callbackErr := errors.New("client disconnected")
	err := scanChatCompletionSSE(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"), func(string) error { return callbackErr })
	if !errors.Is(err, callbackErr) {
		t.Fatalf("callback error = %v", err)
	}
}

func TestOpenAICompatibleStreamerDoesNotExposeErrorBodiesOrFollowRedirects(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusUnauthorized, http.StatusFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				w.Header().Set("Location", "/private-redirect")
				w.WriteHeader(status)
				fmt.Fprint(w, "secret in upstream response")
			}))
			defer server.Close()
			streamer, err := NewOpenAICompatibleStreamer(server.URL+"/v1", "token", "model", 1, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			err = streamer.StreamChat(context.Background(), ChatRequest{Message: "hello"}, func(string) error { return nil })
			if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), server.URL) {
				t.Fatalf("unexpected error: %v", err)
			}
			if requests != 1 {
				t.Fatalf("followed redirect: %d requests", requests)
			}
		})
	}
}

func TestOpenAICompatibleStreamerHonorsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	streamer, err := NewOpenAICompatibleStreamer("http://127.0.0.1:1/v1", "secret", "model", 1, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = streamer.StreamChat(ctx, ChatRequest{Message: "hello"}, func(string) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestOpenAICompatibleStreamerValidatesConfiguration(t *testing.T) {
	t.Parallel()
	for _, baseURL := range []string{"", "ftp://host/v1", "http://remote.example/v1", "https://user:secret@host/v1", "https://host/v1?key=secret", "https://host/v1#fragment"} {
		if _, err := NewOpenAICompatibleStreamer(baseURL, "", "model", 1, time.Second); err == nil {
			t.Errorf("accepted invalid base URL: %s", baseURL)
		}
	}
	if _, err := NewOpenAICompatibleStreamer("https://host/v1", "", "", 1, time.Second); err == nil {
		t.Error("accepted missing model")
	}
}
