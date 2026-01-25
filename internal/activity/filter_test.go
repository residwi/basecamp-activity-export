package activity

import (
	"testing"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/basecamp"
	"github.com/stretchr/testify/assert"
)

const (
	targetUserID int64 = 100
	otherUserID  int64 = 200
	targetYear   int   = 2025
)

func TestFilterTodolists(t *testing.T) {
	tests := []struct {
		name      string
		input     []basecamp.Todolist
		wantCount int
		wantIDs   []int64
	}{
		{
			name:      "nil slice returns empty",
			input:     nil,
			wantCount: 0,
		},
		{
			name:      "empty slice returns empty",
			input:     []basecamp.Todolist{},
			wantCount: 0,
		},
		{
			name: "skips when year does not match",
			input: []basecamp.Todolist{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(2024)}},
			},
			wantCount: 0,
		},
		{
			name: "skips when user does not match",
			input: []basecamp.Todolist{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(otherUserID), CreatedAt: dateIn(targetYear)}},
			},
			wantCount: 0,
		},
		{
			name: "includes when both year and user match",
			input: []basecamp.Todolist{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)}},
			},
			wantCount: 1,
			wantIDs:   []int64{1},
		},
		{
			name: "filters mixed data correctly",
			input: []basecamp.Todolist{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)}},
				{Recording: basecamp.Recording{ID: 2, Creator: creator(otherUserID), CreatedAt: dateIn(targetYear)}},
				{Recording: basecamp.Recording{ID: 3, Creator: creator(targetUserID), CreatedAt: dateIn(2024)}},
				{Recording: basecamp.Recording{ID: 4, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)}},
			},
			wantCount: 2,
			wantIDs:   []int64{1, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterTodolists(tt.input, targetUserID, targetYear)
			assert.Len(t, result, tt.wantCount)

			for i, wantID := range tt.wantIDs {
				assert.Equal(t, wantID, result[i].ID)
			}
		})
	}
}

func TestFilterTodos(t *testing.T) {
	tests := []struct {
		name          string
		input         []basecamp.Todo
		wantCreated   int
		wantCompleted int
		wantAssigned  int
	}{
		{
			name:          "nil slice returns empty slices",
			input:         nil,
			wantCreated:   0,
			wantCompleted: 0,
			wantAssigned:  0,
		},
		{
			name:          "empty slice returns empty slices",
			input:         []basecamp.Todo{},
			wantCreated:   0,
			wantCompleted: 0,
			wantAssigned:  0,
		},
		{
			name: "skips when year does not match",
			input: []basecamp.Todo{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(targetUserID),
						CreatedAt: dateIn(2024),
					},
				},
			},
			wantCreated:   0,
			wantCompleted: 0,
			wantAssigned:  0,
		},
		{
			name: "adds to created when user is creator",
			input: []basecamp.Todo{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(targetUserID),
						CreatedAt: dateIn(targetYear),
					},
				},
			},
			wantCreated:   1,
			wantCompleted: 0,
			wantAssigned:  0,
		},
		{
			name: "adds to assigned when user is assignee but not creator",
			input: []basecamp.Todo{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(otherUserID),
						CreatedAt: dateIn(targetYear),
					},
					Assignees: assignees(targetUserID),
				},
			},
			wantCreated:   0,
			wantCompleted: 0,
			wantAssigned:  1,
		},
		{
			name: "skips when user is neither creator nor assignee",
			input: []basecamp.Todo{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(otherUserID),
						CreatedAt: dateIn(targetYear),
					},
					Assignees: assignees(otherUserID),
				},
			},
			wantCreated:   0,
			wantCompleted: 0,
			wantAssigned:  0,
		},
		{
			name: "adds to completed when assigned user completes in year",
			input: []basecamp.Todo{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(otherUserID),
						CreatedAt: dateIn(targetYear),
					},
					Completed:  true,
					Assignees:  assignees(targetUserID),
					Completion: completion(targetUserID, targetYear),
				},
			},
			wantCreated:   0,
			wantCompleted: 1,
			wantAssigned:  1,
		},
		{
			name: "adds to both created and completed when creator is also assignee who completes",
			input: []basecamp.Todo{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(targetUserID),
						CreatedAt: dateIn(targetYear),
					},
					Completed:  true,
					Assignees:  assignees(targetUserID),
					Completion: completion(targetUserID, targetYear),
				},
			},
			wantCreated:   1,
			wantCompleted: 1,
			wantAssigned:  0,
		},
		{
			name: "does not add to completed when completed by different user",
			input: []basecamp.Todo{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(otherUserID),
						CreatedAt: dateIn(targetYear),
					},
					Assignees:  assignees(targetUserID),
					Completion: completion(otherUserID, targetYear),
				},
			},
			wantCreated:   0,
			wantCompleted: 0,
			wantAssigned:  1,
		},
		{
			name: "does not add to completed when completed in different year",
			input: []basecamp.Todo{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(otherUserID),
						CreatedAt: dateIn(targetYear),
					},
					Assignees:  assignees(targetUserID),
					Completion: completion(targetUserID, 2023),
				},
			},
			wantCreated:   0,
			wantCompleted: 0,
			wantAssigned:  1,
		},
		{
			name: "does not add to completed when user is not assignee",
			input: []basecamp.Todo{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(targetUserID),
						CreatedAt: dateIn(targetYear),
					},
					Assignees:  assignees(otherUserID),
					Completion: completion(targetUserID, targetYear),
				},
			},
			wantCreated:   1,
			wantCompleted: 0,
			wantAssigned:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, completed, assigned := FilterTodos(tt.input, targetUserID, targetYear)

			assert.Len(t, created, tt.wantCreated, "created count mismatch")
			assert.Len(t, completed, tt.wantCompleted, "completed count mismatch")
			assert.Len(t, assigned, tt.wantAssigned, "assigned count mismatch")
		})
	}
}

func TestFilterMessages(t *testing.T) {
	tests := []struct {
		name      string
		input     []basecamp.Message
		wantCount int
		wantIDs   []int64
	}{
		{
			name:      "nil slice returns empty",
			input:     nil,
			wantCount: 0,
		},
		{
			name:      "empty slice returns empty",
			input:     []basecamp.Message{},
			wantCount: 0,
		},
		{
			name: "skips when year does not match",
			input: []basecamp.Message{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(2024)}},
			},
			wantCount: 0,
		},
		{
			name: "skips when user does not match",
			input: []basecamp.Message{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(otherUserID), CreatedAt: dateIn(targetYear)}},
			},
			wantCount: 0,
		},
		{
			name: "includes when both year and user match",
			input: []basecamp.Message{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)}},
			},
			wantCount: 1,
			wantIDs:   []int64{1},
		},
		{
			name: "filters mixed data correctly",
			input: []basecamp.Message{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)}},
				{Recording: basecamp.Recording{ID: 2, Creator: creator(otherUserID), CreatedAt: dateIn(targetYear)}},
				{Recording: basecamp.Recording{ID: 3, Creator: creator(targetUserID), CreatedAt: dateIn(2024)}},
			},
			wantCount: 1,
			wantIDs:   []int64{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterMessages(tt.input, targetUserID, targetYear)
			assert.Len(t, result, tt.wantCount)

			for i, wantID := range tt.wantIDs {
				assert.Equal(t, wantID, result[i].ID)
			}
		})
	}
}

func TestFilterKanbanCards(t *testing.T) {
	tests := []struct {
		name         string
		input        []basecamp.KanbanCard
		wantCreated  int
		wantAssigned int
	}{
		{
			name:         "nil slice returns empty slices",
			input:        nil,
			wantCreated:  0,
			wantAssigned: 0,
		},
		{
			name:         "empty slice returns empty slices",
			input:        []basecamp.KanbanCard{},
			wantCreated:  0,
			wantAssigned: 0,
		},
		{
			name: "skips when year does not match",
			input: []basecamp.KanbanCard{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(2024)}},
			},
			wantCreated:  0,
			wantAssigned: 0,
		},
		{
			name: "adds to created when user is creator",
			input: []basecamp.KanbanCard{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)}},
			},
			wantCreated:  1,
			wantAssigned: 0,
		},
		{
			name: "adds to assigned when user is assignee but not creator",
			input: []basecamp.KanbanCard{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(otherUserID),
						CreatedAt: dateIn(targetYear),
					},
					Assignees: assignees(targetUserID),
				},
			},
			wantCreated:  0,
			wantAssigned: 1,
		},
		{
			name: "skips when user is neither creator nor assignee",
			input: []basecamp.KanbanCard{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(otherUserID),
						CreatedAt: dateIn(targetYear),
					},
					Assignees: assignees(otherUserID),
				},
			},
			wantCreated:  0,
			wantAssigned: 0,
		},
		{
			name: "adds to created only when user is both creator and assignee",
			input: []basecamp.KanbanCard{
				{
					Recording: basecamp.Recording{
						ID:        1,
						Creator:   creator(targetUserID),
						CreatedAt: dateIn(targetYear),
					},
					Assignees: assignees(targetUserID),
				},
			},
			wantCreated:  1,
			wantAssigned: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, assigned := FilterKanbanCards(tt.input, targetUserID, targetYear)

			assert.Len(t, created, tt.wantCreated, "created count mismatch")
			assert.Len(t, assigned, tt.wantAssigned, "assigned count mismatch")
		})
	}
}

func TestFilterComments(t *testing.T) {
	tests := []struct {
		name      string
		input     []basecamp.Comment
		wantCount int
		wantIDs   []int64
	}{
		{
			name:      "nil slice returns empty",
			input:     nil,
			wantCount: 0,
		},
		{
			name:      "empty slice returns empty",
			input:     []basecamp.Comment{},
			wantCount: 0,
		},
		{
			name: "skips when year does not match",
			input: []basecamp.Comment{
				{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(2024)},
			},
			wantCount: 0,
		},
		{
			name: "skips when user does not match",
			input: []basecamp.Comment{
				{ID: 1, Creator: creator(otherUserID), CreatedAt: dateIn(targetYear)},
			},
			wantCount: 0,
		},
		{
			name: "includes when both year and user match",
			input: []basecamp.Comment{
				{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)},
			},
			wantCount: 1,
			wantIDs:   []int64{1},
		},
		{
			name: "filters mixed data correctly",
			input: []basecamp.Comment{
				{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)},
				{ID: 2, Creator: creator(otherUserID), CreatedAt: dateIn(targetYear)},
				{ID: 3, Creator: creator(targetUserID), CreatedAt: dateIn(2024)},
				{ID: 4, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)},
			},
			wantCount: 2,
			wantIDs:   []int64{1, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterComments(tt.input, targetUserID, targetYear)
			assert.Len(t, result, tt.wantCount)

			for i, wantID := range tt.wantIDs {
				assert.Equal(t, wantID, result[i].ID)
			}
		})
	}
}

func TestFilterQuestionAnswers(t *testing.T) {
	tests := []struct {
		name      string
		input     []basecamp.QuestionAnswer
		wantCount int
		wantIDs   []int64
	}{
		{
			name:      "nil slice returns empty",
			input:     nil,
			wantCount: 0,
		},
		{
			name:      "empty slice returns empty",
			input:     []basecamp.QuestionAnswer{},
			wantCount: 0,
		},
		{
			name: "skips when year does not match",
			input: []basecamp.QuestionAnswer{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(2024)}},
			},
			wantCount: 0,
		},
		{
			name: "skips when user does not match",
			input: []basecamp.QuestionAnswer{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(otherUserID), CreatedAt: dateIn(targetYear)}},
			},
			wantCount: 0,
		},
		{
			name: "includes when both year and user match",
			input: []basecamp.QuestionAnswer{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)}},
			},
			wantCount: 1,
			wantIDs:   []int64{1},
		},
		{
			name: "filters mixed data correctly",
			input: []basecamp.QuestionAnswer{
				{Recording: basecamp.Recording{ID: 1, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)}},
				{Recording: basecamp.Recording{ID: 2, Creator: creator(otherUserID), CreatedAt: dateIn(targetYear)}},
				{Recording: basecamp.Recording{ID: 3, Creator: creator(targetUserID), CreatedAt: dateIn(2024)}},
				{Recording: basecamp.Recording{ID: 4, Creator: creator(targetUserID), CreatedAt: dateIn(targetYear)}},
			},
			wantCount: 2,
			wantIDs:   []int64{1, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterQuestionAnswers(tt.input, targetUserID, targetYear)
			assert.Len(t, result, tt.wantCount)

			for i, wantID := range tt.wantIDs {
				assert.Equal(t, wantID, result[i].ID)
			}
		})
	}
}

// dateIn generates an RFC3339 timestamp for June 15th of the given year.
func dateIn(year int) time.Time {
	return time.Date(year, 6, 15, 10, 0, 0, 0, time.UTC)
}

func creator(id int64) basecamp.Person {
	return basecamp.Person{ID: id}
}

func assignees(ids ...int64) []basecamp.Person {
	result := make([]basecamp.Person, len(ids))
	for i, id := range ids {
		result[i] = basecamp.Person{ID: id}
	}
	return result
}

func completion(completorID int64, year int) *basecamp.Completion {
	dateIn := dateIn(year)

	return &basecamp.Completion{
		Creator:   creator(completorID),
		CreatedAt: &dateIn,
	}
}
