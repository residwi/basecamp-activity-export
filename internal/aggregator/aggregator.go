package aggregator

import (
	"context"
	"fmt"
	"sync"

	"github.com/residwi/bcreview/internal/api"
	"github.com/residwi/bcreview/internal/models"
	"github.com/residwi/bcreview/internal/tracking"
	"github.com/residwi/bcreview/internal/tui"
)

// Aggregator handles activity aggregation
type Aggregator struct {
	client  *api.Client
	tracker *tracking.Tracker
	display *tui.Display
}

func NewAggregator(client *api.Client, opts ...Option) *Aggregator {
	agg := &Aggregator{client: client}

	for _, opt := range opts {
		opt(agg)
	}

	if agg.tracker == nil {
		agg.tracker = tracking.NewTracker(true)
	}
	if agg.display == nil {
		agg.display = tui.NewDisplay(agg.tracker)
	}

	return agg
}

func (a *Aggregator) AggregateUserActivity(ctx context.Context, year int) (*models.UserActivity, error) {
	trackerCtx := tracking.WithTracker(ctx, a.tracker)

	for _, recType := range tracking.RecordingTypes {
		a.tracker.RegisterTrace(recType)
	}

	fmt.Println("Fetching user profile...")
	profile, err := a.client.GetMyProfile(trackerCtx)
	if err != nil {
		a.display.PrintError("fetching user profile: %v", err)

		return nil, fmt.Errorf("fetching user profile: %w", err)
	}

	activity := &models.UserActivity{
		User: models.Person{
			ID:           profile.ID,
			Name:         profile.Name,
			EmailAddress: profile.EmailAddress,
		},
		Year: year,
	}

	fmt.Printf("Aggregating activity for %s in %d...\n", profile.Name, year)

	a.display.Start()
	defer a.display.Stop()

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		errOnce  sync.Once
		firstErr error
	)

	// Helper to capture the first error thread-safely
	fail := func(recType tracking.ResourceType, msg string, err error) {
		a.tracker.FailTrace(recType, err)
		a.display.PrintError("%s: %v", msg, err)
		errOnce.Do(func() {
			firstErr = fmt.Errorf("%s: %w", msg, err)
		})
	}

	wg.Add(6)

	// Messages
	go func() {
		defer wg.Done()
		recType := tracking.Messages
		msgCtx := tracking.WithResourceType(trackerCtx, recType)

		result, err := a.client.GetMessages(msgCtx, year)
		if err != nil {
			fail(recType, "fetching messages", err)
			return
		}

		a.tracker.StartSpan(recType, tracking.SpanFilter)
		filtered := filterMessageByUserAndYear(result, profile.ID, year)
		a.tracker.SetTotalItemsAfterFilter(recType, len(filtered))
		a.tracker.CompleteSpan(recType, tracking.SpanFilter)
		a.tracker.CompleteTrace(recType)

		mu.Lock()
		activity.MessagesCreated = filtered
		mu.Unlock()
	}()

	// Todos
	go func() {
		defer wg.Done()
		recType := tracking.Todos
		todoCtx := tracking.WithResourceType(trackerCtx, recType)

		result, err := a.client.GetTodos(todoCtx, year)
		if err != nil {
			fail(recType, "fetching todos", err)
			return
		}

		a.tracker.StartSpan(recType, tracking.SpanFilter)
		created, completed, assigned := filterTodosByUserAndYear(result, profile.ID, year)
		totalTodos := len(created) + len(completed) + len(assigned)
		a.tracker.SetTotalItemsAfterFilter(recType, totalTodos)
		a.tracker.CompleteSpan(recType, tracking.SpanFilter)
		a.tracker.CompleteTrace(recType)

		mu.Lock()
		activity.TodosCreated = created
		activity.TodosCompleted = completed
		activity.TodosAssigned = assigned
		mu.Unlock()
	}()

	// Todolists
	go func() {
		defer wg.Done()
		recType := tracking.Todolists
		listCtx := tracking.WithResourceType(trackerCtx, recType)

		result, err := a.client.GetTodolists(listCtx, year)
		if err != nil {
			fail(recType, "fetching todolists", err)
			return
		}

		a.tracker.StartSpan(recType, tracking.SpanFilter)
		filtered := filterTodolistsByUserAndYear(result, profile.ID, year)
		a.tracker.SetTotalItemsAfterFilter(recType, len(filtered))
		a.tracker.CompleteSpan(recType, tracking.SpanFilter)
		a.tracker.CompleteTrace(recType)

		mu.Lock()
		activity.TodolistsCreated = filtered
		mu.Unlock()
	}()

	// Cards
	go func() {
		defer wg.Done()
		recType := tracking.Cards
		cardCtx := tracking.WithResourceType(trackerCtx, recType)

		result, err := a.client.GetKanbanCards(cardCtx, year)
		if err != nil {
			fail(recType, "fetching Kanban cards", err)
			return
		}

		a.tracker.StartSpan(recType, tracking.SpanFilter)
		created, assigned := filterCardsByUserAndYear(result, profile.ID, year)
		totalCards := len(created) + len(assigned)
		a.tracker.SetTotalItemsAfterFilter(recType, totalCards)
		a.tracker.CompleteSpan(recType, tracking.SpanFilter)
		a.tracker.CompleteTrace(recType)

		mu.Lock()
		activity.CardsCreated = created
		activity.CardsAssigned = assigned
		mu.Unlock()
	}()

	// Comments
	go func() {
		defer wg.Done()
		recType := tracking.Comments
		commentCtx := tracking.WithResourceType(trackerCtx, recType)

		result, err := a.client.GetComments(commentCtx, year)
		if err != nil {
			fail(recType, "fetching comments", err)
			return
		}

		a.tracker.StartSpan(recType, tracking.SpanFilter)
		filtered := filterCommentsByUserAndYear(result, profile.ID, year)
		a.tracker.SetTotalItemsAfterFilter(recType, len(filtered))
		a.tracker.CompleteSpan(recType, tracking.SpanFilter)
		a.tracker.CompleteTrace(recType)

		mu.Lock()
		activity.CommentsWritten = filtered
		mu.Unlock()
	}()

	// Question Answers
	go func() {
		defer wg.Done()
		recType := tracking.QuestionAnswers
		qaCtx := tracking.WithResourceType(trackerCtx, recType)

		result, err := a.client.GetQuestionAnswers(qaCtx, year)
		if err != nil {
			fail(recType, "fetching question answers", err)
			return
		}

		a.tracker.StartSpan(recType, tracking.SpanFilter)
		filtered := filterQuestionAnswersByUserAndYear(result, profile.ID, year)
		a.tracker.SetTotalItemsAfterFilter(recType, len(filtered))
		a.tracker.CompleteSpan(recType, tracking.SpanFilter)
		a.tracker.CompleteTrace(recType)

		mu.Lock()
		activity.QuestionAnswers = filtered
		mu.Unlock()
	}()

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return activity, nil
}

func filterMessageByUserAndYear(messages []models.Message, userID int64, year int) []models.Message {
	filtered := make([]models.Message, 0, len(messages))

	for i := range messages {
		message := messages[i]
		if message.Creator.ID == userID && message.CreatedAt.Year() == year {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func filterTodosByUserAndYear(todos []models.Todo, userID int64, year int) ([]models.Todo, []models.Todo, []models.Todo) {
	created := make([]models.Todo, 0, len(todos))
	completed := make([]models.Todo, 0, len(todos))
	assigned := make([]models.Todo, 0, len(todos))

	for i := range todos {
		todo := todos[i]
		isAssignee := isUserAssignee(todo.Assignees, userID)
		isCreatedYear := todo.CreatedAt.Year() == year

		if todo.Creator.ID == userID && isCreatedYear {
			created = append(created, todo)
		}

		if todo.Completed && todo.Completion != nil && todo.Completion.CreatedAt != nil {
			if isAssignee && todo.Completion.CreatedAt.Year() == year {
				completed = append(completed, todo)
			}
		}

		if isAssignee && todo.Creator.ID != userID && isCreatedYear {
			assigned = append(assigned, todo)
		}
	}
	return created, completed, assigned
}

func filterTodolistsByUserAndYear(todolists []models.Todolist, userID int64, year int) []models.Todolist {
	filtered := make([]models.Todolist, 0, len(todolists))

	for i := range todolists {
		todolist := todolists[i]
		if todolist.Creator.ID == userID && todolist.CreatedAt.Year() == year {
			filtered = append(filtered, todolist)
		}
	}

	return filtered
}

func filterCardsByUserAndYear(cards []models.Card, userID int64, year int) ([]models.Card, []models.Card) {
	created := make([]models.Card, 0, len(cards))
	assigned := make([]models.Card, 0, len(cards))

	for i := range cards {
		card := cards[i]
		isCreatedYear := card.CreatedAt.Year() == year

		if card.Creator.ID == userID && isCreatedYear {
			created = append(created, card)
		}

		if isUserAssignee(card.Assignees, userID) && card.Creator.ID != userID && isCreatedYear {
			assigned = append(assigned, card)
		}
	}
	return created, assigned
}

func filterCommentsByUserAndYear(comments []models.Comment, userID int64, year int) []models.Comment {
	filtered := make([]models.Comment, 0, len(comments))

	for i := range comments {
		comment := comments[i]
		if comment.Creator.ID == userID && comment.CreatedAt.Year() == year {
			filtered = append(filtered, comment)
		}
	}
	return filtered
}

func filterQuestionAnswersByUserAndYear(answers []models.QuestionAnswer, userID int64, year int) []models.QuestionAnswer {
	filtered := make([]models.QuestionAnswer, 0, len(answers))
	for i := range answers {
		answer := answers[i]
		if answer.Creator.ID == userID && answer.CreatedAt.Year() == year {
			filtered = append(filtered, answer)
		}
	}
	return filtered
}

func isUserAssignee(assignees []models.Person, userID int64) bool {
	for i := range assignees {
		if assignees[i].ID == userID {
			return true
		}
	}
	return false
}
