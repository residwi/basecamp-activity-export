package progress

import (
	"strings"
	"time"
)

const (
	// ANSI control codes (no colors, just cursor control)
	clearLine   = "\033[2K"
	clearAndUp  = "\r" + clearLine + "\033[1A"
	clearReturn = "\r" + clearLine + "\r"

	// clearScreen   = "\033[2J"
	// moveCursorUp  = "\033[%dA"
	// saveCursor    = "\033[s"
	// restoreCursor = "\033[u"
	// hideCursor    = "\033[?25l"
	// showCursor    = "\033[?25h"
)

// config controls display behavior
type config struct {
	maxRequestsPerTrace int
	bufferSize          int
	refreshRate         time.Duration
	separator           string
	maxURLLength        int
}

// defaultConfig provides sensible defaults
var defaultConfig = config{
	maxRequestsPerTrace: 3,
	bufferSize:          8192,
	refreshRate:         50 * time.Millisecond, // 20 FPS
	separator:           strings.Repeat("=", 70),
	maxURLLength:        50,
}

// spinnerFrames for visual feedback (plain ASCII)
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
