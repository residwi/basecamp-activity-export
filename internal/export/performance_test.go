package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/residwi/bcreview/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportPerformanceReview(t *testing.T) {
	t.Run("success with full activity", func(t *testing.T) {
		activity := createTestUserActivity()

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "performance_review.md")

		err := ExportPerformanceReview(activity, filename)
		require.NoError(t, err)

		content, err := os.ReadFile(filename)
		require.NoError(t, err)

		output := string(content)

		// verify header section
		assert.Contains(t, output, "# Performance Review - 2024")
		assert.Contains(t, output, "**Employee:** John Doe")
		assert.Contains(t, output, "**Email:** john@example.com")
		assert.Contains(t, output, "**Review Period:** January 1, 2024 - December 31, 2024")

		// verify executive summary
		assert.Contains(t, output, "## Executive Summary")

		// verify metrics table
		assert.Contains(t, output, "### Key Metrics")
		assert.Contains(t, output, "| Metric | Count |")
		assert.Contains(t, output, "| **Total Contributions** |")
		assert.Contains(t, output, "| **Discussions Started** | 1 |")
		assert.Contains(t, output, "| **Tasks Created** | 2 |")
		assert.Contains(t, output, "| **Tasks Completed** | 1 |")

		// verify sections exist
		assert.Contains(t, output, "## Objectives & Epics")
		assert.Contains(t, output, "## Detailed Task Breakdown")
		assert.Contains(t, output, "## Discussions Initiated")
		assert.Contains(t, output, "## Daily Check-ins")

		// verify specific content
		assert.Contains(t, output, "Implement login endpoint")
		assert.Contains(t, output, "OAuth Implementation")
		assert.Contains(t, output, "API Design Discussion")

		// verify links are present
		assert.Contains(t, output, "[View Task](https://3.basecampapi.com/123/todo/301)")
		assert.Contains(t, output, "[View Card](https://3.basecampapi.com/123/card/501)")

		// verify HTML was converted to Markdown
		assert.Contains(t, output, "**API**") // <strong> converted to **
		assert.NotContains(t, output, "<p>")
		assert.NotContains(t, output, "</p>")
		assert.NotContains(t, output, "<strong>")
	})

	t.Run("empty activity generates minimal report", func(t *testing.T) {
		activity := &models.UserActivity{
			User: models.Person{
				ID:           1,
				Name:         "Empty User",
				EmailAddress: "empty@example.com",
			},
			Year: 2024,
		}

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "empty_review.md")

		err := ExportPerformanceReview(activity, filename)
		require.NoError(t, err)

		content, err := os.ReadFile(filename)
		require.NoError(t, err)

		output := string(content)

		// still have header and metrics
		assert.Contains(t, output, "# Performance Review - 2024")
		assert.Contains(t, output, "**Employee:** Empty User")
		assert.Contains(t, output, "### Key Metrics")

		// should not have section headers for empty sections
		assert.NotContains(t, output, "## Objectives & Epics")
	})

	t.Run("file creation error in non-existent directory", func(t *testing.T) {
		activity := createTestUserActivity()

		invalidPath := "/nonexistent/directory/review.md"

		err := ExportPerformanceReview(activity, invalidPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "creating file")
	})

	t.Run("complete workflow with all sections", func(t *testing.T) {
		activity := createTestUserActivity()

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "complete_review.md")

		err := ExportPerformanceReview(activity, filename)
		require.NoError(t, err)

		content, err := os.ReadFile(filename)
		require.NoError(t, err)
		output := string(content)

		sections := []string{
			"# Performance Review",
			"## Executive Summary",
			"### Key Metrics",
			"## Objectives & Epics",
			"## Detailed Task Breakdown",
			"### Tasks I Created",
			"### Tasks Assigned to Me",
			"### Todolists I Created",
			"### Cards I Created",
			"### Cards Assigned to Me",
			"## Discussions Initiated",
			"## Daily Check-ins",
			"## About This Report",
		}

		for _, section := range sections {
			assert.Contains(t, output, section, "Missing section: %s", section)
		}

		assert.Contains(t, output, "**My Comments:**")
		assert.Contains(t, output, "Good progress!")
		assert.Contains(t, output, "Needs more tests")

		assert.Contains(t, output, "`authentication`") // <code> to `code`
		assert.NotContains(t, output, "<code>")

		assert.Contains(t, output, "January")

		// verify todos are sorted chronologically
		todosSection := strings.Split(output, "### Tasks I Created")[1]
		loginIdx := strings.Index(todosSection, "Implement login endpoint")
		testsIdx := strings.Index(todosSection, "Write unit tests")
		assert.True(t, loginIdx < testsIdx, "Todos should be sorted by creation date")
	})

	t.Run("handles nil dates gracefully", func(t *testing.T) {
		activity := &models.UserActivity{
			User: models.Person{
				ID:           1,
				Name:         "Test User",
				EmailAddress: "test@example.com",
			},
			Year: 2024,
			TodosCreated: []models.Todo{
				{
					Recording: models.Recording{
						ID:        1,
						Title:     "Task without due date",
						CreatedAt: time.Now(),
						Bucket:    models.Bucket{ID: 1, Name: "Project"},
						AppURL:    "https://example.com/todo/1",
					},
					DueOn: nil, // No due date
				},
			},
			CardsCreated: []models.Card{
				{
					Recording: models.Recording{
						ID:        1,
						Title:     "Card without due date",
						CreatedAt: time.Now(),
						Bucket:    models.Bucket{ID: 1, Name: "Project"},
						AppURL:    "https://example.com/card/1",
					},
					DueOn:  nil, // No due date
					Parent: models.Parent{Title: "Epic"},
				},
			},
		}

		tempDir := t.TempDir()
		filename := filepath.Join(tempDir, "nil_dates_review.md")

		err := ExportPerformanceReview(activity, filename)
		require.NoError(t, err)

		content, err := os.ReadFile(filename)
		require.NoError(t, err)
		output := string(content)

		// should not have "Due:" lines for items without due dates
		assert.NotContains(t, output, "**Due:** <nil>")

		// but should still have the items
		assert.Contains(t, output, "Task without due date")
		assert.Contains(t, output, "Card without due date")
	})
}

func TestGroupCardsByEpic(t *testing.T) {
	t.Run("groups cards by parent epic", func(t *testing.T) {
		activity := createTestUserActivity()

		epics := groupCardsByEpic(activity)

		require.Len(t, epics, 1)

		epic := epics[0]
		assert.Equal(t, "Epic: User Authentication", epic.Title)
		assert.Equal(t, "Vault::CardTable::Tray", epic.Description)
		assert.Equal(t, "https://3.basecampapi.com/123/card_table/100", epic.AppURL)

		assert.Len(t, epic.Cards, 2)

		cardTitles := []string{epic.Cards[0].Title, epic.Cards[1].Title}
		assert.Contains(t, cardTitles, "OAuth Implementation")
		assert.Contains(t, cardTitles, "JWT Token Validation")
	})

	t.Run("returns empty slice when no cards exist", func(t *testing.T) {
		activity := &models.UserActivity{
			User: models.Person{ID: 1, Name: "Test User"},
			Year: 2024,
		}

		epics := groupCardsByEpic(activity)
		assert.Empty(t, epics)
	})

	t.Run("handles cards with empty parent title", func(t *testing.T) {
		activity := &models.UserActivity{
			User: models.Person{ID: 1, Name: "Test User"},
			Year: 2024,
			CardsCreated: []models.Card{
				{
					Recording: models.Recording{
						ID:        1,
						Title:     "Card without parent title",
						CreatedAt: time.Now(),
						Bucket:    models.Bucket{ID: 1, Name: "Project"},
					},
					Parent: models.Parent{Title: ""}, // Empty parent title
				},
			},
		}

		epics := groupCardsByEpic(activity)

		require.Len(t, epics, 1)
		assert.Equal(t, "", epics[0].Title)
	})

	t.Run("sorts epics alphabetically by title", func(t *testing.T) {
		now := time.Now()
		activity := &models.UserActivity{
			User: models.Person{ID: 1, Name: "Test User"},
			Year: 2024,
			CardsCreated: []models.Card{
				{
					Recording: models.Recording{
						ID:        1,
						Title:     "Card Z",
						CreatedAt: now,
						Bucket:    models.Bucket{ID: 1},
					},
					Parent: models.Parent{Title: "Epic Z"},
				},
				{
					Recording: models.Recording{
						ID:        2,
						Title:     "Card A",
						CreatedAt: now,
						Bucket:    models.Bucket{ID: 1},
					},
					Parent: models.Parent{Title: "Epic A"},
				},
				{
					Recording: models.Recording{
						ID:        3,
						Title:     "Card M",
						CreatedAt: now,
						Bucket:    models.Bucket{ID: 1},
					},
					Parent: models.Parent{Title: "Epic M"},
				},
			},
		}

		epics := groupCardsByEpic(activity)

		require.Len(t, epics, 3)
		assert.Equal(t, "Epic A", epics[0].Title)
		assert.Equal(t, "Epic M", epics[1].Title)
		assert.Equal(t, "Epic Z", epics[2].Title)
	})
}

func TestGetCommentsForRecording(t *testing.T) {
	t.Run("filters and sorts comments for recording and user", func(t *testing.T) {
		now := time.Now()
		userID := int64(123)
		recordingID := int64(301)

		comments := []models.Comment{
			{
				ID:        1,
				Content:   "Comment 1",
				CreatedAt: now.AddDate(0, 0, -2),
				Creator:   models.Creator{ID: userID},
				Parent:    models.Parent{ID: recordingID},
			},
			{
				ID:        2,
				Content:   "Comment 2",
				CreatedAt: now.AddDate(0, 0, -1),
				Creator:   models.Creator{ID: userID},
				Parent:    models.Parent{ID: recordingID},
			},
			{
				ID:        3,
				Content:   "Comment 3 (different user)",
				CreatedAt: now,
				Creator:   models.Creator{ID: 456}, // Different user
				Parent:    models.Parent{ID: recordingID},
			},
			{
				ID:        4,
				Content:   "Comment 4 (different recording)",
				CreatedAt: now,
				Creator:   models.Creator{ID: userID},
				Parent:    models.Parent{ID: 999}, // Different recording
			},
		}

		filtered := getCommentsForRecording(comments, recordingID, userID)

		require.Len(t, filtered, 2)
		assert.Equal(t, int64(1), filtered[0].ID)
		assert.Equal(t, int64(2), filtered[1].ID)

		assert.True(t, filtered[0].CreatedAt.Before(filtered[1].CreatedAt))
	})

	t.Run("returns empty slice when no comments provided", func(t *testing.T) {
		comments := []models.Comment{}
		filtered := getCommentsForRecording(comments, 123, 456)
		assert.Empty(t, filtered)
	})

	t.Run("returns empty slice when no matching comments", func(t *testing.T) {
		comments := []models.Comment{
			{
				ID:      1,
				Creator: models.Creator{ID: 123},
				Parent:  models.Parent{ID: 456},
			},
		}

		filtered := getCommentsForRecording(comments, 999, 123)
		assert.Empty(t, filtered)

		filtered = getCommentsForRecording(comments, 456, 999)
		assert.Empty(t, filtered)
	})
}

func createTestUserActivity() *models.UserActivity {
	now := time.Now()
	user := models.Person{
		ID:           123,
		Name:         "John Doe",
		EmailAddress: "john@example.com",
	}

	bucket := models.Bucket{
		ID:   1,
		Name: "Test Project",
	}

	parent := models.Parent{
		ID:     100,
		Title:  "Epic: User Authentication",
		Type:   "Vault::CardTable::Tray",
		AppURL: "https://3.basecampapi.com/123/card_table/100",
	}

	dueDate := &models.Date{Time: now.AddDate(0, 0, 7)}

	completedTodo := models.Todo{
		Recording: models.Recording{
			ID:        301,
			Title:     "Implement login endpoint",
			CreatedAt: now.AddDate(0, 0, -5),
			Bucket:    bucket,
			AppURL:    "https://3.basecampapi.com/123/todo/301",
		},
		Description: "<p>Create POST /api/login endpoint</p>",
		Completed:   true,
		DueOn:       dueDate,
		Completion: &models.Completion{
			CreatedAt: &now,
		},
		Assignees: []models.Person{
			{ID: 123, Name: "John Doe"},
		},
	}

	return &models.UserActivity{
		User: user,
		Year: 2024,
		MessagesCreated: []models.Message{
			{
				Recording: models.Recording{
					ID:        201,
					Title:     "API Design Discussion",
					Content:   "<p>Let's discuss the <strong>API</strong> design</p>",
					CreatedAt: now,
					Bucket:    bucket,
					AppURL:    "https://3.basecampapi.com/123/message/201",
				},
				Subject: "REST API Design",
			},
		},
		TodosCreated: []models.Todo{
			completedTodo,
			{
				Recording: models.Recording{
					ID:        302,
					Title:     "Write unit tests",
					CreatedAt: now.AddDate(0, 0, -3),
					Bucket:    bucket,
					AppURL:    "https://3.basecampapi.com/123/todo/302",
				},
				Description: "",
				Completed:   false,
			},
		},
		TodosCompleted: []models.Todo{
			completedTodo,
		},
		TodosAssigned: []models.Todo{
			{
				Recording: models.Recording{
					ID:        303,
					Title:     "Code review",
					CreatedAt: now,
					Bucket:    bucket,
					AppURL:    "https://3.basecampapi.com/123/todo/303",
				},
				Completed: false,
			},
		},
		TodolistsCreated: []models.Todolist{
			{
				Recording: models.Recording{
					ID:        401,
					Title:     "Authentication Tasks",
					Content:   "<ul><li>Login</li><li>Logout</li></ul>",
					CreatedAt: now.AddDate(0, 0, -10),
					Bucket:    bucket,
					AppURL:    "https://3.basecampapi.com/123/todolist/401",
				},
				CompletedRatio: "2/3",
			},
		},
		CardsCreated: []models.Card{
			{
				Recording: models.Recording{
					ID:        501,
					Title:     "OAuth Implementation",
					Content:   "<p>Implement OAuth 2.0 flow</p>",
					CreatedAt: now.AddDate(0, 0, -15),
					Bucket:    bucket,
					AppURL:    "https://3.basecampapi.com/123/card/501",
				},
				DueOn:  dueDate,
				Parent: parent,
				Assignees: []models.Person{
					{ID: 123, Name: "John Doe"},
					{ID: 456, Name: "Jane Smith"},
				},
			},
		},
		CardsAssigned: []models.Card{
			{
				Recording: models.Recording{
					ID:        502,
					Title:     "JWT Token Validation",
					Content:   "<p>Validate JWT tokens</p>",
					CreatedAt: now.AddDate(0, 0, -12),
					Bucket:    bucket,
					AppURL:    "https://3.basecampapi.com/123/card/502",
				},
				Parent: parent,
			},
		},
		CommentsWritten: []models.Comment{
			{
				ID:        601,
				Content:   "<p>Good progress!</p>",
				CreatedAt: now.AddDate(0, 0, -2),
				Creator:   models.Creator{ID: 123},
				Parent:    models.Parent{ID: 301}, // Comment on Todo 301
			},
			{
				ID:        602,
				Content:   "<p>Needs more tests</p>",
				CreatedAt: now.AddDate(0, 0, -1),
				Creator:   models.Creator{ID: 123},
				Parent:    models.Parent{ID: 501}, // Comment on Card 501
			},
		},
		QuestionAnswers: []models.QuestionAnswer{
			{
				Recording: models.Recording{
					ID:        701,
					Title:     "What did you work on today?",
					Content:   "<p>Worked on <code>authentication</code> feature</p>",
					CreatedAt: now,
					Bucket:    bucket,
					AppURL:    "https://3.basecampapi.com/123/question/701",
				},
			},
		},
	}
}
