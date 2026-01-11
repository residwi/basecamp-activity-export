package tui

// SpinnerFrames for visual feedback (plain ASCII)
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ANSI control codes (no colors, just cursor control)
const (
	ClearLine     = "\033[2K"
	ClearScreen   = "\033[2J"
	MoveCursorUp  = "\033[%dA"
	SaveCursor    = "\033[s"
	RestoreCursor = "\033[u"
	HideCursor    = "\033[?25l"
	ShowCursor    = "\033[?25h"
)

// DisplayConfig controls display behavior
type DisplayConfig struct {
	MaxRequestsPerResource int
}

// DefaultDisplayConfig provides sensible defaults
var DefaultDisplayConfig = DisplayConfig{
	MaxRequestsPerResource: 3,
}
