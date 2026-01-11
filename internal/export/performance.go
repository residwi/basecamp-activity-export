package export

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/residwi/bcreview/internal/models"
)

// ExportPerformanceReview generates a comprehensive performance review with full content
func ExportPerformanceReview(activity *models.UserActivity, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	metrics := models.CalculateMetrics(activity)

	fmt.Fprintf(f, "# Performance Review - %d\n\n", activity.Year)
	fmt.Fprintf(f, "**Employee:** %s\n", activity.User.Name)
	fmt.Fprintf(f, "**Email:** %s\n", activity.User.EmailAddress)
	fmt.Fprintf(f, "**Review Period:** January 1, %d - December 31, %d\n", activity.Year, activity.Year)
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("January 2, 2006 at 3:04 PM"))
	fmt.Fprintf(f, "---\n\n")

	fmt.Fprintf(f, "## Executive Summary\n\n")
	fmt.Fprintf(f, "This document provides a comprehensive view of my contributions in %d, ", activity.Year)
	fmt.Fprintln(f, "including detailed descriptions of tasks, objectives, and epics.")
	fmt.Fprintf(f, "All content includes direct links to Basecamp for verification.\n\n")

	fmt.Fprintf(f, "### Key Metrics\n\n")
	fmt.Fprintln(f, "| Metric | Count |")
	fmt.Fprintln(f, "|--------|-------|")
	fmt.Fprintf(f, "| **Total Contributions** | %d |\n", metrics.TotalContributions)
	fmt.Fprintf(f, "| **Projects Contributed To** | %d |\n", metrics.ProjectsContributed)
	fmt.Fprintf(f, "| **Discussions Started** | %d |\n", metrics.MessagesCreated)
	fmt.Fprintf(f, "| **Tasks Created** | %d |\n", metrics.TodosCreated)
	fmt.Fprintf(f, "| **Tasks Completed** | %d |\n", metrics.TodosCompleted)
	fmt.Fprintf(f, "| **Todolists Created** | %d |\n", metrics.TodolistsCreated)
	fmt.Fprintf(f, "| **Cards Created** | %d |\n", len(activity.CardsCreated))
	fmt.Fprintf(f, "| **Cards Assigned** | %d |\n", len(activity.CardsAssigned))
	fmt.Fprintf(f, "| **Comments & Feedback** | %d |\n", metrics.CommentsWritten)
	fmt.Fprintf(f, "| **Daily Check-in Answers** | %d |\n", metrics.QuestionAnswers)
	fmt.Fprintln(f)

	epics := groupCardsByEpic(activity)

	if len(epics) > 0 {
		fmt.Fprintf(f, "---\n\n")
		fmt.Fprintf(f, "## Objectives & Epics\n\n")
		fmt.Fprintf(f, "Work organized by Epic/Objective (from Card Tables):\n\n")

		for _, epic := range epics {
			exportEpicSection(f, epic, activity)
		}
	}

	fmt.Fprintf(f, "---\n\n")
	fmt.Fprintf(f, "## Detailed Task Breakdown\n\n")

	if len(activity.TodosCreated) > 0 {
		exportTodosSection(f, "Tasks I Created", activity.TodosCreated, activity.CommentsWritten, activity.User.ID)
	}

	if len(activity.TodosAssigned) > 0 {
		exportTodosSection(f, "Tasks Assigned to Me", activity.TodosAssigned, activity.CommentsWritten, activity.User.ID)
	}

	if len(activity.TodolistsCreated) > 0 {
		fmt.Fprintf(f, "### Todolists I Created\n\n")
		exportTodolists(f, activity.TodolistsCreated, activity.CommentsWritten, activity.User.ID)
	}

	if len(activity.CardsCreated) > 0 {
		exportCardsSection(f, "Cards I Created", activity.CardsCreated, activity.CommentsWritten, activity.User.ID)
	}

	if len(activity.CardsAssigned) > 0 {
		exportCardsSection(f, "Cards Assigned to Me", activity.CardsAssigned, activity.CommentsWritten, activity.User.ID)
	}

	if len(activity.MessagesCreated) > 0 {
		fmt.Fprintf(f, "---\n\n")
		fmt.Fprintf(f, "## Discussions Initiated\n\n")
		exportMessagesSection(f, activity.MessagesCreated, activity.CommentsWritten, activity.User.ID)
	}

	if len(activity.QuestionAnswers) > 0 {
		fmt.Fprintf(f, "---\n\n")
		fmt.Fprintf(f, "## Daily Check-ins\n\n")
		exportQuestionAnswersSection(f, activity.QuestionAnswers, activity.CommentsWritten, activity.User.ID)
	}

	fmt.Fprintf(f, "---\n\n")
	fmt.Fprintf(f, "## About This Report\n\n")
	fmt.Fprintln(f, "This comprehensive report includes:")
	fmt.Fprintln(f, "- Full descriptions and content (HTML converted to Markdown)")
	fmt.Fprintln(f, "- Direct links to all items in Basecamp for verification")
	fmt.Fprintln(f, "- My comments and contributions on each item")
	fmt.Fprintf(f, "- Organized by Epic/Objective\n\n")
	fmt.Fprintln(f, "*This report was automatically generated from Basecamp activity data.*")

	return nil
}

// Epic represents a Card Table with associated work
type Epic struct {
	Title       string
	Description string
	AppURL      string
	Cards       []models.Card
}

func groupCardsByEpic(activity *models.UserActivity) []Epic {
	epicMap := make(map[string]*Epic)

	// Group cards created by user
	for _, card := range activity.CardsCreated {
		key := card.Parent.Title
		if key == "" {
			key = "Uncategorized"
		}

		if _, exists := epicMap[key]; !exists {
			epicMap[key] = &Epic{
				Title:       card.Parent.Title,
				Description: card.Parent.Type,
				AppURL:      card.Parent.AppURL,
				Cards:       []models.Card{},
			}
		}
		epicMap[key].Cards = append(epicMap[key].Cards, card)
	}

	// Add cards assigned to user
	for _, card := range activity.CardsAssigned {
		key := card.Parent.Title
		if key == "" {
			key = "Uncategorized"
		}

		if _, exists := epicMap[key]; !exists {
			epicMap[key] = &Epic{
				Title:       card.Parent.Title,
				Description: card.Parent.Type,
				AppURL:      card.Parent.AppURL,
				Cards:       []models.Card{},
			}
		}
		epicMap[key].Cards = append(epicMap[key].Cards, card)
	}

	// Convert map to sorted slice
	var epics []Epic
	for _, epic := range epicMap {
		epics = append(epics, *epic)
	}

	sort.Slice(epics, func(i, j int) bool {
		return epics[i].Title < epics[j].Title
	})

	return epics
}

func exportEpicSection(f *os.File, epic Epic, activity *models.UserActivity) {
	fmt.Fprintf(f, "### Epic: %s\n\n", epic.Title)
	if epic.AppURL != "" {
		fmt.Fprintf(f, "**Reference:** [View Card Table](%s)\n\n", epic.AppURL)
	}
	fmt.Fprintf(f, "**Total Cards:** %d\n\n", len(epic.Cards))

	// Show cards in this epic
	for i, card := range epic.Cards {
		fmt.Fprintf(f, "#### Card %d: %s\n\n", i+1, card.Title)
		fmt.Fprintf(f, "- **Created:** %s\n", card.CreatedAt.Format("January 2, 2006"))
		if card.DueOn != nil {
			fmt.Fprintf(f, "- **Due:** %s\n", card.DueOn.Format("January 2, 2006"))
		}
		fmt.Fprintf(f, "- **Reference:** [View Card](%s)\n", card.AppURL)

		// Convert and display content
		if card.Content != "" {
			markdown := ConvertHTMLToMarkdown(card.Content)
			fmt.Fprintf(f, "\n**Description:**\n\n%s\n\n", markdown)
		}

		// Show assignees
		if len(card.Assignees) > 0 {
			fmt.Fprintln(f, "**Assigned to:**")
			for _, assignee := range card.Assignees {
				fmt.Fprintf(f, "- %s\n", assignee.Name)
			}
			fmt.Fprintln(f)
		}

		// Show user's comments on this card
		cardComments := getCommentsForRecording(activity.CommentsWritten, card.ID, activity.User.ID)
		if len(cardComments) > 0 {
			fmt.Fprintf(f, "**My Comments:**\n\n")
			for _, comment := range cardComments {
				markdown := ConvertHTMLToMarkdown(comment.Content)
				fmt.Fprintf(f, "- %s: %s\n", comment.CreatedAt.Format("Jan 2"), markdown)
			}
			fmt.Fprintln(f)
		}
	}

	fmt.Fprintln(f)
}

func exportTodosSection(f *os.File, sectionTitle string, todos []models.Todo, comments []models.Comment, userID int64) {
	fmt.Fprintf(f, "### %s (%d)\n\n", sectionTitle, len(todos))

	// Sort by creation date
	sort.Slice(todos, func(i, j int) bool {
		return todos[i].CreatedAt.Before(todos[j].CreatedAt)
	})

	for i, todo := range todos {
		status := "Pending"
		if todo.Completed {
			status = "Completed"
		}

		fmt.Fprintf(f, "#### %d. %s\n\n", i+1, todo.Title)
		fmt.Fprintf(f, "- **Status:** %s\n", status)
		fmt.Fprintf(f, "- **Created:** %s\n", todo.CreatedAt.Format("January 2, 2006"))
		if todo.DueOn != nil {
			fmt.Fprintf(f, "- **Due:** %s\n", todo.DueOn.Format("January 2, 2006"))
		}
		if todo.Completed && todo.Completion.CreatedAt != nil {
			fmt.Fprintf(f, "- **Completed:** %s\n", todo.Completion.CreatedAt.Format("January 2, 2006"))
		}
		fmt.Fprintf(f, "- **Project:** %s\n", todo.Bucket.Name)
		fmt.Fprintf(f, "- **Reference:** [View Task](%s)\n", todo.AppURL)

		if todo.Description != "" {
			markdown := ConvertHTMLToMarkdown(todo.Description)
			fmt.Fprintf(f, "\n**Description:**\n\n%s\n\n", markdown)
		}

		// Show assignees
		if len(todo.Assignees) > 0 {
			fmt.Fprintln(f, "**Assigned to:**")
			for _, assignee := range todo.Assignees {
				fmt.Fprintf(f, "- %s\n", assignee.Name)
			}
			fmt.Fprintln(f)
		}

		// Show user's comments
		todoComments := getCommentsForRecording(comments, todo.ID, userID)
		if len(todoComments) > 0 {
			fmt.Fprintf(f, "**My Comments:**\n\n")
			for _, comment := range todoComments {
				markdown := ConvertHTMLToMarkdown(comment.Content)
				fmt.Fprintf(f, "- %s: %s\n", comment.CreatedAt.Format("Jan 2"), markdown)
			}
			fmt.Fprintln(f)
		}

		fmt.Fprintln(f)
	}
}

func exportTodolists(f *os.File, todolists []models.Todolist, comments []models.Comment, userID int64) {
	// Sort by creation date
	sort.Slice(todolists, func(i, j int) bool {
		return todolists[i].CreatedAt.Before(todolists[j].CreatedAt)
	})

	for i, todolist := range todolists {
		fmt.Fprintf(f, "#### %d. %s\n\n", i+1, todolist.Title)
		fmt.Fprintf(f, "- **Created:** %s\n", todolist.CreatedAt.Format("January 2, 2006"))
		fmt.Fprintf(f, "- **Project:** %s\n", todolist.Bucket.Name)
		fmt.Fprintf(f, "- **Status:** %s\n", todolist.CompletedRatio)
		fmt.Fprintf(f, "- **Reference:** [View Todolist](%s)\n", todolist.AppURL)

		// Convert and display content/description
		if todolist.Content != "" {
			markdown := ConvertHTMLToMarkdown(todolist.Content)
			fmt.Fprintf(f, "\n**Description:**\n\n%s\n\n", markdown)
		}

		// Show user's comments
		todolistComments := getCommentsForRecording(comments, todolist.ID, userID)
		if len(todolistComments) > 0 {
			fmt.Fprintf(f, "**My Comments:**\n\n")
			for _, comment := range todolistComments {
				markdown := ConvertHTMLToMarkdown(comment.Content)
				fmt.Fprintf(f, "- %s: %s\n", comment.CreatedAt.Format("Jan 2"), markdown)
			}
			fmt.Fprintln(f)
		}

		fmt.Fprintln(f)
	}
}

func exportCardsSection(f *os.File, sectionTitle string, cards []models.Card, comments []models.Comment, userID int64) {
	fmt.Fprintf(f, "### %s (%d)\n\n", sectionTitle, len(cards))

	// Sort by creation date
	sort.Slice(cards, func(i, j int) bool {
		return cards[i].CreatedAt.Before(cards[j].CreatedAt)
	})

	for i, card := range cards {
		fmt.Fprintf(f, "#### %d. %s\n\n", i+1, card.Title)
		fmt.Fprintf(f, "- **Card Table:** %s\n", card.Parent.Title)
		if card.Parent.AppURL != "" {
			fmt.Fprintf(f, "- **Card Table Reference:** [View](%s)\n", card.Parent.AppURL)
		}
		fmt.Fprintf(f, "- **Created:** %s\n", card.CreatedAt.Format("January 2, 2006"))
		if card.DueOn != nil {
			fmt.Fprintf(f, "- **Due:** %s\n", card.DueOn.Format("January 2, 2006"))
		}
		fmt.Fprintf(f, "- **Project:** %s\n", card.Bucket.Name)
		fmt.Fprintf(f, "- **Reference:** [View Card](%s)\n", card.AppURL)

		// Convert and display content
		if card.Content != "" {
			markdown := ConvertHTMLToMarkdown(card.Content)
			fmt.Fprintf(f, "\n**Description:**\n\n%s\n\n", markdown)
		}

		// Show assignees
		if len(card.Assignees) > 0 {
			fmt.Fprintln(f, "**Assigned to:**")
			for _, assignee := range card.Assignees {
				fmt.Fprintf(f, "- %s\n", assignee.Name)
			}
			fmt.Fprintln(f)
		}

		// Show user's comments
		cardComments := getCommentsForRecording(comments, card.ID, userID)
		if len(cardComments) > 0 {
			fmt.Fprintf(f, "**My Comments:**\n\n")
			for _, comment := range cardComments {
				markdown := ConvertHTMLToMarkdown(comment.Content)
				fmt.Fprintf(f, "- %s: %s\n", comment.CreatedAt.Format("Jan 2"), markdown)
			}
			fmt.Fprintln(f)
		}

		fmt.Fprintln(f)
	}
}

func exportMessagesSection(f *os.File, messages []models.Message, comments []models.Comment, userID int64) {
	// Sort by creation date
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})

	for i, msg := range messages {
		fmt.Fprintf(f, "### %d. %s\n\n", i+1, msg.Title)
		fmt.Fprintf(f, "- **Created:** %s\n", msg.CreatedAt.Format("January 2, 2006"))
		fmt.Fprintf(f, "- **Project:** %s\n", msg.Bucket.Name)
		if msg.Subject != "" && msg.Subject != msg.Title {
			fmt.Fprintf(f, "- **Subject:** %s\n", msg.Subject)
		}
		fmt.Fprintf(f, "- **Reference:** [View Discussion](%s)\n", msg.AppURL)

		// Convert and display content
		if msg.Content != "" {
			markdown := ConvertHTMLToMarkdown(msg.Content)
			fmt.Fprintf(f, "\n**Content:**\n\n%s\n\n", markdown)
		}

		// Show user's comments
		msgComments := getCommentsForRecording(comments, msg.ID, userID)
		if len(msgComments) > 0 {
			fmt.Fprintf(f, "**My Follow-up Comments:**\n\n")
			for _, comment := range msgComments {
				markdown := ConvertHTMLToMarkdown(comment.Content)
				fmt.Fprintf(f, "- %s: %s\n", comment.CreatedAt.Format("Jan 2"), markdown)
			}
			fmt.Fprintln(f)
		}

		fmt.Fprintln(f)
	}
}

// getCommentsForRecording filters comments for a specific recording
func getCommentsForRecording(comments []models.Comment, recordingID int64, userID int64) []models.Comment {
	var filtered []models.Comment
	for _, comment := range comments {
		if comment.Parent.ID == recordingID && comment.Creator.ID == userID {
			filtered = append(filtered, comment)
		}
	}

	// Sort by date
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
	})

	return filtered
}

func exportQuestionAnswersSection(f *os.File, answers []models.QuestionAnswer, comments []models.Comment, userID int64) {
	// Sort by creation date (chronological order)
	sort.Slice(answers, func(i, j int) bool {
		return answers[i].CreatedAt.Before(answers[j].CreatedAt)
	})

	for i, answer := range answers {
		fmt.Fprintf(f, "### %d. %s\n\n", i+1, answer.CreatedAt.Format("January 2, 2006"))
		fmt.Fprintf(f, "- **Question:** %s\n", answer.Title)
		fmt.Fprintf(f, "- **Project:** %s\n", answer.Bucket.Name)
		fmt.Fprintf(f, "- **Reference:** [View Check-in](%s)\n", answer.AppURL)

		// Convert and display answer content
		if answer.Content != "" {
			markdown := ConvertHTMLToMarkdown(answer.Content)
			fmt.Fprintf(f, "\n**Answer:**\n\n%s\n\n", markdown)
		}

		// Show user's comments on this answer (if any)
		answerComments := getCommentsForRecording(comments, answer.ID, userID)
		if len(answerComments) > 0 {
			fmt.Fprintf(f, "**My Follow-up Comments:**\n\n")
			for _, comment := range answerComments {
				markdown := ConvertHTMLToMarkdown(comment.Content)
				fmt.Fprintf(f, "- %s: %s\n", comment.CreatedAt.Format("Jan 2"), markdown)
			}
			fmt.Fprintln(f)
		}

		fmt.Fprintln(f)
	}
}
