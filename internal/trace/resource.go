package trace

// ResourceType identifies the type of resource being tracked.
type ResourceType string

const (
	Messages        ResourceType = "messages"
	Todos           ResourceType = "todos"
	Todolists       ResourceType = "todolists"
	KanbanCards     ResourceType = "kanban_cards"
	Comments        ResourceType = "comments"
	QuestionAnswers ResourceType = "question_answers"
)

var ResourceTypes = []ResourceType{
	Messages,
	Todos,
	Todolists,
	KanbanCards,
	Comments,
	QuestionAnswers,
}
