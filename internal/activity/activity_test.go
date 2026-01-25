package activity

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/basecamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	user := createTestPerson()
	year := 2025

	ua := New(user, year)

	require.NotNil(t, ua)
	assert.Equal(t, user, ua.user)
	assert.Equal(t, year, ua.year)
	assert.Nil(t, ua.messagesCreated)
	assert.Nil(t, ua.todosCreated)
	assert.Nil(t, ua.kanbanCardsCreated)
}

func TestUserActivity_User(t *testing.T) {
	user := basecamp.Person{ID: 123, Name: "Test User", EmailAddress: "test@example.com"}
	ua := New(user, 2025)

	result := ua.User()

	assert.Equal(t, user, result)
}

func TestUserActivity_Year(t *testing.T) {
	ua := New(basecamp.Person{}, 2025)

	result := ua.Year()

	assert.Equal(t, 2025, result)
}

func TestUserActivity_MessagesCreated(t *testing.T) {
	t.Run("returns nil for empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		result := ua.MessagesCreated()

		assert.Nil(t, result)
	})

	t.Run("returns copy of messages", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		messages := []basecamp.Message{
			createTestMessage(1),
		}
		ua.AddMessages(messages)

		result := ua.MessagesCreated()

		assert.Equal(t, messages, result)
		// verify it's a copy by modifying the result
		result[0].Title = "Modified"
		assert.Equal(t, "Test Message", ua.messagesCreated[0].Title)
	})
}

func TestUserActivity_TodosCreated(t *testing.T) {
	t.Run("returns nil for empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		result := ua.TodosCreated()

		assert.Nil(t, result)
	})

	t.Run("returns copy of todos", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		todos := []basecamp.Todo{
			{Recording: basecamp.Recording{ID: 1, Title: "Todo"}},
		}
		ua.AddTodos(todos, nil, nil)

		result := ua.TodosCreated()

		assert.Equal(t, todos, result)
	})
}

func TestUserActivity_TodosCompleted(t *testing.T) {
	t.Run("returns nil for empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		result := ua.TodosCompleted()

		assert.Nil(t, result)
	})

	t.Run("returns copy of completed todos", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		completed := []basecamp.Todo{
			createTestTodo(1),
		}
		ua.AddTodos(nil, completed, nil)

		result := ua.TodosCompleted()

		assert.Equal(t, completed, result)
	})
}

func TestUserActivity_TodosAssigned(t *testing.T) {
	t.Run("returns nil for empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		result := ua.TodosAssigned()

		assert.Nil(t, result)
	})

	t.Run("returns copy of assigned todos", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		assigned := []basecamp.Todo{
			createTestTodo(1),
		}
		ua.AddTodos(nil, nil, assigned)

		result := ua.TodosAssigned()

		assert.Equal(t, assigned, result)
	})
}

func TestUserActivity_TodolistsCreated(t *testing.T) {
	t.Run("returns nil for empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		result := ua.TodolistsCreated()

		assert.Nil(t, result)
	})

	t.Run("returns copy of todolists", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		lists := []basecamp.Todolist{
			createTestTodolist(1),
		}
		ua.AddTodolists(lists)

		result := ua.TodolistsCreated()

		assert.Equal(t, lists, result)
	})
}

func TestUserActivity_KanbanCardsCreated(t *testing.T) {
	t.Run("returns nil for empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		result := ua.KanbanCardsCreated()

		assert.Nil(t, result)
	})

	t.Run("returns copy of cards", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		cards := []basecamp.KanbanCard{
			createTestKanbanCard(1),
		}
		ua.AddKanbanCards(cards, nil)

		result := ua.KanbanCardsCreated()

		assert.Equal(t, cards, result)
	})
}

func TestUserActivity_KanbanCardsAssigned(t *testing.T) {
	t.Run("returns nil for empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		result := ua.KanbanCardsAssigned()

		assert.Nil(t, result)
	})

	t.Run("returns copy of assigned cards", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		cards := []basecamp.KanbanCard{
			createTestKanbanCard(1),
		}
		ua.AddKanbanCards(nil, cards)

		result := ua.KanbanCardsAssigned()

		assert.Equal(t, cards, result)
	})
}

func TestUserActivity_CommentsWritten(t *testing.T) {
	t.Run("returns nil for empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		result := ua.CommentsWritten()

		assert.Nil(t, result)
	})

	t.Run("returns copy of comments", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		comments := []basecamp.Comment{
			createTestComment(1),
		}
		ua.AddComments(comments)

		result := ua.CommentsWritten()

		assert.Equal(t, comments, result)
	})
}

func TestUserActivity_QuestionAnswers(t *testing.T) {
	t.Run("returns nil for empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		result := ua.QuestionAnswers()

		assert.Nil(t, result)
	})

	t.Run("returns copy of answers", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		answers := []basecamp.QuestionAnswer{
			createTestQuestionAnswer(1),
		}
		ua.AddQuestionAnswers(answers)

		result := ua.QuestionAnswers()

		assert.Equal(t, answers, result)
	})
}

func TestUserActivity_AddMessages(t *testing.T) {
	t.Run("skips empty slice", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		ua.AddMessages([]basecamp.Message{})

		assert.Nil(t, ua.messagesCreated)
	})

	t.Run("skips nil slice", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		ua.AddMessages(nil)

		assert.Nil(t, ua.messagesCreated)
	})

	t.Run("appends messages", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		msg1 := []basecamp.Message{createTestMessage(1)}
		msg2 := []basecamp.Message{createTestMessage(2)}

		ua.AddMessages(msg1)
		ua.AddMessages(msg2)

		assert.Len(t, ua.messagesCreated, 2)
	})
}

func TestUserActivity_AddTodos(t *testing.T) {
	t.Run("skips all empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		ua.AddTodos(nil, nil, nil)
		ua.AddTodos([]basecamp.Todo{}, []basecamp.Todo{}, []basecamp.Todo{})

		assert.Nil(t, ua.todosCreated)
		assert.Nil(t, ua.todosCompleted)
		assert.Nil(t, ua.todosAssigned)
	})

	t.Run("appends to all categories", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		created := []basecamp.Todo{createTestTodo(1)}
		completed := []basecamp.Todo{createTestTodo(2)}
		assigned := []basecamp.Todo{createTestTodo(3)}

		ua.AddTodos(created, completed, assigned)

		assert.Len(t, ua.todosCreated, 1)
		assert.Len(t, ua.todosCompleted, 1)
		assert.Len(t, ua.todosAssigned, 1)
	})

	t.Run("handles partial data", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		ua.AddTodos([]basecamp.Todo{createTestTodo(1)}, nil, nil)

		assert.Len(t, ua.todosCreated, 1)
		assert.Nil(t, ua.todosCompleted)
		assert.Nil(t, ua.todosAssigned)
	})
}

func TestUserActivity_AddTodolists(t *testing.T) {
	t.Run("skips empty slice", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		ua.AddTodolists([]basecamp.Todolist{})

		assert.Nil(t, ua.todolistsCreated)
	})

	t.Run("appends todolists", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		lists := []basecamp.Todolist{createTestTodolist(1), createTestTodolist(2)}

		ua.AddTodolists(lists)

		assert.Len(t, ua.todolistsCreated, 2)
	})
}

func TestUserActivity_AddKanbanCards(t *testing.T) {
	t.Run("skips all empty", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		ua.AddKanbanCards(nil, nil)

		assert.Nil(t, ua.kanbanCardsCreated)
		assert.Nil(t, ua.kanbanCardsAssigned)
	})

	t.Run("appends to both categories", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		created := []basecamp.KanbanCard{createTestKanbanCard(1)}
		assigned := []basecamp.KanbanCard{createTestKanbanCard(2)}

		ua.AddKanbanCards(created, assigned)

		assert.Len(t, ua.kanbanCardsCreated, 1)
		assert.Len(t, ua.kanbanCardsAssigned, 1)
	})
}

func TestUserActivity_AddComments(t *testing.T) {
	t.Run("skips empty slice", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		ua.AddComments([]basecamp.Comment{})

		assert.Nil(t, ua.commentsWritten)
	})

	t.Run("appends comments", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		comments := []basecamp.Comment{{ID: 1}, {ID: 2}}

		ua.AddComments(comments)

		assert.Len(t, ua.commentsWritten, 2)
	})
}

func TestUserActivity_AddQuestionAnswers(t *testing.T) {
	t.Run("skips empty slice", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)

		ua.AddQuestionAnswers([]basecamp.QuestionAnswer{})

		assert.Nil(t, ua.questionAnswers)
	})

	t.Run("appends answers", func(t *testing.T) {
		ua := New(basecamp.Person{}, 2025)
		answers := []basecamp.QuestionAnswer{createTestQuestionAnswer(1), createTestQuestionAnswer(2)}

		ua.AddQuestionAnswers(answers)

		assert.Len(t, ua.questionAnswers, 2)
	})
}

func TestCloneSlice(t *testing.T) {
	t.Run("returns nil for empty slice", func(t *testing.T) {
		result := cloneSlice([]int{})

		assert.Nil(t, result)
	})

	t.Run("returns nil for nil slice", func(t *testing.T) {
		var src []string

		result := cloneSlice(src)

		assert.Nil(t, result)
	})

	t.Run("creates independent copy", func(t *testing.T) {
		src := []int{1, 2, 3}

		result := cloneSlice(src)

		assert.Equal(t, src, result)
		// modify result and verify src is unchanged
		result[0] = 999
		assert.Equal(t, 1, src[0])
	})
}

func TestUserActivity_Concurrency(t *testing.T) {
	t.Run("concurrent writes are safe", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ua := New(basecamp.Person{ID: 1}, 2025)
			var wg sync.WaitGroup

			// simulate concurrent writes from multiple goroutines
			for i := range 10 {
				wg.Add(6)

				go func(id int64) {
					defer wg.Done()
					ua.AddMessages([]basecamp.Message{createTestMessage(id)})
				}(int64(i))

				go func(id int64) {
					defer wg.Done()
					ua.AddTodos(
						[]basecamp.Todo{createTestTodo(id)},
						[]basecamp.Todo{createTestTodo(id + 100)},
						[]basecamp.Todo{createTestTodo(id + 200)},
					)
				}(int64(i))

				go func(id int64) {
					defer wg.Done()
					ua.AddTodolists([]basecamp.Todolist{createTestTodolist(id)})
				}(int64(i))

				go func(id int64) {
					defer wg.Done()
					ua.AddKanbanCards(
						[]basecamp.KanbanCard{createTestKanbanCard(id)},
						[]basecamp.KanbanCard{createTestKanbanCard(id + 100)},
					)
				}(int64(i))

				go func(id int64) {
					defer wg.Done()
					ua.AddComments([]basecamp.Comment{{ID: id}})
				}(int64(i))

				go func(id int64) {
					defer wg.Done()
					ua.AddQuestionAnswers([]basecamp.QuestionAnswer{createTestQuestionAnswer(id)})
				}(int64(i))
			}

			wg.Wait()

			assert.Len(t, ua.MessagesCreated(), 10)
			assert.Len(t, ua.TodosCreated(), 10)
			assert.Len(t, ua.TodosCompleted(), 10)
			assert.Len(t, ua.TodosAssigned(), 10)
			assert.Len(t, ua.TodolistsCreated(), 10)
			assert.Len(t, ua.KanbanCardsCreated(), 10)
			assert.Len(t, ua.KanbanCardsAssigned(), 10)
			assert.Len(t, ua.CommentsWritten(), 10)
			assert.Len(t, ua.QuestionAnswers(), 10)
		})
	})

	t.Run("concurrent reads and writes are safe", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ua := New(basecamp.Person{ID: 1}, 2025)
			ua.AddMessages([]basecamp.Message{createTestMessage(1)})

			var wg sync.WaitGroup

			// Writers
			for i := range 5 {
				wg.Add(1)
				go func(id int64) {
					defer wg.Done()
					ua.AddMessages([]basecamp.Message{createTestMessage(id)})
				}(int64(i + 10))
			}

			// Readers
			for range 5 {
				wg.Go(func() {
					_ = ua.MessagesCreated()
				})
			}

			wg.Wait()
		})
	})
}

func createTestPerson() basecamp.Person {
	return basecamp.Person{
		ID:           123,
		Name:         "John Doe",
		EmailAddress: "info@email.com",
	}
}

func createTestMessage(id int64) basecamp.Message {
	return basecamp.Message{
		Recording: basecamp.Recording{
			ID:        id,
			Type:      "Message",
			Title:     "Test Message",
			CreatedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Creator: basecamp.Person{
				ID:   123,
				Name: "John Doe",
			},
		},
		Subject: "Test Subject",
	}
}

func createTestTodo(id int64) basecamp.Todo {
	return basecamp.Todo{
		Recording: basecamp.Recording{
			ID:        id,
			Type:      "Todo",
			Title:     "Test Todo",
			CreatedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Creator: basecamp.Person{
				ID:   123,
				Name: "John Doe",
			},
		},
		Description: "Test Description",
		Completed:   false,
	}
}

func createTestTodolist(id int64) basecamp.Todolist {
	return basecamp.Todolist{
		Recording: basecamp.Recording{
			ID:        id,
			Type:      "Todolist",
			Title:     "Test Todolist",
			CreatedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Creator: basecamp.Person{
				ID:   123,
				Name: "John Doe",
			},
		},
		Name: "Test Todolist",
	}
}

func createTestKanbanCard(id int64) basecamp.KanbanCard {
	return basecamp.KanbanCard{
		Recording: basecamp.Recording{
			ID:        id,
			Type:      "Kanban::Card",
			Title:     "Test Card",
			CreatedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Creator: basecamp.Person{
				ID:   123,
				Name: "John Doe",
			},
		},
		Description: "Test Card Description",
	}
}

func createTestComment(id int64) basecamp.Comment {
	return basecamp.Comment{
		ID:      id,
		Content: "Test Comment",
		Creator: basecamp.Person{
			ID:   123,
			Name: "John Doe",
		},
		CreatedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
	}
}

func createTestQuestionAnswer(id int64) basecamp.QuestionAnswer {
	return basecamp.QuestionAnswer{
		Recording: basecamp.Recording{
			ID:        id,
			Type:      "Question::Answer",
			Title:     "Test Answer",
			CreatedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
			Creator: basecamp.Person{
				ID:   123,
				Name: "John Doe",
			},
		},
		GroupOn: "2025-01-01",
	}
}
