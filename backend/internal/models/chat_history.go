package models

import "time"

type ChatHistoryEntry struct {
	ID             string         `json:"id"`
	UserID         string         `json:"user_id"`
	Role           string         `json:"role"`
	Content        string         `json:"content"`
	SensorSnapshot map[string]any `json:"sensor_snapshot,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

type CreateChatHistoryEntryInput struct {
	UserID         string         `json:"-"`
	Role           string         `json:"role"`
	Content        string         `json:"content"`
	SensorSnapshot map[string]any `json:"sensor_snapshot,omitempty"`
}
