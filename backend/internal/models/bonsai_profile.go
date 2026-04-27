package models

import "time"

type BonsaiProfile struct {
	UserID        string    `json:"user_id"`
	Name          string    `json:"name"`
	Personality   string    `json:"personality"`
	Backstory     string    `json:"backstory"`
	ToneMode      string    `json:"tone_mode"`
	BondScore     int       `json:"bond_score"`
	BondUpdatedAt time.Time `json:"bond_updated_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UpsertBonsaiProfileInput struct {
	UserID      string `json:"-"`
	Name        string `json:"name"`
	Personality string `json:"personality"`
	Backstory   string `json:"backstory"`
	ToneMode    string `json:"tone_mode"`
}
