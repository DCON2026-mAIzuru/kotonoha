package models

import "time"

type BonsaiProfile struct {
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Personality string    `json:"personality"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UpsertBonsaiProfileInput struct {
	UserID      string `json:"-"`
	Name        string `json:"name"`
	Personality string `json:"personality"`
}
