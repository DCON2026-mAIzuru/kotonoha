package service

import (
	"context"
	"errors"

	"github.com/apple/kotonoha/backend/internal/llm"
	"github.com/apple/kotonoha/backend/internal/models"
)

type memoryBackfillStore interface {
	memoryWriter
	ListUnembeddedConversationPairs(context.Context, string, string, int, bool) ([]models.ConversationMemoryPair, error)
}

type MemoryBackfillResult struct {
	Candidates int
	Processed  int
}

// 過去の成功ペアを通常会話と同じ形式で取り込み、重複はDBの一意制約で防ぐ。
func BackfillConversationMemories(ctx context.Context, store memoryBackfillStore, embedder llm.Embedder, userID string, limit int, apply, legacyAdjacent bool) (MemoryBackfillResult, error) {
	var result MemoryBackfillResult
	if embedder == nil || embedder.Space() == "" {
		return result, errors.New("取り込み用の埋め込みモデルが設定されていません")
	}
	pairs, err := store.ListUnembeddedConversationPairs(ctx, userID, embedder.Space(), limit, legacyAdjacent)
	if err != nil {
		return result, err
	}
	result.Candidates = len(pairs)
	if !apply {
		return result, nil
	}
	for _, pair := range pairs {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := archiveConversationMemory(ctx, store, embedder, pair); err != nil {
			return result, err
		}
		result.Processed++
	}
	return result, nil
}
