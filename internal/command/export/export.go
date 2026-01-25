package export

import (
	"context"
	"fmt"

	"github.com/residwi/basecamp-activity-export/internal/activity"
	"github.com/residwi/basecamp-activity-export/internal/basecamp"
	"github.com/residwi/basecamp-activity-export/internal/progress"
	"github.com/residwi/basecamp-activity-export/internal/trace"
	"golang.org/x/sync/errgroup"
)

type Export struct {
	basecamp *basecamp.Client
	tracker  *trace.Tracer
	display  *progress.Display
	filename string
}

func New(opts ...optionFunc) *Export {
	e := &Export{}

	for _, opt := range opts {
		opt(e)
	}

	if e.display == nil && e.tracker != nil {
		e.display = progress.NewDisplay(e.tracker)
	}

	return e
}

func (s *Export) FetchActivity(ctx context.Context, year int) (*activity.UserActivity, error) {
	trackerCtx := trace.WithTracker(ctx, s.tracker)

	for _, recType := range trace.ResourceTypes {
		s.tracker.RegisterTrace(recType)
	}

	fmt.Println("Fetching user profile...")
	profile, err := s.basecamp.GetMyProfile(trackerCtx)
	if err != nil {
		s.display.PrintError("fetching user profile: %v", err)
		return nil, fmt.Errorf("fetching user profile: %w", err)
	}

	userActivity := activity.New(profile, year)
	fmt.Printf("Aggregating activity for %s in %d...\n", profile.Name, year)

	s.display.Start()
	defer s.display.Stop()

	g, gCtx := errgroup.WithContext(trackerCtx)

	g.Go(func() error {
		recType := trace.Messages
		msgCtx := trace.WithResourceType(gCtx, recType)

		res, err := s.basecamp.GetMessages(msgCtx, year)
		if err != nil {
			s.tracker.FailTrace(recType, err)
			s.display.PrintError("fetching messages: %v", err)
			return err
		}

		s.tracker.StartSpan(recType, trace.SpanFilter)
		filtered := activity.FilterMessages(res, profile.ID, year)
		s.tracker.SetTotalItemsAfterFilter(recType, len(filtered))
		s.tracker.CompleteSpan(recType)
		s.tracker.CompleteTrace(recType)

		userActivity.AddMessages(filtered)
		return nil
	})

	g.Go(func() error {
		recType := trace.Todos
		todoCtx := trace.WithResourceType(gCtx, recType)

		res, err := s.basecamp.GetTodos(todoCtx, year)
		if err != nil {
			s.tracker.FailTrace(recType, err)
			s.display.PrintError("fetching todos: %v", err)
			return err
		}

		s.tracker.StartSpan(recType, trace.SpanFilter)
		created, completed, assigned := activity.FilterTodos(res, profile.ID, year)
		s.tracker.SetTotalItemsAfterFilter(recType, len(created)+len(completed)+len(assigned))
		s.tracker.CompleteSpan(recType)
		s.tracker.CompleteTrace(recType)

		userActivity.AddTodos(created, completed, assigned)
		return nil
	})

	g.Go(func() error {
		recType := trace.Todolists
		todolistCtx := trace.WithResourceType(gCtx, recType)

		res, err := s.basecamp.GetTodolists(todolistCtx, year)
		if err != nil {
			s.tracker.FailTrace(recType, err)
			s.display.PrintError("fetching todolists: %v", err)
			return err
		}

		s.tracker.StartSpan(recType, trace.SpanFilter)
		filtered := activity.FilterTodolists(res, profile.ID, year)
		s.tracker.SetTotalItemsAfterFilter(recType, len(filtered))
		s.tracker.CompleteSpan(recType)
		s.tracker.CompleteTrace(recType)

		userActivity.AddTodolists(filtered)
		return nil
	})

	g.Go(func() error {
		recType := trace.KanbanCards
		cardCtx := trace.WithResourceType(gCtx, recType)

		res, err := s.basecamp.GetKanbanCards(cardCtx, year)
		if err != nil {
			s.tracker.FailTrace(recType, err)
			s.display.PrintError("fetching cards: %v", err)
			return err
		}

		s.tracker.StartSpan(recType, trace.SpanFilter)
		created, assigned := activity.FilterKanbanCards(res, profile.ID, year)
		s.tracker.SetTotalItemsAfterFilter(recType, len(created)+len(assigned))
		s.tracker.CompleteSpan(recType)
		s.tracker.CompleteTrace(recType)

		userActivity.AddKanbanCards(created, assigned)
		return nil
	})

	g.Go(func() error {
		recType := trace.Comments
		commentCtx := trace.WithResourceType(gCtx, recType)

		res, err := s.basecamp.GetComments(commentCtx, year)
		if err != nil {
			s.tracker.FailTrace(recType, err)
			s.display.PrintError("fetching comments: %v", err)
			return err
		}

		s.tracker.StartSpan(recType, trace.SpanFilter)
		filtered := activity.FilterComments(res, profile.ID, year)
		s.tracker.SetTotalItemsAfterFilter(recType, len(filtered))
		s.tracker.CompleteSpan(recType)
		s.tracker.CompleteTrace(recType)

		userActivity.AddComments(filtered)
		return nil
	})

	g.Go(func() error {
		recType := trace.QuestionAnswers
		qnaCtx := trace.WithResourceType(gCtx, recType)

		res, err := s.basecamp.GetQuestionAnswers(qnaCtx, year)
		if err != nil {
			s.tracker.FailTrace(recType, err)
			s.display.PrintError("fetching answers: %v", err)
			return err
		}

		s.tracker.StartSpan(recType, trace.SpanFilter)
		filtered := activity.FilterQuestionAnswers(res, profile.ID, year)
		s.tracker.SetTotalItemsAfterFilter(recType, len(filtered))
		s.tracker.CompleteSpan(recType)
		s.tracker.CompleteTrace(recType)

		userActivity.AddQuestionAnswers(filtered)
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err // return the first error encountered
	}

	return userActivity, nil
}
