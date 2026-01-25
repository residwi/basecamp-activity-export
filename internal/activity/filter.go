package activity

import (
	"github.com/residwi/basecamp-activity-export/internal/basecamp"
)

// FilterTodolists filters todolists created by the specified user in the given year
func FilterTodolists(todolists []basecamp.Todolist, userID int64, year int) []basecamp.Todolist {
	// preallocate capacity using a conservative estimate to reduce allocations and GC pressure
	// while avoiding unnecessary memory overhead
	filtered := make([]basecamp.Todolist, 0, len(todolists)/10) // assuming around 10% match rate

	for i := range todolists {
		if !basecamp.IsInYear(todolists[i].CreatedAt, year) {
			continue
		}
		if !basecamp.IsCreatedByUser(todolists[i].Creator, userID) {
			continue
		}
		filtered = append(filtered, todolists[i])
	}
	return filtered
}

// FilterTodos categorizes todos into three groups:
//   - created: Todos created by the user in the specified year
//   - completed: Todos assigned to and completed by the user in the specified year
//   - assigned: Todos assigned to the user but not created by them in the specified year
func FilterTodos(todos []basecamp.Todo, userID int64, year int) (created, completed, assigned []basecamp.Todo) {
	// preallocate capacity using a conservative estimate to reduce allocations and GC pressure
	// while avoiding unnecessary memory overhead.
	//
	// assuming around 10% match rate
	created = make([]basecamp.Todo, 0, len(todos)/10)
	completed = make([]basecamp.Todo, 0, len(todos)/10)
	assigned = make([]basecamp.Todo, 0, len(todos)/10)

	for i := range todos {
		if !basecamp.IsInYear(todos[i].CreatedAt, year) {
			continue
		}

		isCreatedByUser := basecamp.IsCreatedByUser(todos[i].Creator, userID)
		isAssignedToUser := basecamp.IsAssigned(todos[i].Assignees, userID)

		if isCreatedByUser {
			created = append(created, todos[i])
		} else if isAssignedToUser {
			assigned = append(assigned, todos[i])
		}

		if isAssignedToUser && todos[i].IsCompletedByUser(userID) && todos[i].IsCompletedInYear(year) {
			completed = append(completed, todos[i])
		}
	}

	return created, completed, assigned
}

// FilterMessages filters messages created by the specified user in the given year
func FilterMessages(messages []basecamp.Message, userID int64, year int) []basecamp.Message {
	filtered := make([]basecamp.Message, 0, len(messages)/10)

	for i := range messages {
		if !basecamp.IsInYear(messages[i].CreatedAt, year) {
			continue
		}
		if !basecamp.IsCreatedByUser(messages[i].Creator, userID) {
			continue
		}
		filtered = append(filtered, messages[i])
	}
	return filtered
}

// FilterKanbanCards categorizes cards into two groups:
// - created: Cards created by the user in the specified year
// - assigned: Cards assigned to the user but not created by them in the specified year
func FilterKanbanCards(kanbanCards []basecamp.KanbanCard, userID int64, year int) (created, assigned []basecamp.KanbanCard) {
	created = make([]basecamp.KanbanCard, 0, len(kanbanCards)/10)
	assigned = make([]basecamp.KanbanCard, 0, len(kanbanCards)/10)

	for i := range kanbanCards {
		if !basecamp.IsInYear(kanbanCards[i].CreatedAt, year) {
			continue
		}

		if basecamp.IsCreatedByUser(kanbanCards[i].Creator, userID) {
			created = append(created, kanbanCards[i])
		} else if basecamp.IsAssigned(kanbanCards[i].Assignees, userID) {
			assigned = append(assigned, kanbanCards[i])
		}

	}

	return created, assigned
}

// FilterComments filters comments created by the specified user in the given year
func FilterComments(comments []basecamp.Comment, userID int64, year int) []basecamp.Comment {
	filtered := make([]basecamp.Comment, 0, len(comments)/10)

	for i := range comments {
		if !basecamp.IsInYear(comments[i].CreatedAt, year) {
			continue
		}
		if !basecamp.IsCreatedByUser(comments[i].Creator, userID) {
			continue
		}
		filtered = append(filtered, comments[i])
	}
	return filtered
}

// FilterQuestionAnswers filters question answers created by the specified user in the given year
func FilterQuestionAnswers(qas []basecamp.QuestionAnswer, userID int64, year int) []basecamp.QuestionAnswer {
	filtered := make([]basecamp.QuestionAnswer, 0, len(qas)/10)

	for i := range qas {
		if !basecamp.IsInYear(qas[i].CreatedAt, year) {
			continue
		}
		if !basecamp.IsCreatedByUser(qas[i].Creator, userID) {
			continue
		}
		filtered = append(filtered, qas[i])
	}
	return filtered
}
