package export

import (
	"strings"

	"github.com/JohannesKaufmann/dom"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"golang.org/x/net/html"
)

var conv *converter.Converter

func init() {
	conv = converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
		),
	)

	conv.Register.RendererFor("bc-attachment", converter.TagTypeInline, renderBCAttachment, converter.PriorityStandard)
}

// ConvertHTMLToMarkdown converts HTML content to Markdown format using html-to-markdown library
func ConvertHTMLToMarkdown(html string) string {
	if html == "" {
		return ""
	}

	html = "<blockquote>" + html + "</blockquote>"

	markdown, err := conv.ConvertString(html)
	if err != nil {
		return cleanHTML(html)
	}

	markdown = strings.TrimSpace(markdown)
	markdown = fixCodeBlockInBlockquote(markdown)

	return markdown
}

// cleanHTML is a fallback function that removes HTML tags if conversion fails
func cleanHTML(html string) string {
	cleaned := html
	cleaned = strings.ReplaceAll(cleaned, "<br>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "<br/>", "\n")
	cleaned = strings.ReplaceAll(cleaned, "<br />", "\n")
	cleaned = strings.ReplaceAll(cleaned, "</p>", "\n\n")
	cleaned = strings.ReplaceAll(cleaned, "<p>", "")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}

// renderBCAttachment handles custom rendering for Basecamp attachment nodes
func renderBCAttachment(ctx converter.Context, writer converter.Writer, node *html.Node) converter.RenderStatus {
	contentType := dom.GetAttributeOr(node, "content-type", "")

	if contentType == "application/vnd.basecamp.mention" {
		imgTag := dom.FindFirstNode(node, func(n *html.Node) bool {
			return dom.NodeName(n) == "img"
		})
		if imgTag != nil {
			personName := dom.GetAttributeOr(imgTag, "alt", "")
			writer.WriteString(" " + personName + " ")
		}
	} else if strings.HasPrefix(contentType, "image") {
		url := dom.GetAttributeOr(node, "url", dom.GetAttributeOr(node, "href", ""))
		filename := dom.GetAttributeOr(node, "filename", "")

		writer.WriteString("![")
		writer.WriteString(filename)
		writer.WriteString("](")
		writer.WriteString(url)
		writer.WriteString(")")
	}

	return converter.RenderSuccess
}

func fixCodeBlockInBlockquote(markdown string) string {
	var sb strings.Builder
	sb.Grow(len(markdown) + len(markdown)/10)

	lines := strings.Split(markdown, "\n")
	inCodeBlock := false

	for i, line := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}

		if strings.Contains(line, "```") {
			inCodeBlock = !inCodeBlock
			sb.WriteString(line)
			continue
		}

		if inCodeBlock && !strings.HasPrefix(line, ">") {
			sb.WriteString("> ")
		}
		sb.WriteString(line)
	}

	return sb.String()
}
