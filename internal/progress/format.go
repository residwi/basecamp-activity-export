package progress

import "github.com/residwi/basecamp-activity-export/internal/trace"

// statusIcon returns the display icon for an operation status.
func statusIcon(status trace.Status) string {
	switch status {
	case trace.StatusPending:
		return "[ ]"
	case trace.StatusRunning:
		return "[~]"
	case trace.StatusCompleted:
		return "[✓]"
	case trace.StatusError:
		return "[✗]"
	default:
		return "[ ]"
	}
}

// entityName returns the entity name with proper English pluralization.
// Returns singular for count of exactly 1, plural otherwise (including 0).
func entityName(recType trace.ResourceType, count int) string {
	var name string
	switch recType {
	case trace.Messages:
		name = "message"
	case trace.Todos:
		name = "todo"
	case trace.Todolists:
		name = "todolist"
	case trace.KanbanCards:
		name = "kanban card"
	case trace.Comments:
		name = "comment"
	case trace.QuestionAnswers:
		name = "answer"
	default:
		name = "item"
	}

	if count == 1 {
		return name
	}
	return name + "s"
}

// shortenURL truncates a URL to fit the maximum length by removing the beginning
func shortenURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return "..." + url[len(url)-maxLen+3:]
}
