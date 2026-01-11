package aggregator

import (
	"github.com/residwi/bcreview/internal/tracking"
	"github.com/residwi/bcreview/internal/tui"
)

// Option defines a configuration function for Aggregator
type Option func(*Aggregator)

func WithTracker(t *tracking.Tracker) Option {
	return func(a *Aggregator) { a.tracker = t }
}

func WithDisplay(d *tui.Display) Option {
	return func(a *Aggregator) { a.display = d }
}
