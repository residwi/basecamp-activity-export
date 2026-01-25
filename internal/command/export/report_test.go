package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/residwi/basecamp-activity-export/internal/activity"
	"github.com/residwi/basecamp-activity-export/internal/basecamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatAndSaveReport(t *testing.T) {
	t.Run("creates report file with all sections", func(t *testing.T) {
		ua := createTestUserActivity()
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "report.md")

		err := FormatAndSaveReport(ua, filename)

		require.NoError(t, err)

		content, err := os.ReadFile(filename)
		require.NoError(t, err)

		report := string(content)
		assert.Contains(t, report, "# Activity Report - 2025")
		assert.Contains(t, report, "**User:** Test User")
		assert.Contains(t, report, "## Summary")
		assert.Contains(t, report, "## Objectives & Epics")
		assert.Contains(t, report, "## Detailed Task Breakdown")
		assert.Contains(t, report, "## Discussions Initiated")
		assert.Contains(t, report, "## Daily Check-ins")
		assert.Contains(t, report, "## About This Report")
	})

	t.Run("handles empty activity", func(t *testing.T) {
		ua := activity.New(basecamp.Person{ID: 1, Name: "Empty User"}, 2025)
		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "empty_report.md")

		err := FormatAndSaveReport(ua, filename)

		require.NoError(t, err)

		content, err := os.ReadFile(filename)
		require.NoError(t, err)

		report := string(content)
		assert.Contains(t, report, "# Activity Report - 2025")
		assert.NotContains(t, report, "## Discussions Initiated")
		assert.NotContains(t, report, "## Daily Check-ins")
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		ua := activity.New(basecamp.Person{}, 2025)

		err := FormatAndSaveReport(ua, "/nonexistent/path/report.md")
		require.Error(t, err)

		assert.Contains(t, err.Error(), "creating file")
	})
}

func TestPrepareReportData(t *testing.T) {
	ua := createTestUserActivity()

	data := prepareReportData(ua)

	require.NotNil(t, data)
	assert.Equal(t, 2025, data.year)
	assert.Equal(t, "Test User", data.user.Name)

	assert.Len(t, data.messagesCreated, 2)
	assert.Equal(t, int64(2), data.messagesCreated[0].ID)
	assert.Equal(t, int64(1), data.messagesCreated[1].ID)

	assert.Len(t, data.todosCreated, 2)
	assert.Equal(t, int64(1), data.todosCreated[0].ID)
	assert.Equal(t, int64(2), data.todosCreated[1].ID)

	assert.Len(t, data.todosCompleted, 1)

	assert.Len(t, data.todosAssigned, 2)
	assert.Equal(t, int64(1), data.todosAssigned[0].ID)
	assert.Equal(t, int64(4), data.todosAssigned[1].ID)

	assert.Len(t, data.todolistsCreated, 2)
	assert.Equal(t, int64(6), data.todolistsCreated[0].ID)
	assert.Equal(t, int64(5), data.todolistsCreated[1].ID)

	assert.Len(t, data.kanbanCardsCreated, 2)
	assert.Equal(t, int64(8), data.kanbanCardsCreated[0].ID)
	assert.Equal(t, int64(6), data.kanbanCardsCreated[1].ID)

	assert.Len(t, data.kanbanCardsAssigned, 2)
	assert.Equal(t, int64(8), data.kanbanCardsAssigned[0].ID)
	assert.Equal(t, int64(7), data.kanbanCardsAssigned[1].ID)

	assert.Len(t, data.commentsWritten, 2)

	assert.Len(t, data.questionAnswers, 2)
	assert.Equal(t, int64(11), data.questionAnswers[0].ID)
	assert.Equal(t, int64(10), data.questionAnswers[1].ID)

	assert.NotNil(t, data.commentIndex)
	assert.NotEmpty(t, data.epics)
}

func TestGroupKanbanCardsByEpic(t *testing.T) {
	t.Run("groups cards by parent title", func(t *testing.T) {
		cards1 := []basecamp.KanbanCard{
			{Recording: basecamp.Recording{ID: 1}, Parent: basecamp.Parent{Title: "Epic A"}},
			{Recording: basecamp.Recording{ID: 2}, Parent: basecamp.Parent{Title: "Epic B"}},
		}
		cards2 := []basecamp.KanbanCard{
			{Recording: basecamp.Recording{ID: 3}, Parent: basecamp.Parent{Title: "Epic A"}},
		}

		epics := groupKanbanCardsByEpic(cards1, cards2)

		assert.Len(t, epics, 2)
		// epics are sorted alphabetically
		assert.Equal(t, "Epic A", epics[0].Title)
		assert.Len(t, epics[0].Cards, 2)
		assert.Equal(t, "Epic B", epics[1].Title)
		assert.Len(t, epics[1].Cards, 1)
	})

	t.Run("handles empty parent title as Uncategorized", func(t *testing.T) {
		cards := []basecamp.KanbanCard{
			{Recording: basecamp.Recording{ID: 1}, Parent: basecamp.Parent{Title: ""}},
		}

		epics := groupKanbanCardsByEpic(cards, nil)

		assert.Len(t, epics, 1)
		assert.Equal(t, "Uncategorized", epics[0].Title)
	})

	t.Run("returns empty slice for no cards", func(t *testing.T) {
		epics := groupKanbanCardsByEpic(nil, nil)

		assert.Empty(t, epics)
	})
}

func TestSortByDate(t *testing.T) {
	t.Run("sorts in ascending order", func(t *testing.T) {
		items := []basecamp.Message{
			{Recording: basecamp.Recording{ID: 3, CreatedAt: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)}},
			{Recording: basecamp.Recording{ID: 1, CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}},
			{Recording: basecamp.Recording{ID: 2, CreatedAt: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)}},
		}

		sortByDate(items, func(m basecamp.Message) time.Time { return m.CreatedAt })

		assert.Equal(t, int64(1), items[0].ID)
		assert.Equal(t, int64(2), items[1].ID)
		assert.Equal(t, int64(3), items[2].ID)
	})

	t.Run("handles empty slice", func(t *testing.T) {
		var items []basecamp.Message

		sortByDate(items, func(m basecamp.Message) time.Time { return m.CreatedAt })

		assert.Empty(t, items)
	})
}

func TestCalculateMetrics(t *testing.T) {
	t.Run("calculates total contributions", func(t *testing.T) {
		data := &reportData{
			messagesCreated:    make([]basecamp.Message, 2),
			todosCreated:       make([]basecamp.Todo, 3),
			todosCompleted:     make([]basecamp.Todo, 1),
			kanbanCardsCreated: make([]basecamp.KanbanCard, 2),
			commentsWritten:    make([]basecamp.Comment, 4),
			questionAnswers:    make([]basecamp.QuestionAnswer, 1),
		}

		metrics := calculateMetrics(data)

		assert.Equal(t, 13, metrics.totalContributions)
	})

	t.Run("counts unique projects", func(t *testing.T) {
		data := &reportData{
			messagesCreated: []basecamp.Message{
				{Recording: basecamp.Recording{Bucket: basecamp.Bucket{ID: 1}}},
				{Recording: basecamp.Recording{Bucket: basecamp.Bucket{ID: 2}}},
			},
			todosCreated: []basecamp.Todo{
				{Recording: basecamp.Recording{Bucket: basecamp.Bucket{ID: 2}}},
			},
			kanbanCardsCreated: []basecamp.KanbanCard{
				{Recording: basecamp.Recording{Bucket: basecamp.Bucket{ID: 3}}},
				{Recording: basecamp.Recording{Bucket: basecamp.Bucket{ID: 1}}},
			},
		}

		metrics := calculateMetrics(data)

		assert.Equal(t, 3, metrics.projectsContributed)
	})
}

func TestBuildCommentIndex(t *testing.T) {
	userID := int64(123)
	now := time.Now()

	t.Run("filters comments by user ID", func(t *testing.T) {
		comments := []basecamp.Comment{
			{ID: 1, Parent: basecamp.Parent{ID: 10}, Creator: basecamp.Person{ID: userID}, CreatedAt: now},
			{ID: 2, Parent: basecamp.Parent{ID: 10}, Creator: basecamp.Person{ID: 999}, CreatedAt: now},
			{ID: 3, Parent: basecamp.Parent{ID: 20}, Creator: basecamp.Person{ID: userID}, CreatedAt: now},
		}

		idx := buildCommentIndex(comments, userID)

		assert.Len(t, idx[10], 1)
		assert.Len(t, idx[20], 1)
		assert.Equal(t, int64(1), idx[10][0].ID)
	})

	t.Run("sorts comments by date within group", func(t *testing.T) {
		comments := []basecamp.Comment{
			{ID: 2, Parent: basecamp.Parent{ID: 10}, Creator: basecamp.Person{ID: userID}, CreatedAt: now.Add(time.Hour)},
			{ID: 1, Parent: basecamp.Parent{ID: 10}, Creator: basecamp.Person{ID: userID}, CreatedAt: now},
		}

		idx := buildCommentIndex(comments, userID)

		assert.Equal(t, int64(1), idx[10][0].ID)
		assert.Equal(t, int64(2), idx[10][1].ID)
	})

	t.Run("returns empty index for no matching comments", func(t *testing.T) {
		comments := []basecamp.Comment{
			{ID: 1, Creator: basecamp.Person{ID: 999}},
		}

		idx := buildCommentIndex(comments, userID)

		assert.Empty(t, idx)
	})
}

func TestCommentIndex_GetComments(t *testing.T) {
	t.Run("returns comments for recording", func(t *testing.T) {
		idx := commentIndex{
			10: []basecamp.Comment{{ID: 1}, {ID: 2}},
		}

		comments := idx.getComments(10)

		assert.Len(t, comments, 2)
	})

	t.Run("returns nil for unknown recording", func(t *testing.T) {
		idx := commentIndex{}

		comments := idx.getComments(999)

		assert.Nil(t, comments)
	})
}

func TestWriteFunctions(t *testing.T) {
	t.Run("writeAssignees handles empty list", func(t *testing.T) {
		ua := activity.New(basecamp.Person{ID: 1, Name: "User"}, 2025)
		ua.AddTodos([]basecamp.Todo{{
			Recording: basecamp.Recording{
				ID:    1,
				Title: "No Assignees",
			},
			Assignees: []basecamp.Person{},
		}}, nil, nil)

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "report.md")

		err := FormatAndSaveReport(ua, filename)
		require.NoError(t, err)

		content, _ := os.ReadFile(filename)
		assert.NotContains(t, string(content), "**Assigned to:**")
	})

	t.Run("writeComments handles empty list", func(t *testing.T) {
		ua := activity.New(basecamp.Person{ID: 1, Name: "User"}, 2025)
		ua.AddMessages([]basecamp.Message{
			{Recording: basecamp.Recording{ID: 1, Title: "Message without Comments"}},
		})

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "report.md")

		err := FormatAndSaveReport(ua, filename)
		require.NoError(t, err)

		content, _ := os.ReadFile(filename)
		assert.NotContains(t, string(content), "**My Comments:**")
	})

	t.Run("writes message with different subject", func(t *testing.T) {
		ua := activity.New(basecamp.Person{ID: 1, Name: "User"}, 2025)
		ua.AddMessages([]basecamp.Message{
			{Recording: basecamp.Recording{ID: 1, Title: "Title"}, Subject: "Different Subject"},
		})

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "report.md")

		err := FormatAndSaveReport(ua, filename)
		require.NoError(t, err)

		content, _ := os.ReadFile(filename)
		assert.Contains(t, string(content), "**Subject:** Different Subject")
	})

	t.Run("skips subject when same as title", func(t *testing.T) {
		ua := activity.New(basecamp.Person{ID: 1, Name: "User"}, 2025)
		ua.AddMessages([]basecamp.Message{
			{Recording: basecamp.Recording{ID: 1, Title: "Same Title"}, Subject: "Same Title"},
		})

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "report.md")

		err := FormatAndSaveReport(ua, filename)
		require.NoError(t, err)

		content, _ := os.ReadFile(filename)
		assert.Equal(t, 1, strings.Count(string(content), "Same Title"))
	})

	t.Run("writes todo with due date", func(t *testing.T) {
		ua := activity.New(basecamp.Person{ID: 1, Name: "User"}, 2025)
		ua.AddTodos(
			[]basecamp.Todo{
				{
					Recording: basecamp.Recording{
						ID:    1,
						Title: "Todo with Due",
					},
					DueOn: "2025-06-15",
				},
			},
			nil,
			nil,
		)

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "report.md")

		err := FormatAndSaveReport(ua, filename)
		require.NoError(t, err)

		content, _ := os.ReadFile(filename)
		assert.Contains(t, string(content), "**Due:** June 15, 2025")
	})

	t.Run("writes card without parent URL", func(t *testing.T) {
		ua := activity.New(basecamp.Person{ID: 1, Name: "User"}, 2025)
		ua.AddKanbanCards(
			[]basecamp.KanbanCard{{
				Recording: basecamp.Recording{
					ID:    1,
					Title: "Card",
				},
				Parent: basecamp.Parent{Title: "Epic", AppURL: ""},
			}},
			nil,
		)

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "report.md")

		err := FormatAndSaveReport(ua, filename)
		require.NoError(t, err)

		content, _ := os.ReadFile(filename)
		assert.NotContains(t, string(content), "**Card Table Reference:**")
	})
}

func createTestUserActivity() *activity.UserActivity {
	user := basecamp.Person{ID: 123, Name: "Test User", EmailAddress: "test@example.com"}
	ua := activity.New(user, 2025)

	now := time.Date(2025, 5, 1, 10, 0, 0, 0, time.UTC)
	completedAt := now.AddDate(0, 0, -1)

	bucket := basecamp.Bucket{ID: 1, Name: "Test Project"}
	parent := basecamp.Parent{ID: 10, Title: "Epic One", Type: "Kanban::Board", AppURL: "https://basecamp.com/board"}

	ua.AddMessages([]basecamp.Message{
		{
			Recording: basecamp.Recording{
				ID:        1,
				Title:     "Test Message",
				Content:   "<p>Message content</p>",
				Bucket:    bucket,
				Creator:   user,
				CreatedAt: now,
				AppURL:    "https://basecamp.com/message/1",
			},
			Subject: "Different Subject",
		},
		{
			Recording: basecamp.Recording{
				ID:        2,
				Title:     "Another Message",
				Content:   "<p>Another message content</p>",
				Bucket:    bucket,
				Creator:   user,
				CreatedAt: now.AddDate(0, 0, -3),
				AppURL:    "https://basecamp.com/message/2",
			},
			Subject: "Another Message",
		},
	})

	ua.AddTodos(
		[]basecamp.Todo{
			{
				Recording: basecamp.Recording{
					ID:        1,
					Title:     "Created Todo",
					Bucket:    bucket,
					Creator:   user,
					CreatedAt: now.AddDate(0, 0, -2),
					AppURL:    "https://basecamp.com/todo/2",
				},
				Description: "<p>Todo description</p>",
				DueOn:       "2025-05-08",
				Assignees:   []basecamp.Person{{ID: 456, Name: "Assignee"}},
			},
			{
				Recording: basecamp.Recording{
					ID:        2,
					Title:     "Completed Todo",
					Bucket:    bucket,
					Creator:   user,
					CreatedAt: now,
					AppURL:    "https://basecamp.com/todo/3",
				},
				Completed: true,
				Completion: &basecamp.Completion{
					CreatedAt: &completedAt,
				},
				Assignees: []basecamp.Person{{ID: 123, Name: "Test User"}},
			},
		},
		[]basecamp.Todo{{
			Recording: basecamp.Recording{
				ID:        2,
				Title:     "Completed Todo",
				Bucket:    bucket,
				Creator:   user,
				CreatedAt: now,
				AppURL:    "https://basecamp.com/todo/3",
			},
			Completed: true,
			Completion: &basecamp.Completion{
				CreatedAt: &completedAt,
			},
			Assignees: []basecamp.Person{{ID: 123, Name: "Test User"}},
		}},
		[]basecamp.Todo{
			{
				Recording: basecamp.Recording{
					ID:        4,
					Title:     "Assigned Todo",
					Bucket:    bucket,
					Creator:   user,
					CreatedAt: now,
					AppURL:    "https://basecamp.com/todo/4",
				},
				Assignees: []basecamp.Person{{ID: 123, Name: "Test User"}},
			},
			{
				Recording: basecamp.Recording{
					ID:        1,
					Title:     "Created Todo",
					Bucket:    bucket,
					Creator:   user,
					CreatedAt: now.AddDate(0, 0, -2),
					AppURL:    "https://basecamp.com/todo/2",
				},
				Description: "<p>Todo description</p>",
				DueOn:       "2025-05-08",
				Assignees:   []basecamp.Person{{ID: 456, Name: "Assignee"}},
			},
		},
	)

	ua.AddTodolists([]basecamp.Todolist{
		{
			Recording: basecamp.Recording{
				ID:        5,
				Title:     "Test Todolist",
				Content:   "<p>Todolist description</p>",
				Bucket:    bucket,
				Creator:   user,
				CreatedAt: now,
				AppURL:    "https://basecamp.com/todolist/5",
			},
			CompletedRatio: "2/5",
		},
		{
			Recording: basecamp.Recording{
				ID:        6,
				Title:     "Old Todolist",
				Bucket:    bucket,
				Creator:   user,
				CreatedAt: now.AddDate(-1, 0, 0),
				AppURL:    "https://basecamp.com/todolist/6",
			},
			CompletedRatio: "5/5",
		},
	})

	ua.AddKanbanCards(
		[]basecamp.KanbanCard{
			{
				Recording: basecamp.Recording{
					ID:        6,
					Title:     "Created Card",
					Content:   "<p>Card content</p>",
					Bucket:    bucket,
					Creator:   user,
					CreatedAt: now,
					AppURL:    "https://basecamp.com/card/6",
				},
				Parent:    parent,
				DueOn:     "2025-05-08",
				Assignees: []basecamp.Person{{ID: 789, Name: "Card Assignee"}},
			},
			{
				Recording: basecamp.Recording{
					ID:        8,
					Title:     "Old Card",
					Bucket:    bucket,
					Creator:   user,
					CreatedAt: now.AddDate(-1, 0, 0),
					AppURL:    "https://basecamp.com/card/8",
				},
				Parent:    parent,
				Assignees: []basecamp.Person{{ID: 123, Name: "Test User"}},
			},
		},
		[]basecamp.KanbanCard{
			{
				Recording: basecamp.Recording{
					ID:        7,
					Title:     "Assigned Card",
					Bucket:    bucket,
					Creator:   user,
					CreatedAt: now,
					AppURL:    "https://basecamp.com/card/7",
				},
				Parent: parent,
			},
			{
				Recording: basecamp.Recording{
					ID:        8,
					Title:     "Old Card",
					Bucket:    bucket,
					Creator:   user,
					CreatedAt: now.AddDate(-1, 0, 0),
					AppURL:    "https://basecamp.com/card/8",
				},
				Parent:    parent,
				Assignees: []basecamp.Person{{ID: 123, Name: "Test User"}},
			},
		},
	)

	ua.AddComments([]basecamp.Comment{{
		ID:        8,
		Content:   "<p>Comment on message</p>",
		Parent:    basecamp.Parent{ID: 1},
		Creator:   user,
		CreatedAt: now,
	}, {
		ID:        9,
		Content:   "<p>Comment on todo</p>",
		Parent:    basecamp.Parent{ID: 2},
		Creator:   user,
		CreatedAt: now,
	}})

	ua.AddQuestionAnswers([]basecamp.QuestionAnswer{
		{
			Recording: basecamp.Recording{
				ID:        10,
				Title:     "What did you work on?",
				Content:   "<p>I worked on testing</p>",
				Bucket:    bucket,
				Creator:   user,
				CreatedAt: now,
				AppURL:    "https://basecamp.com/checkin/10",
			},
		},
		{
			Recording: basecamp.Recording{
				ID:        11,
				Title:     "Old Check-in",
				Bucket:    bucket,
				Creator:   user,
				CreatedAt: now.AddDate(-1, 0, 0),
				AppURL:    "https://basecamp.com/checkin/11",
			},
		},
	})

	return ua
}
