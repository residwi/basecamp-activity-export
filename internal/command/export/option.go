package export

import (
	"github.com/residwi/basecamp-activity-export/internal/basecamp"
	"github.com/residwi/basecamp-activity-export/internal/progress"
	"github.com/residwi/basecamp-activity-export/internal/trace"
)

type optionFunc func(*Export)

func WithBasecampClient(c *basecamp.Client) optionFunc {
	return func(s *Export) {
		s.basecamp = c
	}
}

func WithTracker(t *trace.Tracer) optionFunc {
	return func(s *Export) {
		s.tracker = t
	}
}

func WithFilename(filename string) optionFunc {
	return func(s *Export) {
		s.filename = filename
	}
}

func WithDisplay(d *progress.Display) optionFunc {
	return func(e *Export) {
		e.display = d
	}
}
