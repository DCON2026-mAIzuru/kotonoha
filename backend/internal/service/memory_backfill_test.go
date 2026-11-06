package service

import (
	"context"
	"errors"
	"testing"

	"github.com/apple/kotonoha/backend/internal/llm"
	"github.com/apple/kotonoha/backend/internal/models"
)

type backfillTestStore struct {
	pairs         []models.ConversationMemoryPair
	saved         []models.CreateBonsaiMemoryInput
	userID, space string
	legacy        bool
}

func (s *backfillTestStore) ListUnembeddedConversationPairs(_ context.Context, userID, space string, limit int, legacy bool) ([]models.ConversationMemoryPair, error) {
	s.userID, s.space, s.legacy = userID, space, legacy
	return s.pairs, nil
}
func (s *backfillTestStore) CreateBonsaiMemory(_ context.Context, input models.CreateBonsaiMemoryInput) error {
	s.saved = append(s.saved, input)
	return nil
}

type backfillTestEmbedder struct {
	calls   int
	fail    bool
	request llm.EmbeddingRequest
}

func (e *backfillTestEmbedder) Space() string { return "test:revision:passage:padding3072" }
func (e *backfillTestEmbedder) Embed(_ context.Context, req llm.EmbeddingRequest) ([]float64, error) {
	e.calls++
	e.request = req
	if e.fail {
		return nil, errors.New("unavailable")
	}
	vector := make([]float64, 3072)
	vector[0] = 1
	return vector, nil
}

func TestMemoryBackfillDryRunAndExplicitLegacyOptIn(t *testing.T) {
	store := &backfillTestStore{pairs: []models.ConversationMemoryPair{{UserID: "owner", UserMessageID: "user-id", AssistantMessageID: "assistant-id", UserText: "水やりした", AssistantText: "ありがとう"}}}
	embedder := &backfillTestEmbedder{}
	result, err := BackfillConversationMemories(context.Background(), store, embedder, "owner", 100, false, false)
	if err != nil || result.Candidates != 1 || result.Processed != 0 || embedder.calls != 0 || len(store.saved) != 0 || store.legacy {
		t.Fatalf("dry run mutated data: %+v / %v", result, err)
	}
	result, err = BackfillConversationMemories(context.Background(), store, embedder, "owner", 100, true, true)
	if err != nil || result.Processed != 1 || !store.legacy {
		t.Fatalf("backfill failed: %+v / %v", result, err)
	}
	if embedder.request.TaskType != llm.EmbeddingTaskRetrievalDocument || embedder.request.Text != "ユーザー: 水やりした\n盆栽: ありがとう" {
		t.Fatalf("wrong retrieval document: %+v", embedder.request)
	}
	memory := store.saved[0]
	if store.userID != "owner" || store.space != embedder.Space() || memory.EmbeddingSpace != embedder.Space() || memory.UserID != "owner" || memory.SourceMessageIDs[0] != "user-id" || memory.SourceMessageIDs[1] != "assistant-id" {
		t.Fatal("backfill lost user, source IDs, or embedding space")
	}
}

func TestMemoryBackfillEmbeddingFailureDoesNotWriteFakeMemory(t *testing.T) {
	store := &backfillTestStore{pairs: []models.ConversationMemoryPair{{UserID: "owner", UserMessageID: "user-id", AssistantMessageID: "assistant-id", UserText: "text", AssistantText: "reply"}}}
	embedder := &backfillTestEmbedder{fail: true}
	result, err := BackfillConversationMemories(context.Background(), store, embedder, "owner", 100, true, false)
	if err == nil || result.Processed != 0 || len(store.saved) != 0 {
		t.Fatal("failed embedding produced memory")
	}
}
