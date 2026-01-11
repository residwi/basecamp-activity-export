package models

import "time"

// Person represents a Basecamp user
type Person struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	EmailAddress string    `json:"email_address"`
	Admin        bool      `json:"admin"`
	Owner        bool      `json:"owner"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PersonProfile represents the current user's profile
type PersonProfile struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	EmailAddress string    `json:"email_address"`
	Company      Company   `json:"company"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Company represents the Basecamp company/organization
type Company struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
