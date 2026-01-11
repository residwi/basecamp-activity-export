package models

import (
	"encoding/json"
	"time"
)

// Recording represents any Basecamp item (message, todo, todolist, kanban card, etc.)
type Recording struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"` // active, archived, trashed
	Type      string    `json:"type"`   // Message, Todo, Todolist, Kanban::Card, etc.
	Title     string    `json:"title"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Bucket    Bucket    `json:"bucket"`
	Creator   Creator   `json:"creator"`
	URL       string    `json:"url"`
	AppURL    string    `json:"app_url"`

	// Comment metadata for optimization
	CommentsCount int    `json:"comments_count"` // Number of comments on this recording
	CommentsURL   string `json:"comments_url"`   // API endpoint for comments
}

// Bucket represents a project or container
type Bucket struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Creator represents the person who created an item
type Creator struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	EmailAddress string `json:"email_address,omitempty"`
}

// Date represents a date without time component (YYYY-MM-DD format)
type Date struct {
	time.Time
}

// UnmarshalJSON handles both null and date string values
func (d *Date) UnmarshalJSON(data []byte) error {
	// Handle null
	if string(data) == "null" {
		return nil
	}

	// Remove quotes
	var dateStr string
	if err := json.Unmarshal(data, &dateStr); err != nil {
		return err
	}

	// Parse date-only format
	parsed, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return err
	}

	d.Time = parsed
	return nil
}

// MarshalJSON outputs date in YYYY-MM-DD format or null
func (d Date) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Time.Format("2006-01-02"))
}

// Completion represents when and by whom a todo was completed
type Completion struct {
	CreatedAt *time.Time `json:"created_at,omitempty"`
	Creator   Creator    `json:"creator"`
}

// Todo represents a to-do item
type Todo struct {
	Recording
	Description           string      `json:"description,omitempty"`
	Completed             bool        `json:"completed"`
	Completion            *Completion `json:"completion,omitempty"`
	StartsOn              *Date       `json:"starts_on,omitempty"`
	DueOn                 *Date       `json:"due_on,omitempty"`
	Assignees             []Person    `json:"assignees,omitempty"`
	CompletionSubscribers []Person    `json:"completion_subscribers,omitempty"`
	Parent                Parent      `json:"parent"`
}

// Message represents a message/discussion
type Message struct {
	Recording
	Subject string `json:"subject"`
}

// Todolist represents a collection of todos (like a milestone or category)
type Todolist struct {
	Recording
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	Completed      bool   `json:"completed"`
	CompletedRatio string `json:"completed_ratio"` // e.g., "3/10"
	Position       int    `json:"position,omitempty"`
	TodosURL       string `json:"todos_url"`
	GroupsURL      string `json:"groups_url,omitempty"`
	AppTodosURL    string `json:"app_todos_url,omitempty"`
	Parent         Parent `json:"parent"`
}

// Step represents a sub-task within a Kanban card
type Step struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	Title     string    `json:"title"`
	Type      string    `json:"type"` // "Kanban::Step"
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Parent    Parent    `json:"parent"`
	Bucket    Bucket    `json:"bucket"`
	Creator   Creator   `json:"creator"`
	URL       string    `json:"url"`
	AppURL    string    `json:"app_url"`
}

// Card represents a card in a Card Table (Kanban board)
type Card struct {
	Recording
	Description           string   `json:"description,omitempty"`
	Completed             bool     `json:"completed"`
	DueOn                 *Date    `json:"due_on,omitempty"`
	Assignees             []Person `json:"assignees,omitempty"`
	CompletionSubscribers []Person `json:"completion_subscribers,omitempty"`
	CompletionURL         string   `json:"completion_url,omitempty"`
	CommentCount          int      `json:"comment_count"`
	Steps                 []Step   `json:"steps,omitempty"`
	Parent                Parent   `json:"parent"` // Card Table column reference
}

// QuestionAnswer represents an answer to a daily check-in question
type QuestionAnswer struct {
	Recording
	GroupOn string `json:"group_on"` // Date grouping for daily check-ins (YYYY-MM-DD)
}
