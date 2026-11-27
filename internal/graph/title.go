package graph

import (
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"

	"github.com/koopa0/yomihon/internal/vault"
)

// TitleHeading is the first level-1 heading a reader is shown: the plain words
// the page and every list call the note by, and the source lines the heading
// occupies, so the page can leave it out where the title already says it.
type TitleHeading struct {
	Words string
	// Line is the zero-based index of the heading's first source line, and Lines
	// how many it spans: one for a marked heading, the words' lines and the
	// underline for an underlined one.
	Line, Lines int
}

// FirstTitleHeading reads the first level-1 heading of body over the text a
// reader is shown: a heading inside a fence or an authored HTML block is not
// one, and neither is one inside an Obsidian comment or a math block. A heading
// with no words is passed over. The lines it reports are lines of body, since
// what is hidden keeps its line breaks.
func FirstTitleHeading(body string) (TitleHeading, bool) {
	return titleHeadingIn(withoutHiddenText(body))
}

func titleHeadingIn(shown string) (TitleHeading, bool) {
	lines := strings.Split(shown, "\n")
	definitions, read := "", false
	for _, heading := range Headings(shown, LineSkipZones(shown)) {
		if heading.Level != 1 {
			continue
		}
		if !read && strings.Contains(heading.Text, "[^") {
			definitions, read = footnoteDefinitions(shown), true
		}
		words := headingWords(heading.Text, definitions)
		if words == "" {
			continue
		}
		span := 1
		if !ATXHeading.MatchString(lines[heading.Line]) {
			span = strings.Count(heading.Text, "\n") + 2
		}
		return TitleHeading{Words: words, Line: heading.Line, Lines: span}, true
	}
	return TitleHeading{}, false
}

// FirstH1 returns the plain words of the first level-1 heading in body, as
// FirstTitleHeading reads it.
func FirstH1(body string) (string, bool) {
	heading, ok := FirstTitleHeading(body)
	return heading.Words, ok
}

// WrittenTitle is the title a note's author wrote for it: the plain words of
// its first level-1 heading, else its frontmatter title. It is false for a
// note that wrote neither, which a list names by something other than its file.
func WrittenTitle(n *vault.Note) (string, bool) {
	if heading, ok := FirstH1(n.Body); ok {
		return heading, true
	}
	if declared, _ := n.Text("title"); strings.TrimSpace(declared) != "" {
		return strings.Join(strings.Fields(declared), " "), true
	}
	return "", false
}

// TitleOf is the title a note is read by: what WrittenTitle finds, and when the
// author wrote none the file name without its extension.
func TitleOf(n *vault.Note) string {
	if title, ok := WrittenTitle(n); ok {
		return title
	}
	return FileStem(n.RelPath)
}

// FileStem is a note's file name without its directory and its .md extension,
// the title a note with neither heading nor declared title is read by.
func FileStem(relPath string) string {
	return strings.TrimSuffix(filename(relPath), ".md")
}

// withoutHiddenText is body with its %% and HTML comments and its $$ math
// blocks cut out, each leaving the line breaks it covered. The page strips
// comments before it reads a heading, so a title read from the unstripped
// text could name words the page never shows; and the words either side of a
// cut still sit on the lines the author gave them.
func withoutHiddenText(body string) string {
	if !strings.Contains(body, "%%") && !strings.Contains(body, "<!--") && !strings.Contains(body, "$$") {
		return body
	}
	code := CodeZones(ParseCommonMark(body))
	comments := CommentZones(body, code)
	hidden := slices.Concat(comments, mathZones(body, slices.Concat(code, comments)))
	slices.SortFunc(hidden, func(a, b Span) int { return a.Start - b.Start })
	var b strings.Builder
	last := 0
	for _, zone := range hidden {
		if zone.Start < last {
			zone.Start = last
		}
		if zone.Stop <= zone.Start {
			continue
		}
		b.WriteString(body[last:zone.Start])
		b.WriteString(strings.Repeat("\n", strings.Count(body[zone.Start:zone.Stop], "\n")))
		last = zone.Stop
	}
	b.WriteString(body[last:])
	return b.String()
}

// CodeZones are the code blocks and code spans of a parsed body: the places a
// comment, math or link marker is displayed text.
func CodeZones(doc ast.Node) []Span {
	var zones []Span
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the visitor never returns an error
		if entering {
			if zone, ok := codeZone(n); ok {
				zones = append(zones, zone)
			}
		}
		return ast.WalkContinue, nil
	})
	return zones
}

// codeZone is the source span of a code block's lines, or of the text a code
// span holds, and false for any other node.
func codeZone(n ast.Node) (Span, bool) {
	switch n.Kind() {
	case ast.KindFencedCodeBlock, ast.KindCodeBlock:
		lines := n.Lines()
		if lines.Len() == 0 {
			return Span{}, false
		}
		return Span{Start: lines.At(0).Start, Stop: lines.At(lines.Len() - 1).Stop}, true
	case ast.KindCodeSpan:
		first, last := -1, 0
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			part, ok := child.(*ast.Text)
			if !ok {
				continue
			}
			if first < 0 {
				first = part.Segment.Start
			}
			last = part.Segment.Stop
		}
		return Span{Start: first, Stop: last}, first >= 0
	}
	return Span{}, false
}

// mathZones are the $$ … $$ spans of body, in source order. A marker in
// skip (code, a comment) or escaped with a backslash is text, and so is one
// with no partner after it: a lone $$ is never taken to hide the rest of the
// note.
func mathZones(body string, skip []Span) []Span {
	var zones []Span
	for off := 0; ; {
		start := strings.Index(body[off:], "$$")
		if start < 0 {
			return zones
		}
		at := off + start
		off = at + 2
		if In(skip, at) || escapedAt(body, at) {
			continue
		}
		closer := off
		for {
			end := strings.Index(body[closer:], "$$")
			if end < 0 {
				return zones
			}
			closer += end
			if !In(skip, closer) && !escapedAt(body, closer) {
				break
			}
			closer += 2
		}
		zones = append(zones, Span{Start: at, Stop: closer + 2})
		off = closer + 2
	}
}

// escapedAt reports whether the byte at off follows an odd run of
// backslashes, the CommonMark escape.
func escapedAt(body string, off int) bool {
	escapes := 0
	for i := off - 1; i >= 0 && body[i] == '\\'; i-- {
		escapes++
	}
	return escapes%2 == 1
}
