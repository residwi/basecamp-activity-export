package models

import "time"

// Comment represents a comment on any recording
type Comment struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Creator   Creator   `json:"creator"`
	Parent    Parent    `json:"parent"`
	URL       string    `json:"url"`
	AppURL    string    `json:"app_url"`
}

// Parent represents the parent recording of a comment or the parent container (e.g., Card Table for a Card)
type Parent struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	URL    string `json:"url"`
	AppURL string `json:"app_url,omitempty"`
}
