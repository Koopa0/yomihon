package render

import (
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/koopa0/yomihon/internal/wording"
)

// neutralTaskMarkers are Obsidian's progress, cancellation and forwarding
// markers. They preserve the author's character without claiming completion
// or interpreting a theme's vocabulary of icons.
const neutralTaskMarkers = "/->"

const taskMarkerAttr = "yomihonTaskMarker"

type neutralTaskParser struct{}

func (neutralTaskParser) Trigger() []byte { return []byte{'['} }

func (neutralTaskParser) Parse(parent ast.Node, block text.Reader, _ parser.Context) ast.Node {
	item, ok := parent.Parent().(*ast.ListItem)
	if !ok || item.FirstChild() != parent || parent.HasChildren() {
		return nil
	}
	line, _ := block.PeekLine()
	if len(line) < 3 || line[0] != '[' || line[2] != ']' || !strings.ContainsRune(neutralTaskMarkers, rune(line[1])) {
		return nil
	}
	if len(line) > 3 && !util.IsSpace(line[3]) {
		return nil
	}
	consumed := 3
	for consumed < len(line) && util.IsSpace(line[consumed]) {
		consumed++
	}
	block.Advance(consumed)
	node := east.NewTaskCheckBox(false)
	node.SetAttributeString(taskMarkerAttr, string(line[1]))
	return node
}

func (neutralTaskParser) CloseBlock(ast.Node, parser.Context) {}

// taskListRenderer keeps the checkbox and its first block's inline content in
// one native label. Closing at that block rather than at the list item keeps
// subsequent paragraphs and nested tasks outside the control's name. Labels
// need no ids, so anchor-free excerpts and repeated embeds retain their names.
type taskListRenderer struct {
	paragraph renderer.NodeRendererFunc
	textBlock renderer.NodeRendererFunc
}

func newTaskListRenderer() taskListRenderer {
	defaults := defaultRenderers{}
	goldmarkhtml.NewRenderer().RegisterFuncs(defaults)
	return taskListRenderer{paragraph: defaults[ast.KindParagraph], textBlock: defaults[ast.KindTextBlock]}
}

func (r taskListRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(east.KindTaskCheckBox, renderTaskCheckBox)
	reg.Register(ast.KindParagraph, r.renderTaskBlock)
	reg.Register(ast.KindTextBlock, r.renderTaskBlock)
}

func renderTaskCheckBox(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n, ok := node.(*east.TaskCheckBox)
	if !ok {
		return ast.WalkContinue, nil
	}
	if parent := n.Parent(); parent.Kind() == ast.KindParagraph || parent.Kind() == ast.KindTextBlock {
		if err := writeStrings(w, `<label class="y-task">`); err != nil {
			return ast.WalkStop, err
		}
	}
	if err := writeStrings(w, `<input `); err != nil {
		return ast.WalkStop, err
	}
	if n.IsChecked {
		if err := writeStrings(w, `checked="" `); err != nil {
			return ast.WalkStop, err
		}
	}
	if err := writeStrings(w, `disabled="" type="checkbox"`); err != nil {
		return ast.WalkStop, err
	}
	if marker, present := n.AttributeString(taskMarkerAttr); present {
		if value, known := marker.(string); known {
			if err := writeStrings(w, ` data-task="`, html.EscapeString(value), `"`); err != nil {
				return ast.WalkStop, err
			}
		}
	}
	if err := writeStrings(w, "> "); err != nil {
		return ast.WalkStop, err
	}

	return ast.WalkContinue, nil
}

func (r taskListRenderer) renderTaskBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering && node.FirstChild() != nil && node.FirstChild().Kind() == east.KindTaskCheckBox {
		if err := writeStrings(w, "</label>"); err != nil {
			return ast.WalkStop, err
		}
	}
	if node.Kind() == ast.KindParagraph {
		return r.paragraph(w, source, node, entering)
	}
	return r.textBlock(w, source, node, entering)
}

type taskListExtension struct{}

func (taskListExtension) Extend(markdown goldmark.Markdown) {
	markdown.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(neutralTaskParser{}, -100)))
	markdown.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(newTaskListRenderer(), 200)))
}

// taskLabelMarkup sees only first-party labels before embed markers are
// redeemed. Authored labels are escaped, and expanded child tasks are still
// held outside the rendered document in the marker table.
var (
	taskLabelMarkup = regexp.MustCompile(`(?s)<label class="y-task">(<input [^>]*>)(.*?)</label>`)
	taskImageAlt    = regexp.MustCompile(`<img [^>]*alt="([^"]*)"[^>]*>`)
)

// nameTaskLabels keeps flow content and nested controls outside a label.
// Its name reads only this item's inline words: block markers are removed before
// the existing inline redemption and visible-word fold. The original body is
// then left intact for the ordinary, single-pass embed substitution. The same
// fold detects names made empty by rendered character references or inert tags.
func nameTaskLabels(body string, inline []string, lang wording.Lang) string {
	const opener = `<label class="y-task">`
	const closer = `</label>`
	var named strings.Builder
	for body != "" {
		start := strings.Index(body, opener)
		if start < 0 {
			named.WriteString(body)
			break
		}
		named.WriteString(body[:start])
		tail := body[start+len(opener):]
		closeAt := strings.Index(tail, closer)
		end := len(tail)
		if closeAt >= 0 {
			end = closeAt
		}
		for _, boundary := range []string{"<label", "</li>"} {
			if next := strings.Index(tail, boundary); next >= 0 && next < end {
				end = next
				closeAt = -1
			}
		}
		if closeAt < 0 {
			named.WriteString(opener)
			named.WriteString(tail[:end])
			body = tail[end:]
			continue
		}
		end += len(closer)
		named.WriteString(nameTaskLabel(opener+tail[:end], inline, lang))
		body = tail[end:]
	}
	return named.String()
}

func nameTaskLabel(label string, inline []string, lang wording.Lang) string {
	label = stripListRowRole(label)
	parts := taskLabelMarkup.FindStringSubmatch(label)
	if len(parts) != 3 {
		return label
	}
	own := parts[2]
	block := blockMarkupMarker.MatchString(own)
	words := blockMarkupMarker.ReplaceAllString(own, " ")
	words = substituteBlocks(words, nil, inline)
	// The first-party image renderer escapes its alt attribute. Keep that
	// authored wording while the shared fold removes the remaining markup.
	words = taskImageAlt.ReplaceAllString(words, "$1")
	words = strings.Join(strings.Fields(headingInnerText(words)), " ")
	language := ""
	if words == "" {
		words = wording.TaskWithoutText.In(lang)
		language = ` lang="` + lang.Tag() + `"`
		if !block {
			return `<label class="y-task">` + parts[1] + ` <span` + language + `>` + html.EscapeString(words) + `</span>` + strings.TrimSpace(own) + `</label>`
		}
	}
	if !block {
		return label
	}
	return `<label class="y-task">` + parts[1] + ` <span class="y-offscreen"` + language + `>` + html.EscapeString(words) + `</span></label>` + own
}
