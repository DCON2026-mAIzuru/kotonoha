package llm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testLocalMetadata() map[string]any {
	metadata := map[string]any{
		"model_id": "intfloat/multilingual-e5-small", "revision": strings.Repeat("a", 40),
		"dimension": float64(384), "normalized": true, "query_prefix": "query: ", "passage_prefix": "passage: ",
		"max_tokens": float64(512), "pooling": "attention-mask-mean", "normalization": "l2-float32",
		"padding_dimension": float64(3072), "padding": "right-zero", "runtime": "test-runtime",
	}
	canonical, _ := json.Marshal(metadata)
	metadata["embedding_space"] = fmt.Sprintf("e5-small-v1-%x", sha256.Sum256(canonical))
	metadata["service"] = "kotonoha-local-embeddings"
	return metadata
}

func TestLocalEmbedderUsesTaskKindsAndPadsNormalizedVectors(t *testing.T) {
	metadata := testLocalMetadata()
	var kinds []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/metadata" {
			_ = json.NewEncoder(w).Encode(metadata)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/embed" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}
		var input map[string]string
		_ = json.NewDecoder(r.Body).Decode(&input)
		kinds = append(kinds, input["kind"])
		if input["text"] != "水やり" {
			t.Errorf("text must be passed without a duplicate prefix: %q", input["text"])
		}
		vector := make([]float64, 384)
		vector[0], vector[1] = .6, .8
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": vector, "dimension": 384, "embedding_space": metadata["embedding_space"]})
	}))
	defer server.Close()
	embedder, err := NewLocalEmbedder(context.Background(), server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range []EmbeddingTask{EmbeddingTaskRetrievalQuery, EmbeddingTaskRetrievalDocument} {
		vector, err := embedder.Embed(context.Background(), EmbeddingRequest{Text: " 水やり ", TaskType: task})
		if err != nil {
			t.Fatal(err)
		}
		if len(vector) != 3072 || vector[0] != .6 || vector[1] != .8 {
			t.Fatal("real vector was changed")
		}
		for _, value := range vector[384:] {
			if value != 0 {
				t.Fatal("padding must be zero")
			}
		}
	}
	if strings.Join(kinds, ",") != "query,passage" {
		t.Fatalf("wrong task kinds: %v", kinds)
	}
	if embedder.Space() != metadata["embedding_space"] {
		t.Fatal("metadata space not retained")
	}
}

func TestLocalEmbedderRejectsMismatchedOrInvalidResponses(t *testing.T) {
	for _, test := range []struct {
		name           string
		dimension      int
		length         int
		norm           float64
		differentSpace bool
	}{
		{"dimension", 383, 384, 1, false}, {"length", 384, 383, 1, false},
		{"zero", 384, 384, 0, false}, {"unnormalized", 384, 384, 2, false},
		{"changed-model", 384, 384, 1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			metadata := testLocalMetadata()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/metadata" {
					_ = json.NewEncoder(w).Encode(metadata)
					return
				}
				vector := make([]float64, test.length)
				vector[0] = test.norm
				space := metadata["embedding_space"]
				if test.differentSpace {
					space = "another-model"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"embedding": vector, "dimension": test.dimension, "embedding_space": space})
			}))
			defer server.Close()
			embedder, err := NewLocalEmbedder(context.Background(), server.URL, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = embedder.Embed(context.Background(), EmbeddingRequest{Text: "test", TaskType: EmbeddingTaskRetrievalQuery}); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}

func TestLocalEmbeddingMetadataDefinesIsolatedSpace(t *testing.T) {
	metadata := testLocalMetadata()
	if _, err := validateLocalEmbeddingMetadata(metadata); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"revision", "query_prefix", "passage_prefix", "runtime", "embedding_space"} {
		t.Run(field, func(t *testing.T) {
			changed := testLocalMetadata()
			changed[field] = "changed"
			if _, err := validateLocalEmbeddingMetadata(changed); err == nil {
				t.Fatal("metadata change accepted in old space")
			}
		})
	}
}

func TestLocalEmbedderRejectsRemoteURLsAndRedirects(t *testing.T) {
	for _, address := range []string{"https://example.com", "http://localhost:50022", "http://192.168.1.1", "http://user:secret@127.0.0.1", "http://127.0.0.1/path", "http://127.0.0.1?key=secret"} {
		if _, err := NewLocalEmbedder(context.Background(), address, time.Second); err == nil {
			t.Errorf("unsafe URL accepted: %s", address)
		}
	}
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { targetCalls++ }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer server.Close()
	_, err := NewLocalEmbedder(context.Background(), server.URL, time.Second)
	if err == nil || targetCalls != 0 {
		t.Fatalf("redirect followed: %v / %d", err, targetCalls)
	}
}

func TestLocalEmbedderDoesNotExposeServiceErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "private conversation or secret", http.StatusInternalServerError)
	}))
	defer server.Close()
	_, err := NewLocalEmbedder(context.Background(), server.URL, time.Second)
	if err == nil || strings.Contains(err.Error(), "private conversation") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}
