package llm

import (
	"context"
	"errors"
	"hash/fnv"
	"math"
	"strings"
)

const fallbackEmbeddingDimension = 768

type FallbackEmbedder struct{}

func NewFallbackEmbedder() *FallbackEmbedder {
	return &FallbackEmbedder{}
}

func (e *FallbackEmbedder) Embed(ctx context.Context, req EmbeddingRequest) ([]float64, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, errors.New("埋め込み対象テキストが空です")
	}

	vector := make([]float64, fallbackEmbeddingDimension)
	hasher := fnv.New64a()
	runes := []rune(text)

	for i, r := range runes {
		hasher.Reset()
		_, _ = hasher.Write([]byte(string(r)))
		index := int((hasher.Sum64() + uint64(i*17)) % fallbackEmbeddingDimension)

		weight := float64((i%7)+1) / 7.0
		if i%2 == 0 {
			vector[index] += weight
		} else {
			vector[index] -= weight
		}
	}

	var norm float64
	for _, value := range vector {
		norm += value * value
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return vector, nil
	}

	for i, value := range vector {
		vector[i] = value / norm
	}

	return vector, nil
}
