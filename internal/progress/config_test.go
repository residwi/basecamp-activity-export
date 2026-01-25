package progress

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConstants(t *testing.T) {
	t.Run("ANSI control codes", func(t *testing.T) {
		assert.Equal(t, "\033[2K", clearLine)
		assert.Equal(t, "\r\033[2K\033[1A", clearAndUp)
		assert.Equal(t, "\r\033[2K\r", clearReturn)
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		assert.Equal(t, 3, defaultConfig.maxRequestsPerTrace)
		assert.Equal(t, 8192, defaultConfig.bufferSize)
		assert.Equal(t, 50*time.Millisecond, defaultConfig.refreshRate)
		assert.Equal(t, strings.Repeat("=", 70), defaultConfig.separator)
		assert.Equal(t, 50, defaultConfig.maxURLLength)
	})
}

func TestSpinnerFrames(t *testing.T) {
	t.Run("spinner frames are defined", func(t *testing.T) {
		assert.NotEmpty(t, spinnerFrames)
		assert.Len(t, spinnerFrames, 10)

		expectedFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		assert.Equal(t, expectedFrames, spinnerFrames)
	})
}
