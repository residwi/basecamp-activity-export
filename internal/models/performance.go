package models

import "time"

// PerformanceMetrics calculates comprehensive metrics for performance reviews
type PerformanceMetrics struct {
	// Activity Metrics
	TotalContributions int
	MessagesCreated    int
	TodosCreated       int
	TodosCompleted     int
	TodolistsCreated   int
	CommentsWritten    int
	QuestionAnswers    int

	// Engagement Metrics
	ProjectsContributed   int
	AvgCommentsPerMessage float64
	TodoCompletionRate    float64

	// Timeline Metrics
	MostActiveMonth   string
	MostActiveProject string
	FirstContribution time.Time
	LastContribution  time.Time

	// Impact Metrics
	DiscussionsStarted  int
	CollaborationsCount int
}

// CalculateMetrics computes performance metrics from user activity
func CalculateMetrics(activity *UserActivity) PerformanceMetrics {
	metrics := PerformanceMetrics{
		MessagesCreated:  len(activity.MessagesCreated),
		TodosCreated:     len(activity.TodosCreated),
		TodosCompleted:   len(activity.TodosCompleted),
		TodolistsCreated: len(activity.TodolistsCreated),
		CommentsWritten:  len(activity.CommentsWritten),
		QuestionAnswers:  len(activity.QuestionAnswers),
	}

	metrics.TotalContributions = metrics.MessagesCreated + metrics.TodosCreated +
		metrics.TodosCompleted + metrics.TodolistsCreated + metrics.CommentsWritten + metrics.QuestionAnswers

	// Calculate completion rate
	if metrics.TodosCreated > 0 {
		metrics.TodoCompletionRate = float64(metrics.TodosCompleted) / float64(metrics.TodosCreated) * 100
	}

	// Find unique projects
	projectMap := make(map[string]bool)
	for _, msg := range activity.MessagesCreated {
		projectMap[msg.Bucket.Name] = true
	}
	for _, todo := range activity.TodosCreated {
		projectMap[todo.Bucket.Name] = true
	}
	for _, todolist := range activity.TodolistsCreated {
		projectMap[todolist.Bucket.Name] = true
	}
	for _, answer := range activity.QuestionAnswers {
		projectMap[answer.Bucket.Name] = true
	}
	for _, comment := range activity.CommentsWritten {
		if comment.Parent.Title != "" {
			projectMap[comment.Parent.Title] = true
		}
	}
	metrics.ProjectsContributed = len(projectMap)

	// Find most active project
	projectCounts := make(map[string]int)
	for _, msg := range activity.MessagesCreated {
		projectCounts[msg.Bucket.Name]++
	}
	for _, todo := range activity.TodosCreated {
		projectCounts[todo.Bucket.Name]++
	}
	for _, todolist := range activity.TodolistsCreated {
		projectCounts[todolist.Bucket.Name]++
	}
	for _, answer := range activity.QuestionAnswers {
		projectCounts[answer.Bucket.Name]++
	}

	maxCount := 0
	for project, count := range projectCounts {
		if count > maxCount {
			maxCount = count
			metrics.MostActiveProject = project
		}
	}

	// Find most active month
	monthCounts := make(map[string]int)
	for _, msg := range activity.MessagesCreated {
		month := msg.CreatedAt.Format("January 2006")
		monthCounts[month]++
	}
	for _, todo := range activity.TodosCreated {
		month := todo.CreatedAt.Format("January 2006")
		monthCounts[month]++
	}
	for _, todolist := range activity.TodolistsCreated {
		month := todolist.CreatedAt.Format("January 2006")
		monthCounts[month]++
	}
	for _, answer := range activity.QuestionAnswers {
		month := answer.CreatedAt.Format("January 2006")
		monthCounts[month]++
	}

	maxCount = 0
	for month, count := range monthCounts {
		if count > maxCount {
			maxCount = count
			metrics.MostActiveMonth = month
		}
	}

	// Find first and last contributions
	var allTimes []time.Time
	for _, msg := range activity.MessagesCreated {
		allTimes = append(allTimes, msg.CreatedAt)
	}
	for _, todo := range activity.TodosCreated {
		allTimes = append(allTimes, todo.CreatedAt)
	}
	for _, todolist := range activity.TodolistsCreated {
		allTimes = append(allTimes, todolist.CreatedAt)
	}
	for _, answer := range activity.QuestionAnswers {
		allTimes = append(allTimes, answer.CreatedAt)
	}
	for _, comment := range activity.CommentsWritten {
		allTimes = append(allTimes, comment.CreatedAt)
	}

	if len(allTimes) > 0 {
		metrics.FirstContribution = allTimes[0]
		metrics.LastContribution = allTimes[0]
		for _, t := range allTimes {
			if t.Before(metrics.FirstContribution) {
				metrics.FirstContribution = t
			}
			if t.After(metrics.LastContribution) {
				metrics.LastContribution = t
			}
		}
	}

	return metrics
}
