package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVoicevoxServiceSynthesize(t *testing.T) {
	t.Parallel()

	const wantAudio = "RIFFvoicevox-wav"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio_query":
			if r.Method != http.MethodPost {
				t.Fatalf("audio_query method = %s, want POST", r.Method)
			}
			if got := r.URL.Query().Get("speaker"); got != "13" {
				t.Fatalf("audio_query speaker = %s, want 13", got)
			}
			if got := r.URL.Query().Get("text"); got != "こんにちは" {
				t.Fatalf("audio_query text = %s, want こんにちは", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"accent_phrases": []any{}})
		case "/synthesis":
			if r.Method != http.MethodPost {
				t.Fatalf("synthesis method = %s, want POST", r.Method)
			}
			if got := r.URL.Query().Get("speaker"); got != "13" {
				t.Fatalf("synthesis speaker = %s, want 13", got)
			}
			var requestBody map[string]any
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Fatalf("decode synthesis body: %v", err)
			}
			if got := requestBody["speedScale"]; got != 1.12 {
				t.Fatalf("speedScale = %#v, want 1.12", got)
			}
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte(wantAudio))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewVoicevoxService(server.URL, "13", "", "", "1.12", time.Second)
	audio, err := service.Synthesize(context.Background(), "こんにちは")
	if err != nil {
		t.Fatalf("Synthesize returned error: %v", err)
	}
	if got := string(audio); got != wantAudio {
		t.Fatalf("audio = %q, want %q", got, wantAudio)
	}
}

func TestVoicevoxServiceSynthesizeRejectsEmptyText(t *testing.T) {
	t.Parallel()

	service := NewVoicevoxService("http://127.0.0.1:50021", "13", "", "", "1.12", time.Second)
	if _, err := service.Synthesize(context.Background(), " \n\t "); err == nil {
		t.Fatal("Synthesize returned nil error for empty text")
	}
}
