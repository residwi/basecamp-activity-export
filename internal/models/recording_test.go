package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDate_UnmarshalJSON_Valid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "valid date",
			input:    `"2025-01-15"`,
			expected: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "leap year date",
			input:    `"2024-02-29"`,
			expected: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "end of year",
			input:    `"2025-12-31"`,
			expected: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Date
			err := json.Unmarshal([]byte(tt.input), &d)

			require.NoError(t, err, "Unmarshal failed")
			assert.True(t, d.Time.Equal(tt.expected), "Date should match expected value")
		})
	}
}

func TestDate_UnmarshalJSON_Null(t *testing.T) {
	var d Date
	err := json.Unmarshal([]byte("null"), &d)

	require.NoError(t, err, "Unmarshal null failed")
	assert.True(t, d.Time.IsZero(), "Expected zero time for null")
}

func TestDate_UnmarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid format", `"2025/01/15"`},
		{"invalid month", `"2025-13-15"`},
		{"invalid day", `"2025-01-32"`},
		{"non-leap year", `"2023-02-29"`},
		{"invalid json", `invalid`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Date
			err := json.Unmarshal([]byte(tt.input), &d)

			assert.Error(t, err, "Expected error for invalid input")
		})
	}
}

func TestDate_MarshalJSON_Valid(t *testing.T) {
	d := Date{Time: time.Date(2025, 6, 15, 12, 30, 45, 0, time.UTC)}

	data, err := json.Marshal(d)
	require.NoError(t, err, "Marshal failed")

	expected := `"2025-06-15"`
	assert.Equal(t, expected, string(data), "Marshaled date should match expected format")
}

func TestDate_MarshalJSON_Zero(t *testing.T) {
	d := Date{Time: time.Time{}}

	data, err := json.Marshal(d)
	require.NoError(t, err, "Marshal failed")

	assert.Equal(t, "null", string(data), "Zero time should marshal to null")
}

func TestDate_RoundTrip(t *testing.T) {
	original := Date{Time: time.Date(2025, 3, 20, 0, 0, 0, 0, time.UTC)}

	data, err := json.Marshal(original)
	require.NoError(t, err, "Marshal failed")

	var decoded Date
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err, "Unmarshal failed")

	assert.Equal(t, original.Time.Year(), decoded.Time.Year(), "Year should match")
	assert.Equal(t, original.Time.Month(), decoded.Time.Month(), "Month should match")
	assert.Equal(t, original.Time.Day(), decoded.Time.Day(), "Day should match")
}

func TestTodo_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": 123,
		"type": "Todo",
		"title": "Test Todo",
		"description": "Test description",
		"completed": false,
		"created_at": "2025-01-15T10:30:00Z",
		"updated_at": "2025-01-15T11:00:00Z",
		"creator": {
			"id": 456,
			"name": "Test User"
		},
		"url": "https://3.basecampapi.com/123/todos/123.json",
		"app_url": "https://3.basecamp.com/123/todos/123",
		"bucket": {
			"id": 789,
			"name": "Test Project",
			"type": "Project"
		},
		"due_on": "2025-02-01",
		"parent": {
			"id": 999,
			"type": "Todolist"
		}
	}`

	var todo Todo
	err := json.Unmarshal([]byte(jsonData), &todo)

	require.NoError(t, err, "Unmarshal failed")

	assert.Equal(t, int64(123), todo.ID, "ID should match")
	assert.Equal(t, "Todo", todo.Type, "Type should match")
	assert.Equal(t, "Test Todo", todo.Title, "Title should match")
	assert.Equal(t, "Test description", todo.Description, "Description should match")
	assert.False(t, todo.Completed, "Completed should be false")
	assert.Equal(t, int64(456), todo.Creator.ID, "Creator ID should match")
	assert.Equal(t, "Test User", todo.Creator.Name, "Creator Name should match")
	assert.Equal(t, int64(789), todo.Bucket.ID, "Bucket ID should match")

	require.NotNil(t, todo.DueOn, "DueOn should be set")
	expectedDue := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	assert.True(t, todo.DueOn.Time.Equal(expectedDue), "DueOn should match expected date")
}

func TestMessage_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": 456,
		"type": "Message",
		"title": "Test Title",
		"subject": "Test Subject",
		"content": "<p>Test content</p>",
		"created_at": "2025-01-15T10:30:00Z",
		"updated_at": "2025-01-15T11:00:00Z",
		"creator": {
			"id": 789,
			"name": "Message Creator"
		},
		"url": "https://3.basecampapi.com/123/messages/456.json",
		"app_url": "https://3.basecamp.com/123/messages/456",
		"bucket": {
			"id": 111,
			"name": "Test Project",
			"type": "Project"
		}
	}`

	var message Message
	err := json.Unmarshal([]byte(jsonData), &message)

	require.NoError(t, err, "Unmarshal failed")

	assert.Equal(t, int64(456), message.ID, "ID should match")
	assert.Equal(t, "Test Subject", message.Subject, "Subject should match")
	assert.Equal(t, "<p>Test content</p>", message.Content, "Content should match")
}

func TestCard_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": 789,
		"type": "Kanban::Card",
		"title": "Test Card",
		"description": "Card description",
		"completed": false,
		"created_at": "2025-01-15T10:30:00Z",
		"updated_at": "2025-01-15T11:00:00Z",
		"creator": {
			"id": 100,
			"name": "Card Creator"
		},
		"assignees": [
			{"id": 200, "name": "Assignee 1"},
			{"id": 201, "name": "Assignee 2"}
		],
		"due_on": "2025-02-15",
		"comment_count": 5,
		"url": "https://3.basecampapi.com/123/cards/789.json",
		"app_url": "https://3.basecamp.com/123/cards/789",
		"bucket": {
			"id": 222,
			"name": "Test Project",
			"type": "Project"
		},
		"parent": {
			"id": 333,
			"type": "Kanban::Column"
		}
	}`

	var card Card
	err := json.Unmarshal([]byte(jsonData), &card)

	require.NoError(t, err, "Unmarshal failed")

	assert.Equal(t, int64(789), card.ID, "ID should match")
	assert.Equal(t, "Card description", card.Description, "Description should match")
	assert.Len(t, card.Assignees, 2, "Should have 2 assignees")
	assert.Equal(t, int64(200), card.Assignees[0].ID, "First assignee ID should match")
	assert.Equal(t, 5, card.CommentCount, "CommentCount should match")
}

func TestCompletion_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"created_at": "2025-01-15T14:30:00Z",
		"creator": {
			"id": 500,
			"name": "Completer"
		}
	}`

	var completion Completion
	err := json.Unmarshal([]byte(jsonData), &completion)

	require.NoError(t, err, "Unmarshal failed")

	require.NotNil(t, completion.CreatedAt, "CreatedAt should be set")
	assert.Equal(t, int64(500), completion.Creator.ID, "Creator ID should match")
}

func TestTodo_WithCompletion(t *testing.T) {
	jsonData := `{
		"id": 999,
		"type": "Todo",
		"title": "Completed Todo",
		"completed": true,
		"completion": {
			"created_at": "2025-01-16T10:00:00Z",
			"creator": {
				"id": 600,
				"name": "Completer User"
			}
		},
		"created_at": "2025-01-15T10:30:00Z",
		"updated_at": "2025-01-16T10:00:00Z",
		"creator": {
			"id": 700,
			"name": "Creator User"
		},
		"url": "https://3.basecampapi.com/123/todos/999.json",
		"app_url": "https://3.basecamp.com/123/todos/999",
		"bucket": {
			"id": 800,
			"name": "Test Project",
			"type": "Project"
		},
		"parent": {
			"id": 900,
			"type": "Todolist"
		}
	}`

	var todo Todo
	err := json.Unmarshal([]byte(jsonData), &todo)

	require.NoError(t, err, "Unmarshal failed")

	assert.True(t, todo.Completed, "Todo should be completed")
	require.NotNil(t, todo.Completion, "Completion should be set")
	assert.Equal(t, int64(600), todo.Completion.Creator.ID, "Completion creator ID should match")
}

func TestTodolist_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": 1111,
		"type": "Todolist",
		"title": "Q1 Goals",
		"name": "Q1 Goals",
		"description": "First quarter goals",
		"completed": false,
		"completed_ratio": "3/10",
		"created_at": "2025-01-01T10:00:00Z",
		"updated_at": "2025-01-15T10:00:00Z",
		"creator": {
			"id": 1000,
			"name": "List Creator"
		},
		"todos_url": "https://3.basecampapi.com/123/todolists/1111/todos.json",
		"url": "https://3.basecampapi.com/123/todolists/1111.json",
		"app_url": "https://3.basecamp.com/123/todolists/1111",
		"bucket": {
			"id": 2000,
			"name": "Test Project",
			"type": "Project"
		},
		"parent": {
			"id": 3000,
			"type": "TodoSet"
		}
	}`

	var todolist Todolist
	err := json.Unmarshal([]byte(jsonData), &todolist)

	require.NoError(t, err, "Unmarshal failed")

	assert.Equal(t, int64(1111), todolist.ID, "ID should match")
	assert.Equal(t, "Q1 Goals", todolist.Name, "Name should match")
	assert.Equal(t, "3/10", todolist.CompletedRatio, "CompletedRatio should match")
}

func TestQuestionAnswer_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"id": 5555,
		"type": "Question::Answer",
		"title": "Daily Check-in",
		"content": "Making good progress today",
		"group_on": "2025-01-15",
		"created_at": "2025-01-15T09:00:00Z",
		"updated_at": "2025-01-15T09:00:00Z",
		"creator": {
			"id": 6000,
			"name": "Daily User"
		},
		"url": "https://3.basecampapi.com/123/answers/5555.json",
		"app_url": "https://3.basecamp.com/123/answers/5555",
		"bucket": {
			"id": 7000,
			"name": "Test Project",
			"type": "Project"
		}
	}`

	var answer QuestionAnswer
	err := json.Unmarshal([]byte(jsonData), &answer)

	require.NoError(t, err, "Unmarshal failed")

	assert.Equal(t, int64(5555), answer.ID, "ID should match")
	assert.Equal(t, "2025-01-15", answer.GroupOn, "GroupOn should match")
}
