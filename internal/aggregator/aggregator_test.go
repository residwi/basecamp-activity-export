package aggregator

import (
	"context"
	"testing"
	"time"

	"github.com/residwi/bcreview/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestFilterMessageByUserAndYear(t *testing.T) {
	userID := int64(100)
	year := 2025

	messages := []models.Message{
		{
			Recording: models.Recording{
				ID:        1,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			Recording: models.Recording{
				ID:        2,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC), // Wrong year
			},
		},
		{
			Recording: models.Recording{
				ID:        3,
				Creator:   models.Creator{ID: 200}, // Wrong user
				CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			Recording: models.Recording{
				ID:        4,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			},
		},
	}

	filtered := filterMessageByUserAndYear(messages, userID, year)

	assert.Len(t, filtered, 2, "Should filter 2 messages")
	assert.Equal(t, messages[0].Recording.ID, filtered[0].ID, "First message ID should match")
	assert.Equal(t, messages[3].Recording.ID, filtered[1].ID, "Second message ID should match")
}

func TestFilterMessageByUserAndYear_Empty(t *testing.T) {
	filtered := filterMessageByUserAndYear([]models.Message{}, 100, 2025)

	assert.Len(t, filtered, 0, "Empty input should return empty output")
}

func TestFilterTodosByUserAndYear(t *testing.T) {
	todos := []models.Todo{
		{
			// Created by user
			Recording: models.Recording{
				ID:        1,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			Completed: false,
		},
		{
			// Completed by user
			Recording: models.Recording{
				ID:        2,
				Creator:   models.Creator{ID: 200},
				CreatedAt: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
			},
			Completed: true,
			Completion: &models.Completion{
				Creator: models.Creator{ID: 100},
				CreatedAt: func() *time.Time {
					t := time.Date(2025, 2, 1, 10, 0, 0, 0, time.UTC)
					return &t
				}(),
			},
			Assignees: []models.Person{{ID: 100}},
		},
		{
			// Assigned to user (not created or completed by them)
			Recording: models.Recording{
				ID:        3,
				Creator:   models.Creator{ID: 200},
				CreatedAt: time.Date(2025, 1, 5, 10, 0, 0, 0, time.UTC),
			},
			Completed: false,
			Assignees: []models.Person{{ID: 100}},
		},
		{
			// Wrong year (should be excluded)
			Recording: models.Recording{
				ID:        4,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC),
			},
			Completed: false,
		},
		{
			// Not related to user
			Recording: models.Recording{
				ID:        5,
				Creator:   models.Creator{ID: 200},
				CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			Completed: false,
			Assignees: []models.Person{{ID: 300}},
		},
	}

	userID := int64(100)
	year := 2025
	created, completed, assigned := filterTodosByUserAndYear(todos, userID, year)

	assert.Len(t, created, 1, "Should have 1 created todo")
	assert.Equal(t, todos[0].Recording.ID, created[0].ID, "Created todo ID should match")

	assert.Len(t, completed, 1, "Should have 1 completed todo")
	assert.Equal(t, todos[1].Recording.ID, completed[0].ID, "Completed todo ID should match")

	assert.Len(t, assigned, 2, "Should have 2 assigned todo")
	assert.Equal(t, todos[1].Recording.ID, assigned[0].ID, "Assigned todo ID should match")
	assert.Equal(t, todos[2].Recording.ID, assigned[1].ID, "Assigned todo ID should match")
}

func TestFilterTodolistsByUserAndYear(t *testing.T) {
	todolists := []models.Todolist{
		{
			Recording: models.Recording{
				ID:        1,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			Recording: models.Recording{
				ID:        2,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC), // Wrong year
			},
		},
		{
			Recording: models.Recording{
				ID:        3,
				Creator:   models.Creator{ID: 200}, // Wrong user
				CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
	}

	userID := int64(100)
	year := 2025
	filtered := filterTodolistsByUserAndYear(todolists, userID, year)

	assert.Len(t, filtered, 1, "Should filter 1 todolist")
	assert.Equal(t, todolists[0].Recording.ID, filtered[0].ID, "Todolist ID should match")
}

func TestFilterCardsByUserAndYear(t *testing.T) {
	cards := []models.Card{
		{
			// Created by user
			Recording: models.Recording{
				ID:        1,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			// Assigned to user (not created by them)
			Recording: models.Recording{
				ID:        2,
				Creator:   models.Creator{ID: 200},
				CreatedAt: time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC),
			},
			Assignees: []models.Person{{ID: 100}},
		},
		{
			// Wrong year
			Recording: models.Recording{
				ID:        3,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			// Not related to user
			Recording: models.Recording{
				ID:        4,
				Creator:   models.Creator{ID: 200},
				CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			Assignees: []models.Person{{ID: 300}},
		},
	}

	userID := int64(100)
	year := 2025
	created, assigned := filterCardsByUserAndYear(cards, userID, year)

	assert.Len(t, created, 1, "Should have 1 created card")
	assert.Equal(t, cards[0].Recording.ID, created[0].ID, "Created card ID should match")

	assert.Len(t, assigned, 1, "Should have 1 assigned card")
	assert.Equal(t, cards[1].Recording.ID, assigned[0].ID, "Assigned card ID should match")
}

func TestFilterCommentsByUserAndYear(t *testing.T) {
	comments := []models.Comment{
		{
			ID:        1,
			Creator:   models.Creator{ID: 100},
			CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:        2,
			Creator:   models.Creator{ID: 100},
			CreatedAt: time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC), // Wrong year
		},
		{
			ID:        3,
			Creator:   models.Creator{ID: 200}, // Wrong user
			CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		},
	}

	userID := int64(100)
	year := 2025
	filtered := filterCommentsByUserAndYear(comments, userID, year)

	assert.Len(t, filtered, 1, "Should filter 1 comment")
	assert.Equal(t, comments[0].ID, filtered[0].ID, "Comment ID should match")
}

func TestFilterQuestionAnswersByUserAndYear(t *testing.T) {
	answers := []models.QuestionAnswer{
		{
			Recording: models.Recording{
				ID:        1,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
		{
			Recording: models.Recording{
				ID:        2,
				Creator:   models.Creator{ID: 100},
				CreatedAt: time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC), // Wrong year
			},
		},
		{
			Recording: models.Recording{
				ID:        3,
				Creator:   models.Creator{ID: 200}, // Wrong user
				CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
			},
		},
	}

	userID := int64(100)
	year := 2025
	filtered := filterQuestionAnswersByUserAndYear(answers, userID, year)

	assert.Len(t, filtered, 1, "Should filter 1 answer")
	assert.Equal(t, answers[0].Recording.ID, filtered[0].ID, "Answer ID should match")
}

func TestFilterTodosByUserAndYear_EdgeCases(t *testing.T) {
	userID := int64(100)
	year := 2025

	t.Run("nil assignees", func(t *testing.T) {
		todos := []models.Todo{
			{
				Recording: models.Recording{
					ID:        1,
					Creator:   models.Creator{ID: 200},
					CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
				},
				Assignees: nil,
			},
		}

		created, completed, assigned := filterTodosByUserAndYear(todos, userID, year)

		assert.NotNil(t, created)
		assert.NotNil(t, completed)
		assert.NotNil(t, assigned)
	})

	t.Run("nil completion", func(t *testing.T) {
		todos := []models.Todo{
			{
				Recording: models.Recording{
					ID:        1,
					Creator:   models.Creator{ID: 200},
					CreatedAt: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
				},
				Completed:  true,
				Completion: nil,
			},
		}

		_, completed, _ := filterTodosByUserAndYear(todos, userID, year)

		assert.Len(t, completed, 0, "Should not panic and return empty completed slice")
	})

	t.Run("year boundary - last second of year", func(t *testing.T) {
		todos := []models.Todo{
			{
				Recording: models.Recording{
					ID:        1,
					Creator:   models.Creator{ID: 100},
					CreatedAt: time.Date(2025, 12, 31, 23, 59, 59, 999999999, time.UTC),
				},
			},
		}

		created, _, _ := filterTodosByUserAndYear(todos, userID, year)

		assert.Len(t, created, 1, "Should match last second of year")
	})

	t.Run("year boundary - first second of next year", func(t *testing.T) {
		todos := []models.Todo{
			{
				Recording: models.Recording{
					ID:        1,
					Creator:   models.Creator{ID: 100},
					CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		}

		created, _, _ := filterTodosByUserAndYear(todos, userID, year)

		assert.Len(t, created, 0, "Should not match first second of next year")
	})
}

func TestNewAggregator(t *testing.T) {
	// This test requires a mock client, but demonstrates the basic structure
	// In a real test, we'd use dependency injection with a mock client
	ctx := context.Background()
	_ = ctx // Placeholder for future test expansion

	// Test that NewAggregator doesn't panic with nil client
	// Note: In production code, you'd want to validate this
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewAggregator panicked: %v", r)
		}
	}()

	agg := NewAggregator(nil)
	assert.NotNil(t, agg, "Expected non-nil Aggregator")
	assert.NotNil(t, agg.tracker, "Expected non-nil Tracker")
	assert.NotNil(t, agg.display, "Expected non-nil Aggregator")
}
