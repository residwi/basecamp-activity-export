package basecamp

import (
	"context"
	"encoding/json"
	"net/url"
	"time"
)

// Recording represents the base structure for any Basecamp item (message, todo, todolist, kanban card, etc)
type Recording struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"` // active, archived, trashed
	Type      string    `json:"type"`   // Message, Todo, Todolist, Kanban::Card, etc.
	Title     string    `json:"title"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Bucket    Bucket    `json:"bucket"`
	Creator   Person    `json:"creator"`
	URL       string    `json:"url"`
	AppURL    string    `json:"app_url"`

	// comment metadata
	CommentsCount int    `json:"comments_count"` // number of comments on this recording
	CommentsURL   string `json:"comments_url"`   // API endpoint for comments
}

// Bucket represents a project or container
type Bucket struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Parent represents the parent recording of a comment or the parent container (e.g., Card Table for a KanbanCard)
type Parent struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	URL    string `json:"url"`
	AppURL string `json:"app_url"`
}

// Todolist represents a collection of todos (like a milestone or category)
type Todolist struct {
	Recording
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	Completed      bool   `json:"completed"`
	CompletedRatio string `json:"completed_ratio"` // e.g., "3/10"
	Position       int    `json:"position,omitempty"`
	Parent         Parent `json:"parent"`
}

// Todo represents a to-do item
type Todo struct {
	Recording
	Description string      `json:"description,omitempty"`
	Completed   bool        `json:"completed"`
	Completion  *Completion `json:"completion,omitempty"`
	StartsOn    string      `json:"starts_on,omitempty"`
	DueOn       string      `json:"due_on,omitempty"`
	Assignees   []Person    `json:"assignees,omitempty"`
	Parent      Parent      `json:"parent"`
}

type Completion struct {
	CreatedAt *time.Time `json:"created_at,omitempty"`
	Creator   Person     `json:"creator"`
}

// Message represents a message/discussion
type Message struct {
	Recording
	Subject string `json:"subject"`
}

// KanbanCard represents a card in a KanbanCard Table (Kanban board)
type KanbanCard struct {
	Recording
	Description  string       `json:"description,omitempty"`
	Completed    bool         `json:"completed"`
	DueOn        string       `json:"due_on,omitempty"`
	Assignees    []Person     `json:"assignees,omitempty"`
	CommentCount int          `json:"comment_count"`
	Steps        []KanbanStep `json:"steps,omitempty"`
	Parent       Parent       `json:"parent"` // Card Table column reference
}

// KanbanStep represents a sub-task within a Kanban card
type KanbanStep struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	Title     string    `json:"title"`
	Type      string    `json:"type"` // "Kanban::Step"
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Parent    Parent    `json:"parent"`
	Bucket    Bucket    `json:"bucket"`
	Creator   Person    `json:"creator"`
	URL       string    `json:"url"`
}

// Comment represents a comment on any recording
type Comment struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Creator   Person    `json:"creator"`
	Parent    Parent    `json:"parent"`
	URL       string    `json:"url"`
}

type QuestionAnswer struct {
	Recording
	GroupOn string `json:"group_on"` // Date grouping for daily check-ins (YYYY-MM-DD)
}

func (c *Client) GetTodolists(ctx context.Context, year int) ([]Todolist, error) {
	return GetRecordingsTyped[Todolist](c, ctx, "Todolist", year)
}

func (c *Client) GetTodos(ctx context.Context, year int) ([]Todo, error) {
	return GetRecordingsTyped[Todo](c, ctx, "Todo", year)
}

func (c *Client) GetMessages(ctx context.Context, year int) ([]Message, error) {
	return GetRecordingsTyped[Message](c, ctx, "Message", year)
}

func (c *Client) GetKanbanCards(ctx context.Context, year int) ([]KanbanCard, error) {
	return GetRecordingsTyped[KanbanCard](c, ctx, "Kanban::Card", year)
}

func (c *Client) GetComments(ctx context.Context, year int) ([]Comment, error) {
	return GetRecordingsTyped[Comment](c, ctx, "Comment", year)
}

func (c *Client) GetQuestionAnswers(ctx context.Context, year int) ([]QuestionAnswer, error) {
	return GetRecordingsTyped[QuestionAnswer](c, ctx, "Question::Answer", year)
}

func GetRecordingsTyped[T any](c *Client, ctx context.Context, recordingType string, year int) ([]T, error) {
	query := url.Values{}
	if recordingType != "" {
		query.Set("type", recordingType)
	}
	rawResults, err := c.GetPaginated(ctx, "/projects/recordings.json", query, year)
	if err != nil {
		return nil, err
	}

	items := make([]T, 0, len(rawResults))
	for _, raw := range rawResults {
		var item T
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (t Todo) IsCompletedByUser(userID int64) bool {
	if !t.Completed || t.Completion == nil {
		return false
	}
	return t.Completion.Creator.ID == userID
}

func (t Todo) IsCompletedInYear(year int) bool {
	if !t.Completed || t.Completion == nil || t.Completion.CreatedAt == nil {
		return false
	}
	return t.Completion.CreatedAt.Year() == year
}

func IsAssigned(assignees []Person, userID int64) bool {
	for _, assignee := range assignees {
		if assignee.ID == userID {
			return true
		}
	}
	return false
}

func IsCreatedByUser(creator Person, userID int64) bool {
	return creator.ID == userID
}

func IsInYear(t time.Time, year int) bool {
	return t.Year() == year
}
