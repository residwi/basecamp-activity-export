package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserActivity_Summary(t *testing.T) {
	activity := UserActivity{
		User: Person{ID: 123, Name: "Test User"},
		Year: 2025,
		MessagesCreated: []Message{
			{Recording: Recording{ID: 1}},
			{Recording: Recording{ID: 2}},
			{Recording: Recording{ID: 3}},
		},
		TodosCreated: []Todo{
			{Recording: Recording{ID: 4}},
			{Recording: Recording{ID: 5}},
		},
		TodosCompleted: []Todo{
			{Recording: Recording{ID: 6}},
		},
		TodosAssigned: []Todo{
			{Recording: Recording{ID: 7}},
			{Recording: Recording{ID: 8}},
		},
		TodolistsCreated: []Todolist{
			{Recording: Recording{ID: 9}},
		},
		CardsCreated: []Card{
			{Recording: Recording{ID: 10}},
			{Recording: Recording{ID: 11}},
			{Recording: Recording{ID: 12}},
		},
		CardsAssigned: []Card{
			{Recording: Recording{ID: 13}},
		},
		CommentsWritten: []Comment{
			{ID: 14},
			{ID: 15},
			{ID: 16},
			{ID: 17},
		},
		QuestionAnswers: []QuestionAnswer{
			{Recording: Recording{ID: 18}},
			{Recording: Recording{ID: 19}},
		},
	}

	summary := activity.Summary()

	tests := []struct {
		name     string
		got      int
		expected int
	}{
		{"TotalMessages", summary.TotalMessages, 3},
		{"TotalTodos", summary.TotalTodos, 2},
		{"CompletedTodos", summary.CompletedTodos, 1},
		{"AssignedTodos", summary.AssignedTodos, 2},
		{"TotalTodolists", summary.TotalTodolists, 1},
		{"TotalCardsCreated", summary.TotalCardsCreated, 3},
		{"TotalCardsAssigned", summary.TotalCardsAssigned, 1},
		{"TotalComments", summary.TotalComments, 4},
		{"TotalQuestionAnswers", summary.TotalQuestionAnswers, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.got, "%s should match", tt.name)
		})
	}
}

func TestUserActivity_Summary_Empty(t *testing.T) {
	activity := UserActivity{
		User: Person{ID: 123, Name: "Test User"},
		Year: 2025,
	}

	summary := activity.Summary()

	assert.Equal(t, 0, summary.TotalMessages, "TotalMessages should be 0")
	assert.Equal(t, 0, summary.TotalTodos, "TotalTodos should be 0")
	assert.Equal(t, 0, summary.CompletedTodos, "CompletedTodos should be 0")
	assert.Equal(t, 0, summary.AssignedTodos, "AssignedTodos should be 0")
	assert.Equal(t, 0, summary.TotalTodolists, "TotalTodolists should be 0")
	assert.Equal(t, 0, summary.TotalCardsCreated, "TotalCardsCreated should be 0")
	assert.Equal(t, 0, summary.TotalCardsAssigned, "TotalCardsAssigned should be 0")
	assert.Equal(t, 0, summary.TotalComments, "TotalComments should be 0")
	assert.Equal(t, 0, summary.TotalQuestionAnswers, "TotalQuestionAnswers should be 0")
}

func TestUserActivity_Summary_NilSlices(t *testing.T) {
	activity := UserActivity{
		User:             Person{ID: 123, Name: "Test User"},
		Year:             2025,
		MessagesCreated:  nil,
		TodosCreated:     nil,
		TodosCompleted:   nil,
		TodosAssigned:    nil,
		TodolistsCreated: nil,
		CardsCreated:     nil,
		CardsAssigned:    nil,
		CommentsWritten:  nil,
		QuestionAnswers:  nil,
	}

	summary := activity.Summary()

	assert.Equal(t, 0, summary.TotalMessages, "TotalMessages should be 0 for nil slice")
	assert.Equal(t, 0, summary.TotalTodos, "TotalTodos should be 0 for nil slice")
}

func TestUserActivity_InitialState(t *testing.T) {
	activity := UserActivity{
		User: Person{ID: 456, Name: "Another User"},
		Year: 2024,
	}

	assert.Equal(t, int64(456), activity.User.ID, "User ID should match")
	assert.Equal(t, "Another User", activity.User.Name, "User Name should match")
	assert.Equal(t, 2024, activity.Year, "Year should match")
}
