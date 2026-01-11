package models

// UserActivity aggregates all user activity for a given year
type UserActivity struct {
	User             Person
	Year             int
	MessagesCreated  []Message
	TodosCreated     []Todo
	TodosCompleted   []Todo
	TodosAssigned    []Todo     // Todos assigned to user (but not created by them)
	TodolistsCreated []Todolist // Todolists created by user
	CardsCreated     []Card     // Cards created by user
	CardsAssigned    []Card     // Cards assigned to user (but not created by them)
	CommentsWritten  []Comment
	QuestionAnswers  []QuestionAnswer // Daily check-in answers created by user
}

// Summary provides activity statistics
type Summary struct {
	TotalMessages        int
	TotalTodos           int
	CompletedTodos       int
	AssignedTodos        int
	TotalTodolists       int
	TotalCardsCreated    int
	TotalCardsAssigned   int
	TotalComments        int
	TotalQuestionAnswers int
}

// Summary calculates summary statistics from user activity
func (ua *UserActivity) Summary() Summary {
	return Summary{
		TotalMessages:        len(ua.MessagesCreated),
		TotalTodos:           len(ua.TodosCreated),
		CompletedTodos:       len(ua.TodosCompleted),
		AssignedTodos:        len(ua.TodosAssigned),
		TotalTodolists:       len(ua.TodolistsCreated),
		TotalCardsCreated:    len(ua.CardsCreated),
		TotalCardsAssigned:   len(ua.CardsAssigned),
		TotalComments:        len(ua.CommentsWritten),
		TotalQuestionAnswers: len(ua.QuestionAnswers),
	}
}
