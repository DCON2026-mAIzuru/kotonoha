package llm

import (
	"context"
	"strings"
)

type EmbeddingTask string

const (
	EmbeddingTaskRetrievalQuery    EmbeddingTask = "RETRIEVAL_QUERY"
	EmbeddingTaskRetrievalDocument EmbeddingTask = "RETRIEVAL_DOCUMENT"
)

type EmbeddingRequest struct {
	Text     string
	TaskType EmbeddingTask
	Title    string
}

type Embedder interface {
	Embed(ctx context.Context, req EmbeddingRequest) ([]float64, error)
}

func NewEmbedder(geminiAPIKey string, geminiModel string) Embedder {
	if strings.TrimSpace(geminiAPIKey) == "" {
		return NewFallbackEmbedder()
	}

	model := strings.TrimSpace(geminiModel)
	if model == "" {
		model = "text-embedding-004"
	}

	return NewGeminiEmbedder(geminiAPIKey, model, nil)
}
