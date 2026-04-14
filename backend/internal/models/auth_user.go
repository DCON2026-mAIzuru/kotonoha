package models

type AuthUser struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
	Role  string `json:"role,omitempty"`
}
