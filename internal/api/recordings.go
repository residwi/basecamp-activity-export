package api

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/residwi/bcreview/internal/models"
)

// GetRecordings fetches recordings with filters and unmarshals to the specified type
// This generic function eliminates code duplication while preserving type safety
// Year parameter enables early-stop optimization during pagination
func GetRecordingsTyped[T any](c *Client, ctx context.Context, recordingType string, year int) ([]T, error) {
	query := url.Values{}
	if recordingType != "" {
		query.Set("type", recordingType)
	}
	rawResults, err := c.getPaginated(ctx, "/projects/recordings.json", query, year)
	if err != nil {
		return nil, err
	}

	items := make([]T, 0, len(rawResults))
	for _, raw := range rawResults {
		var item T
		if err := json.Unmarshal(raw, &item); err != nil {
			// Skip malformed entries silently - progress tracker handles errors
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

// GetRecordings fetches recordings with filters (legacy compatibility)
// Deprecated: Use GetRecordingsTyped for type-safe operations
func (c *Client) GetRecordings(ctx context.Context, recordingType string, year int) ([]models.Recording, error) {
	return GetRecordingsTyped[models.Recording](c, ctx, recordingType, year)
}

// GetTodos fetches all todos for a specific year
func (c *Client) GetTodos(ctx context.Context, year int) ([]models.Todo, error) {
	return GetRecordingsTyped[models.Todo](c, ctx, "Todo", year)
}

// GetMessages fetches all messages/discussions for a specific year
func (c *Client) GetMessages(ctx context.Context, year int) ([]models.Message, error) {
	return GetRecordingsTyped[models.Message](c, ctx, "Message", year)
}

// GetTodolists fetches all todolists for a specific year
func (c *Client) GetTodolists(ctx context.Context, year int) ([]models.Todolist, error) {
	return GetRecordingsTyped[models.Todolist](c, ctx, "Todolist", year)
}

// GetKanbanCards fetches all Kanban cards (cards in Card Tables) for a specific year
func (c *Client) GetKanbanCards(ctx context.Context, year int) ([]models.Card, error) {
	return GetRecordingsTyped[models.Card](c, ctx, "Kanban::Card", year)
}

// GetComments fetches all comments for a specific year
func (c *Client) GetComments(ctx context.Context, year int) ([]models.Comment, error) {
	return GetRecordingsTyped[models.Comment](c, ctx, "Comment", year)
}

// GetQuestionAnswers fetches all Question::Answer recordings (daily check-ins) for a specific year
func (c *Client) GetQuestionAnswers(ctx context.Context, year int) ([]models.QuestionAnswer, error) {
	return GetRecordingsTyped[models.QuestionAnswer](c, ctx, "Question::Answer", year)
}
