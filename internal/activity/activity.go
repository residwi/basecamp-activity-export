package activity

import (
	"sync"

	"github.com/residwi/basecamp-activity-export/internal/basecamp"
)

// UserActivity represents the rich domain aggregate for user activity data
// It is concurrency-safe for both reads and writes
type UserActivity struct {
	mu                  sync.RWMutex
	user                basecamp.Person
	year                int
	messagesCreated     []basecamp.Message
	todosCreated        []basecamp.Todo
	todosCompleted      []basecamp.Todo
	todosAssigned       []basecamp.Todo
	todolistsCreated    []basecamp.Todolist
	kanbanCardsCreated  []basecamp.KanbanCard
	kanbanCardsAssigned []basecamp.KanbanCard
	commentsWritten     []basecamp.Comment
	questionAnswers     []basecamp.QuestionAnswer
}

// New creates a new UserActivity instance.
func New(user basecamp.Person, year int) *UserActivity {
	return &UserActivity{
		user: user,
		year: year,
	}
}

func (ua *UserActivity) User() basecamp.Person {
	return ua.user
}

func (ua *UserActivity) Year() int {
	return ua.year
}

func (ua *UserActivity) TodosAssigned() []basecamp.Todo {
	ua.mu.RLock()
	defer ua.mu.RUnlock()

	return cloneSlice(ua.todosAssigned)
}

func (ua *UserActivity) TodolistsCreated() []basecamp.Todolist {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	return cloneSlice(ua.todolistsCreated)
}

func (ua *UserActivity) TodosCreated() []basecamp.Todo {
	ua.mu.RLock()
	defer ua.mu.RUnlock()

	return cloneSlice(ua.todosCreated)
}

func (ua *UserActivity) TodosCompleted() []basecamp.Todo {
	ua.mu.RLock()
	defer ua.mu.RUnlock()

	return cloneSlice(ua.todosCompleted)
}

func (ua *UserActivity) MessagesCreated() []basecamp.Message {
	ua.mu.RLock()
	defer ua.mu.RUnlock()

	return cloneSlice(ua.messagesCreated)
}

func (ua *UserActivity) KanbanCardsCreated() []basecamp.KanbanCard {
	ua.mu.RLock()
	defer ua.mu.RUnlock()

	return cloneSlice(ua.kanbanCardsCreated)
}

func (ua *UserActivity) KanbanCardsAssigned() []basecamp.KanbanCard {
	ua.mu.RLock()
	defer ua.mu.RUnlock()

	return cloneSlice(ua.kanbanCardsAssigned)
}

func (ua *UserActivity) CommentsWritten() []basecamp.Comment {
	ua.mu.RLock()
	defer ua.mu.RUnlock()

	return cloneSlice(ua.commentsWritten)
}

func (ua *UserActivity) QuestionAnswers() []basecamp.QuestionAnswer {
	ua.mu.RLock()
	defer ua.mu.RUnlock()

	return cloneSlice(ua.questionAnswers)
}

func (ua *UserActivity) AddMessages(messages []basecamp.Message) {
	if len(messages) == 0 {
		return
	}

	ua.mu.Lock()
	defer ua.mu.Unlock()

	ua.messagesCreated = append(ua.messagesCreated, messages...)
}

func (ua *UserActivity) AddTodos(created, completed, assigned []basecamp.Todo) {
	if len(created) == 0 && len(completed) == 0 && len(assigned) == 0 {
		return
	}

	ua.mu.Lock()
	defer ua.mu.Unlock()

	ua.todosCreated = append(ua.todosCreated, created...)
	ua.todosCompleted = append(ua.todosCompleted, completed...)
	ua.todosAssigned = append(ua.todosAssigned, assigned...)
}

func (ua *UserActivity) AddTodolists(todolists []basecamp.Todolist) {
	if len(todolists) == 0 {
		return
	}

	ua.mu.Lock()
	defer ua.mu.Unlock()

	ua.todolistsCreated = append(ua.todolistsCreated, todolists...)
}

func (ua *UserActivity) AddKanbanCards(created, assigned []basecamp.KanbanCard) {
	if len(created) == 0 && len(assigned) == 0 {
		return
	}

	ua.mu.Lock()
	defer ua.mu.Unlock()

	ua.kanbanCardsCreated = append(ua.kanbanCardsCreated, created...)
	ua.kanbanCardsAssigned = append(ua.kanbanCardsAssigned, assigned...)
}

func (ua *UserActivity) AddComments(comments []basecamp.Comment) {
	if len(comments) == 0 {
		return
	}

	ua.mu.Lock()
	defer ua.mu.Unlock()

	ua.commentsWritten = append(ua.commentsWritten, comments...)
}

func (ua *UserActivity) AddQuestionAnswers(qas []basecamp.QuestionAnswer) {
	if len(qas) == 0 {
		return
	}

	ua.mu.Lock()
	defer ua.mu.Unlock()

	ua.questionAnswers = append(ua.questionAnswers, qas...)
}

// cloneSlice creates a shallow copy of a slice.
func cloneSlice[T any](src []T) []T {
	if len(src) == 0 {
		return nil
	}
	dst := make([]T, len(src))
	copy(dst, src)
	return dst
}
