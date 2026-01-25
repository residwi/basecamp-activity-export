package export

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/activity"
	"github.com/residwi/basecamp-activity-export/internal/basecamp"
)

// commentIndex maps recording IDs to comments made by the user
type commentIndex map[int64][]basecamp.Comment

// reportData holds pre-processed data for report generation
type reportData struct {
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
	commentIndex        commentIndex
	metrics             reportMetrics
	epics               []epic
}

type reportMetrics struct {
	totalContributions  int
	projectsContributed int
}

// epic represents a Card Table with associated work.
type epic struct {
	Title       string
	Description string
	AppURL      string
	Cards       []basecamp.KanbanCard
}

// FormatAndSaveReport generates a markdown report from user activity
func FormatAndSaveReport(ua *activity.UserActivity, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	data := prepareReportData(ua)

	writeHeader(w, data)
	writeExecutiveSummary(w, data)

	if len(data.epics) > 0 {
		writeEpicsSection(w, data)
	}

	writeTaskBreakdown(w, data)

	if len(data.messagesCreated) > 0 {
		writeDiscussionsSection(w, data)
	}

	if len(data.questionAnswers) > 0 {
		writeCheckInsSection(w, data)
	}

	writeFooter(w)

	if err := w.Flush(); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}

	return nil
}

func prepareReportData(ua *activity.UserActivity) *reportData {
	user := ua.User()

	data := &reportData{
		user:                user,
		year:                ua.Year(),
		messagesCreated:     ua.MessagesCreated(),
		todosCreated:        ua.TodosCreated(),
		todosCompleted:      ua.TodosCompleted(),
		todosAssigned:       ua.TodosAssigned(),
		todolistsCreated:    ua.TodolistsCreated(),
		kanbanCardsCreated:  ua.KanbanCardsCreated(),
		kanbanCardsAssigned: ua.KanbanCardsAssigned(),
		commentsWritten:     ua.CommentsWritten(),
		questionAnswers:     ua.QuestionAnswers(),
	}
	data.commentIndex = buildCommentIndex(data.commentsWritten, user.ID)

	sortByDate(data.messagesCreated, func(m basecamp.Message) time.Time { return m.CreatedAt })
	sortByDate(data.todosCreated, func(t basecamp.Todo) time.Time { return t.CreatedAt })
	sortByDate(data.todosAssigned, func(t basecamp.Todo) time.Time { return t.CreatedAt })
	sortByDate(data.todolistsCreated, func(t basecamp.Todolist) time.Time { return t.CreatedAt })
	sortByDate(data.kanbanCardsCreated, func(c basecamp.KanbanCard) time.Time { return c.CreatedAt })
	sortByDate(data.kanbanCardsAssigned, func(c basecamp.KanbanCard) time.Time { return c.CreatedAt })
	sortByDate(data.questionAnswers, func(q basecamp.QuestionAnswer) time.Time { return q.CreatedAt })

	data.metrics = calculateMetrics(data)
	data.epics = groupKanbanCardsByEpic(data.kanbanCardsCreated, data.kanbanCardsAssigned)

	return data
}

func groupKanbanCardsByEpic(cardsCreated, cardsAssigned []basecamp.KanbanCard) []epic {
	epicMap := make(map[string]*epic, len(cardsCreated)/3+1)

	addCard := func(card basecamp.KanbanCard) {
		key := card.Parent.Title
		if key == "" {
			key = "Uncategorized"
		}

		ep := epicMap[key]
		if ep == nil {
			ep = &epic{
				Title:       key,
				Description: card.Parent.Type,
				AppURL:      card.Parent.AppURL,
				Cards:       make([]basecamp.KanbanCard, 0, 4), // Pre-allocate small
			}
			epicMap[key] = ep
		}
		ep.Cards = append(ep.Cards, card)
	}

	for _, card := range cardsCreated {
		addCard(card)
	}
	for _, card := range cardsAssigned {
		addCard(card)
	}

	epics := make([]epic, 0, len(epicMap))
	for _, ep := range epicMap {
		epics = append(epics, *ep)
	}

	sort.Slice(epics, func(i, j int) bool {
		return epics[i].Title < epics[j].Title
	})

	return epics
}

func writeHeader(w *bufio.Writer, data *reportData) {
	fmt.Fprintf(w, "# Activity Report - %d\n\n", data.year)
	fmt.Fprintf(w, "**User:** %s\n", data.user.Name)
	fmt.Fprintf(w, "**Email:** %s\n", data.user.EmailAddress)
	fmt.Fprintf(w, "**Report Period:** January 1, %d - December 31, %d\n", data.year, data.year)
	fmt.Fprintf(w, "**Generated:** %s\n\n", time.Now().Format("January 2, 2006 at 3:04 PM"))
	fmt.Fprint(w, "---\n\n")
}

func writeExecutiveSummary(w *bufio.Writer, data *reportData) {
	fmt.Fprint(w, "## Summary\n\n")
	fmt.Fprintf(w, "This document provides a comprehensive view of activity in %d, ", data.year)
	fmt.Fprint(w, "including detailed descriptions of tasks, objectives, and epics.\n")
	fmt.Fprint(w, "All content includes direct links to Basecamp for verification.\n\n")

	fmt.Fprint(w, "### Key Metrics\n\n")
	fmt.Fprint(w, "| Metric | Count |\n")
	fmt.Fprint(w, "|--------|-------|\n")
	fmt.Fprintf(w, "| **Total Activities** | %d |\n", data.metrics.totalContributions)
	fmt.Fprintf(w, "| **Projects Involved** | %d |\n", data.metrics.projectsContributed)
	fmt.Fprintf(w, "| **Discussions Started** | %d |\n", len(data.messagesCreated))
	fmt.Fprintf(w, "| **Tasks Created** | %d |\n", len(data.todosCreated))
	fmt.Fprintf(w, "| **Tasks Completed** | %d |\n", len(data.todosCompleted))
	fmt.Fprintf(w, "| **Todolists Created** | %d |\n", len(data.todolistsCreated))
	fmt.Fprintf(w, "| **Cards Created** | %d |\n", len(data.kanbanCardsCreated))
	fmt.Fprintf(w, "| **Cards Assigned** | %d |\n", len(data.kanbanCardsAssigned))
	fmt.Fprintf(w, "| **Comments & Feedback** | %d |\n", len(data.commentsWritten))
	fmt.Fprintf(w, "| **Daily Check-in Answers** | %d |\n\n", len(data.questionAnswers))
}

func writeEpicsSection(w *bufio.Writer, data *reportData) {
	fmt.Fprint(w, "---\n\n")
	fmt.Fprint(w, "## Objectives & Epics\n\n")
	fmt.Fprint(w, "Work organized by Epic/Objective (from Card Tables):\n\n")

	for _, ep := range data.epics {
		writeEpic(w, ep, data.commentIndex)
	}
}

func writeEpic(w *bufio.Writer, ep epic, idx commentIndex) {
	fmt.Fprintf(w, "### Epic: %s\n\n", ep.Title)
	if ep.AppURL != "" {
		fmt.Fprintf(w, "**Reference:** [View Card Table](%s)\n\n", ep.AppURL)
	}
	fmt.Fprintf(w, "**Total Cards:** %d\n\n", len(ep.Cards))

	for i, card := range ep.Cards {
		fmt.Fprintf(w, "#### Card %d: %s\n\n", i+1, card.Title)
		fmt.Fprintf(w, "- **Created:** %s\n", card.CreatedAt.Format("January 2, 2006"))
		if card.DueOn != "" {
			if dueOn, err := time.Parse(time.DateOnly, card.DueOn); err == nil {
				fmt.Fprintf(w, "- **Due:** %s\n", dueOn.Format("January 2, 2006"))
			}
		}
		fmt.Fprintf(w, "- **Reference:** [View Card](%s)\n", card.AppURL)

		if card.Content != "" {
			fmt.Fprintf(w, "\n**Description:**\n\n%s\n\n", ConvertHTMLToMarkdown(card.Content))
		}

		writeAssignees(w, card.Assignees)
		writeComments(w, idx.getComments(card.ID), "My Comments")
	}

	fmt.Fprint(w, "\n")
}

func writeTaskBreakdown(w *bufio.Writer, data *reportData) {
	fmt.Fprint(w, "---\n\n")
	fmt.Fprint(w, "## Detailed Task Breakdown\n\n")

	if len(data.todosCreated) > 0 {
		writeTodosSection(w, "Tasks I Created", data.todosCreated, data.commentIndex)
	}

	if len(data.todosAssigned) > 0 {
		writeTodosSection(w, "Tasks Assigned to Me", data.todosAssigned, data.commentIndex)
	}

	if len(data.todolistsCreated) > 0 {
		fmt.Fprint(w, "### Todolists I Created\n\n")
		writeTodolists(w, data.todolistsCreated, data.commentIndex)
	}

	if len(data.kanbanCardsCreated) > 0 {
		writeKanbanCardsSection(w, "Kanban Cards I Created", data.kanbanCardsCreated, data.commentIndex)
	}

	if len(data.kanbanCardsAssigned) > 0 {
		writeKanbanCardsSection(w, "Kanban Cards Assigned to Me", data.kanbanCardsAssigned, data.commentIndex)
	}
}

func writeTodosSection(w *bufio.Writer, title string, todos []basecamp.Todo, idx commentIndex) {
	fmt.Fprintf(w, "### %s (%d)\n\n", title, len(todos))

	for i, todo := range todos {
		status := "Pending"
		if todo.Completed {
			status = "Completed"
		}

		fmt.Fprintf(w, "#### %d. %s\n\n", i+1, todo.Title)
		fmt.Fprintf(w, "- **Status:** %s\n", status)
		fmt.Fprintf(w, "- **Created:** %s\n", todo.CreatedAt.Format("January 2, 2006"))

		if todo.DueOn != "" {
			if dueOn, err := time.Parse(time.DateOnly, todo.DueOn); err == nil {
				fmt.Fprintf(w, "- **Due:** %s\n", dueOn.Format("January 2, 2006"))
			}
		}
		if todo.Completed && todo.Completion != nil && todo.Completion.CreatedAt != nil {
			fmt.Fprintf(w, "- **Completed:** %s\n", todo.Completion.CreatedAt.Format("January 2, 2006"))
		}

		fmt.Fprintf(w, "- **Project:** %s\n", todo.Bucket.Name)
		fmt.Fprintf(w, "- **Reference:** [View Task](%s)\n", todo.AppURL)

		if todo.Description != "" {
			fmt.Fprintf(w, "\n**Description:**\n\n%s\n\n", ConvertHTMLToMarkdown(todo.Description))
		}

		writeAssignees(w, todo.Assignees)
		writeComments(w, idx.getComments(todo.ID), "My Comments")
		fmt.Fprint(w, "\n")
	}
}

func writeTodolists(w *bufio.Writer, todolists []basecamp.Todolist, idx commentIndex) {
	for i, todolist := range todolists {
		fmt.Fprintf(w, "#### %d. %s\n\n", i+1, todolist.Title)
		fmt.Fprintf(w, "- **Created:** %s\n", todolist.CreatedAt.Format("January 2, 2006"))
		fmt.Fprintf(w, "- **Project:** %s\n", todolist.Bucket.Name)
		fmt.Fprintf(w, "- **Status:** %s\n", todolist.CompletedRatio)
		fmt.Fprintf(w, "- **Reference:** [View Todolist](%s)\n", todolist.AppURL)

		if todolist.Content != "" {
			fmt.Fprintf(w, "\n**Description:**\n\n%s\n\n", ConvertHTMLToMarkdown(todolist.Content))
		}

		writeComments(w, idx.getComments(todolist.ID), "My Comments")
		fmt.Fprint(w, "\n")
	}
}

func writeKanbanCardsSection(w *bufio.Writer, title string, cards []basecamp.KanbanCard, idx commentIndex) {
	fmt.Fprintf(w, "### %s (%d)\n\n", title, len(cards))

	for i, card := range cards {
		fmt.Fprintf(w, "#### %d. %s\n\n", i+1, card.Title)
		fmt.Fprintf(w, "- **Card Table:** %s\n", card.Parent.Title)
		if card.Parent.AppURL != "" {
			fmt.Fprintf(w, "- **Card Table Reference:** [View](%s)\n", card.Parent.AppURL)
		}
		fmt.Fprintf(w, "- **Created:** %s\n", card.CreatedAt.Format("January 2, 2006"))
		if card.DueOn != "" {
			if dueOn, err := time.Parse(time.DateOnly, card.DueOn); err == nil {
				fmt.Fprintf(w, "- **Due:** %s\n", dueOn.Format("January 2, 2006"))
			}
		}
		fmt.Fprintf(w, "- **Project:** %s\n", card.Bucket.Name)
		fmt.Fprintf(w, "- **Reference:** [View Card](%s)\n", card.AppURL)

		if card.Content != "" {
			fmt.Fprintf(w, "\n**Description:**\n\n%s\n\n", ConvertHTMLToMarkdown(card.Content))
		}

		writeAssignees(w, card.Assignees)
		writeComments(w, idx.getComments(card.ID), "My Comments")
		fmt.Fprint(w, "\n")
	}
}

func writeDiscussionsSection(w *bufio.Writer, data *reportData) {
	fmt.Fprint(w, "---\n\n")
	fmt.Fprint(w, "## Discussions Initiated\n\n")

	for i, msg := range data.messagesCreated {
		fmt.Fprintf(w, "### %d. %s\n\n", i+1, msg.Title)
		fmt.Fprintf(w, "- **Created:** %s\n", msg.CreatedAt.Format("January 2, 2006"))
		fmt.Fprintf(w, "- **Project:** %s\n", msg.Bucket.Name)
		if msg.Subject != "" && msg.Subject != msg.Title {
			fmt.Fprintf(w, "- **Subject:** %s\n", msg.Subject)
		}
		fmt.Fprintf(w, "- **Reference:** [View Discussion](%s)\n", msg.AppURL)

		if msg.Content != "" {
			fmt.Fprintf(w, "\n**Content:**\n\n%s\n\n", ConvertHTMLToMarkdown(msg.Content))
		}

		writeComments(w, data.commentIndex.getComments(msg.ID), "My Follow-up Comments")
		fmt.Fprint(w, "\n")
	}
}

func writeCheckInsSection(w *bufio.Writer, data *reportData) {
	fmt.Fprint(w, "---\n\n")
	fmt.Fprint(w, "## Daily Check-ins\n\n")

	for i, answer := range data.questionAnswers {
		fmt.Fprintf(w, "### %d. %s\n\n", i+1, answer.CreatedAt.Format("January 2, 2006"))
		fmt.Fprintf(w, "- **Question:** %s\n", answer.Title)
		fmt.Fprintf(w, "- **Project:** %s\n", answer.Bucket.Name)
		fmt.Fprintf(w, "- **Reference:** [View Check-in](%s)\n", answer.AppURL)

		if answer.Content != "" {
			fmt.Fprintf(w, "\n**Answer:**\n\n%s\n\n", ConvertHTMLToMarkdown(answer.Content))
		}

		writeComments(w, data.commentIndex.getComments(answer.ID), "My Follow-up Comments")
		fmt.Fprint(w, "\n")
	}
}

func writeFooter(w *bufio.Writer) {
	fmt.Fprint(w, "---\n\n")
	fmt.Fprint(w, "## About This Report\n\n")
	fmt.Fprint(w, "This comprehensive report includes:\n")
	fmt.Fprint(w, "- Full descriptions and content (HTML converted to Markdown)\n")
	fmt.Fprint(w, "- Direct links to all items in Basecamp for verification\n")
	fmt.Fprint(w, "- My comments and contributions on each item\n")
	fmt.Fprint(w, "- Organized by Epic/Objective\n\n")
	fmt.Fprint(w, "*This report was automatically generated from Basecamp activity data.*\n")
}

func writeAssignees(w *bufio.Writer, assignees []basecamp.Person) {
	if len(assignees) == 0 {
		return
	}

	fmt.Fprint(w, "**Assigned to:**\n")
	for _, assignee := range assignees {
		fmt.Fprintf(w, "- %s\n", assignee.Name)
	}
	fmt.Fprint(w, "\n")
}

func writeComments(w *bufio.Writer, comments []basecamp.Comment, title string) {
	if len(comments) == 0 {
		return
	}

	fmt.Fprintf(w, "**%s:**\n\n", title)
	for _, comment := range comments {
		fmt.Fprintf(w, "- %s: %s\n", comment.CreatedAt.Format("Jan 2"), ConvertHTMLToMarkdown(comment.Content))
	}
	fmt.Fprint(w, "\n")
}

func sortByDate[T any](items []T, getDate func(T) time.Time) {
	sort.Slice(items, func(i, j int) bool {
		return getDate(items[i]).Before(getDate(items[j]))
	})
}

func calculateMetrics(data *reportData) reportMetrics {
	total := len(data.messagesCreated) + len(data.todosCreated) +
		len(data.todosCompleted) + len(data.kanbanCardsCreated) +
		len(data.commentsWritten) + len(data.questionAnswers)

	projects := make(map[int64]struct{})
	for i := range data.messagesCreated {
		projects[data.messagesCreated[i].Bucket.ID] = struct{}{}
	}
	for i := range data.todosCreated {
		projects[data.todosCreated[i].Bucket.ID] = struct{}{}
	}
	for i := range data.kanbanCardsCreated {
		projects[data.kanbanCardsCreated[i].Bucket.ID] = struct{}{}
	}

	return reportMetrics{
		totalContributions:  total,
		projectsContributed: len(projects),
	}
}

func buildCommentIndex(comments []basecamp.Comment, userID int64) commentIndex {
	// pre-allocate with reasonable estimate
	idx := make(commentIndex, len(comments)/5+1)

	for i := range comments {
		c := &comments[i]
		if c.Creator.ID == userID {
			idx[c.Parent.ID] = append(idx[c.Parent.ID], *c)
		}
	}

	// sort by ascending created date within each group
	for _, group := range idx {
		sort.Slice(group, func(i, j int) bool {
			return group[i].CreatedAt.Before(group[j].CreatedAt)
		})
	}

	return idx
}

// getComments returns comments for a recording from the pre-built index.
func (idx commentIndex) getComments(recordingID int64) []basecamp.Comment {
	return idx[recordingID]
}
