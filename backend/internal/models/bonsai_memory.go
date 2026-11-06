package models

import "time"

type BonsaiMemory struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id,omitempty"`
	Summary          string    `json:"summary"`
	SourceMessageIDs []string  `json:"source_message_ids,omitempty"`
	Similarity       float64   `json:"similarity,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type CreateBonsaiMemoryInput struct {
	UserID           string
	Summary          string
	SourceMessageIDs []string
	Embedding        []float64
	EmbeddingSpace   string
}

type ConversationMemoryPair struct {
	UserID             string
	UserMessageID      string
	AssistantMessageID string
	UserText           string
	AssistantText      string
}
