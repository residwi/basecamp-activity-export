package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceType(t *testing.T) {
	t.Run("constants have expected values", func(t *testing.T) {
		tests := []struct {
			rt       ResourceType
			expected string
		}{
			{Messages, "messages"},
			{Todos, "todos"},
			{Todolists, "todolists"},
			{KanbanCards, "kanban_cards"},
			{Comments, "comments"},
			{QuestionAnswers, "question_answers"},
		}

		for _, tt := range tests {
			assert.Equal(t, ResourceType(tt.expected), tt.rt)
		}
	})
}

func TestResourceTypes(t *testing.T) {
	t.Run("slice contains all resource types in order", func(t *testing.T) {
		expected := []ResourceType{
			Messages,
			Todos,
			Todolists,
			KanbanCards,
			Comments,
			QuestionAnswers,
		}

		assert.Equal(t, expected, ResourceTypes)
	})

	t.Run("slice has correct length", func(t *testing.T) {
		assert.Len(t, ResourceTypes, 6)
	})
}
