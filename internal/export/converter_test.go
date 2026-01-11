package export

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertHTMLToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "simple paragraph",
			input:    "<p>Hello World</p>",
			expected: "> Hello World",
		},
		{
			name:     "paragraph with bold",
			input:    "<p>This is <strong>bold</strong> text</p>",
			expected: "> This is **bold** text",
		},
		{
			name:     "paragraph with italic",
			input:    "<p>This is <em>italic</em> text</p>",
			expected: "> This is *italic* text",
		},
		{
			name:     "link",
			input:    "<p>Check <a href=\"https://example.com\">this link</a></p>",
			expected: "> Check [this link](https://example.com)",
		},
		{
			name:     "unordered list",
			input:    "<ul><li>Item 1</li><li>Item 2</li><li>Item 3</li></ul>",
			expected: "> - Item 1\n> - Item 2\n> - Item 3",
		},
		{
			name:     "ordered list",
			input:    "<ol><li>First</li><li>Second</li><li>Third</li></ol>",
			expected: "> 1. First\n> 2. Second\n> 3. Third",
		},
		{
			name:     "code block",
			input:    "<pre><code>func main() {\n  fmt.Println(\"Hello\")\n}</code></pre>",
			expected: "> ```\n> func main() {\n>   fmt.Println(\"Hello\")\n> }\n> ```",
		},
		{
			name:     "inline code",
			input:    "<p>Use <code>fmt.Println()</code> for output</p>",
			expected: "> Use `fmt.Println()` for output",
		},
		{
			name:     "mixed content",
			input:    "<p>This is <strong>important</strong> with <a href=\"https://example.com\">a link</a> and <code>code</code></p>",
			expected: "> This is **important** with [a link](https://example.com) and `code`",
		},
		{
			name:     "line breaks",
			input:    "<p>Line 1<br>Line 2<br/>Line 3</p>",
			expected: "> Line 1  \n> Line 2  \n> Line 3",
		},
		{
			name:     "multiple paragraphs",
			input:    "<p>First paragraph</p><p>Second paragraph</p>",
			expected: "> First paragraph\n> \n> Second paragraph",
		},
		{
			name:     "heading",
			input:    "<h1>Main Title</h1>",
			expected: "> # Main Title",
		},
		{
			name:     "nested lists",
			input:    "<ul><li>Item 1<ul><li>Nested 1</li></ul></li><li>Item 2</li></ul>",
			expected: "> - Item 1\n>   \n>   - Nested 1\n> - Item 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertHTMLToMarkdown(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "simple text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "br tag",
			input:    "Line 1<br>Line 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "br self-closing tag",
			input:    "Line 1<br/>Line 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "br with space",
			input:    "Line 1<br />Line 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "paragraph tags",
			input:    "<p>Paragraph 1</p><p>Paragraph 2</p>",
			expected: "Paragraph 1\n\nParagraph 2",
		},
		{
			name:     "mixed tags",
			input:    "<p>First</p>Text<br>More<br/>text",
			expected: "First\n\nText\nMore\ntext",
		},
		{
			name:     "whitespace trimming",
			input:    "  <p>Content</p>  ",
			expected: "Content",
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
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text no code block",
			input:    "> Simple text",
			expected: "> Simple text",
		},
		{
			name:     "code block in blockquote",
			input:    "> ```\ncode line 1\ncode line 2\n> ```",
			expected: "> ```\n> code line 1\n> code line 2\n> ```",
		},
		{
			name:     "code block already prefixed",
			input:    "> ```\n> code line 1\n> code line 2\n> ```",
			expected: "> ```\n> code line 1\n> code line 2\n> ```",
		},
		{
			name:     "multiple code blocks",
			input:    "> ```\ncode1\n> ```\n> normal\n> ```\ncode2\n> ```",
			expected: "> ```\n> code1\n> ```\n> normal\n> ```\n> code2\n> ```",
		},
		{
			name:     "code block with language",
			input:    "> ```go\nfunc main() {}\n> ```",
			expected: "> ```go\n> func main() {}\n> ```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fixCodeBlockInBlockquote(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertHTMLToMarkdown_BasecampSpecificContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string // check if result contains this string
	}{
		{
			name:     "mention tag",
			input:    "<bc-attachment content-type=\"application/vnd.basecamp.mention\"><img alt=\"John Doe\"/></bc-attachment>",
			contains: "John Doe",
		},
		{
			name:     "image attachment",
			input:    "<bc-attachment content-type=\"image/png\" filename=\"screenshot.png\" url=\"https://example.com/image.png\"></bc-attachment>",
			contains: "![screenshot.png](https://example.com/image.png)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertHTMLToMarkdown(tt.input)
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestConvertHTMLToMarkdown_ErrorFallback(t *testing.T) {
	// test that conversion doesn't panic on malformed HTML
	malformedHTML := "<div><p>Unclosed paragraph<div>Nested unclosed</div>"

	result := ConvertHTMLToMarkdown(malformedHTML)
	assert.NotEmpty(t, result, "Should return something even with malformed HTML")
}

func TestConvertHTMLToMarkdown_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "ampersand",
			input: "<p>This &amp; that</p>",
		},
		{
			name:  "quotes",
			input: "<p>He said &quot;hello&quot;</p>",
		},
		{
			name:  "less than greater than",
			input: "<p>x &lt; 5 &gt; 2</p>",
		},
		{
			name:  "unicode characters",
			input: "<p>Hello 世界 🌍</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertHTMLToMarkdown(tt.input)
			assert.NotEmpty(t, result, "Should handle special characters")
			assert.NotContains(t, result, "<p>", "Should not contain HTML tags")
		})
	}
}
