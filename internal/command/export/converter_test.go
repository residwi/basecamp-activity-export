package export

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertHTMLToMarkdown(t *testing.T) {
	t.Run("returns empty string for empty input", func(t *testing.T) {
		result := ConvertHTMLToMarkdown("")

		assert.Equal(t, "", result)
	})

	t.Run("converts basic HTML paragraphs", func(t *testing.T) {
		html := "<p>Hello world</p>"

		result := ConvertHTMLToMarkdown(html)

		assert.Contains(t, result, "Hello world")
	})

	t.Run("converts HTML links", func(t *testing.T) {
		html := `<a href="https://example.com">Link</a>`

		result := ConvertHTMLToMarkdown(html)

		assert.Contains(t, result, "[Link](https://example.com)")
	})

	t.Run("converts HTML lists", func(t *testing.T) {
		html := "<ul><li>Item 1</li><li>Item 2</li></ul>"

		result := ConvertHTMLToMarkdown(html)

		assert.Contains(t, result, "Item 1")
		assert.Contains(t, result, "Item 2")
	})

	t.Run("handles code blocks", func(t *testing.T) {
		html := "<pre><code>func main() {}</code></pre>"

		result := ConvertHTMLToMarkdown(html)

		assert.Contains(t, result, "func main()")
	})

	t.Run("handles bc-attachment mention", func(t *testing.T) {
		html := `<bc-attachment content-type="application/vnd.basecamp.mention"><img alt="John Doe"></bc-attachment>`

		result := ConvertHTMLToMarkdown(html)

		assert.Contains(t, result, "John Doe")
	})

	t.Run("handles bc-attachment image", func(t *testing.T) {
		html := `<bc-attachment content-type="image/png" url="https://example.com/image.png" filename="screenshot.png"></bc-attachment>`

		result := ConvertHTMLToMarkdown(html)

		assert.Contains(t, result, "![screenshot.png](https://example.com/image.png)")
	})

	t.Run("handles bc-attachment image with href fallback", func(t *testing.T) {
		html := `<bc-attachment content-type="image/jpeg" href="https://example.com/photo.jpg" filename="photo.jpg"></bc-attachment>`

		result := ConvertHTMLToMarkdown(html)

		assert.Contains(t, result, "![photo.jpg](https://example.com/photo.jpg)")
	})

	t.Run("handles complex nested HTML", func(t *testing.T) {
		html := `<div><p>Text with <strong>bold</strong> and <em>italic</em></p></div>`

		result := ConvertHTMLToMarkdown(html)

		assert.Contains(t, result, "Text with")
		assert.Contains(t, result, "bold")
		assert.Contains(t, result, "italic")
	})

	t.Run("trims whitespace from result", func(t *testing.T) {
		html := "   <p>Trim me</p>   "

		result := ConvertHTMLToMarkdown(html)

		assert.Equal(t, "> Trim me", result)
	})

	t.Run("fallbacks to cleanHTML on conversion error", func(t *testing.T) {
		html := "<invalid><p>Broken HTML</p>"
		result := ConvertHTMLToMarkdown(html)

		assert.Contains(t, result, "Broken HTML")
	})
}

func TestCleanHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "replaces br tags",
			input:    "Line 1<br>Line 2<br/>Line 3<br />Line 4",
			expected: "Line 1\nLine 2\nLine 3\nLine 4",
		},
		{
			name:     "replaces p tags",
			input:    "<p>Paragraph 1</p><p>Paragraph 2</p>",
			expected: "Paragraph 1\n\nParagraph 2",
		},
		{
			name:     "trims whitespace",
			input:    "  <p>Content</p>  ",
			expected: "Content",
		},
		{
			name:     "handles empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanHTML(tt.input)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFixCodeBlockInBlockquote(t *testing.T) {
	t.Run("adds blockquote prefix inside code blocks", func(t *testing.T) {
		input := "> Some text\n```\ncode line 1\ncode line 2\n```\n> More text"

		result := fixCodeBlockInBlockquote(input)

		assert.Contains(t, result, "> code line 1")
		assert.Contains(t, result, "> code line 2")
	})

	t.Run("preserves existing blockquote prefix", func(t *testing.T) {
		input := "> Some text\n```\n> already quoted\n```"

		result := fixCodeBlockInBlockquote(input)

		// should not double-prefix
		assert.NotContains(t, result, "> > already quoted")
	})

	t.Run("handles multiple code blocks", func(t *testing.T) {
		input := "```\nblock1\n```\ntext\n```\nblock2\n```"

		result := fixCodeBlockInBlockquote(input)

		assert.Contains(t, result, "> block1")
		assert.Contains(t, result, "> block2")
	})

	t.Run("handles empty input", func(t *testing.T) {
		result := fixCodeBlockInBlockquote("")

		assert.Equal(t, "", result)
	})

	t.Run("handles no code blocks", func(t *testing.T) {
		input := "> Regular blockquote\n> More text"

		result := fixCodeBlockInBlockquote(input)

		assert.Equal(t, input, result)
	})
}

func BenchmarkConvertHTMLToMarkdown(b *testing.B) {
	html := `<div>
		<p>This is a <strong>test</strong> with <a href="https://example.com">links</a>.</p>
		<ul>
			<li>Item 1</li>
			<li>Item 2</li>
		</ul>
		<pre><code>func main() {
			fmt.Println("Hello")
		}</code></pre>
	</div>`

	for b.Loop() {
		ConvertHTMLToMarkdown(html)
	}
}

func BenchmarkCleanHTML(b *testing.B) {
	html := "<p>Paragraph 1</p><br><p>Paragraph 2</p><br/><p>Paragraph 3</p>"

	for b.Loop() {
		cleanHTML(html)
	}
}
