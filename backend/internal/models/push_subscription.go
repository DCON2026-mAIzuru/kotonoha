package models

import "time"

type PushSubscription struct {
	ID           string         `json:"id"`
	UserID       string         `json:"user_id"`
	Endpoint     string         `json:"endpoint"`
	Subscription map[string]any `json:"subscription"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type UpsertPushSubscriptionInput struct {
	UserID       string         `json:"-"`
	Endpoint     string         `json:"endpoint"`
	Subscription map[string]any `json:"subscription"`
}
