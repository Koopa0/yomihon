// Package vault reads notes from the Obsidian vault on disk. The vault is
// the single source of truth; nothing in this package ever writes to it.
//
// Reading is fault-tolerant by contract: a broken frontmatter block
// yields a note with a diagnostic attached, never an error that would stop
// rendering.
package vault

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Note is one markdown file, frontmatter split from body.
type Note struct {
	RelPath     string
	Frontmatter map[string]any
	// FMDiagnostic is non-empty when the frontmatter block exists but is not
	// valid YAML. Display-only: yomihon reports the fault; a human edits the file.
	FMDiagnostic string
	Body         string
	// BodyLine is the 1-based file line Body begins on — 1 for a note with no
	// frontmatter — so a parser reading the body can report the lines an
	// editor shows rather than lines counted from the split.
	BodyLine int
}

// Parse splits raw file bytes into frontmatter and body and decodes the
// frontmatter. rel is stored on the returned Note as-is (callers pass a
// slash-form vault-relative path). This is the one place that decides what a
// captured note's frontmatter means, so read and write projections cannot
// disagree about the current status.
func Parse(rel string, data []byte) *Note {
	n := &Note{RelPath: rel}
	block, found := SplitFrontmatter(data)
	n.Body = string(block.Body)
	n.BodyLine = block.BodyStartLine
	if !found {
		return n
	}
	content := block.Content
	// The yaml parser numbers lines from the first byte it is handed.
	// Prefixing the newlines that precede the block in the file makes it
	// count in the file's geometry rather than the block's, so the line
	// numbers in the diagnostic are the file's own — where the parser
	// places a fault exactly, that is the line an editor shows. Leading
	// blank lines carry no YAML meaning, so a block that decodes cleanly
	// decodes identically.
	if newlines := bytes.Count(data[:block.ContentStart], []byte("\n")); newlines > 0 {
		content = slices.Concat(bytes.Repeat([]byte("\n"), newlines), block.Content)
	}
	var fields map[string]any
	if err := yaml.Unmarshal(content, &fields); err != nil {
		n.FMDiagnostic = fmt.Sprintf("frontmatter is not valid YAML: %v", err)
		return n
	}
	n.Frontmatter = fields
	return n
}

// Title is the frontmatter title, falling back to the filename stem.
func (n *Note) Title() string {
	if t, ok := n.Frontmatter["title"].(string); ok && t != "" {
		return t
	}
	base := filepath.Base(filepath.FromSlash(n.RelPath))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Status is the frontmatter status, empty when absent (legal for e.g.
// drills: the contract says no_frontmatter_is_legal).
func (n *Note) Status() string {
	if s, ok := n.Frontmatter["status"].(string); ok {
		return s
	}
	return ""
}

// Type is the frontmatter type, empty when absent.
func (n *Note) Type() string {
	if t, ok := n.Frontmatter["type"].(string); ok {
		return t
	}
	return ""
}

// Domain is the frontmatter domain, empty when absent.
func (n *Note) Domain() string {
	if d, ok := n.Frontmatter["domain"].(string); ok {
		return d
	}
	return ""
}

// Slug is the frontmatter slug, empty when absent. It is a lesson's stable
// identity (jp-minna-lNN) and the join key to its slot sidecar — the
// filename is never that key (lesson filenames carry a human title and are
// not derivable from the sidecar's name).
func (n *Note) Slug() string {
	if s, ok := n.Frontmatter["slug"].(string); ok {
		return s
	}
	return ""
}

// FrontmatterSplit is the byte-level split of one note. Content is the YAML
// between the fence lines, including the newline before the closing fence
// when one is present. ContentStart locates that slice in the original input
// so the status write face can replace one line without rebuilding any
// delimiter or newline.
type FrontmatterSplit struct {
	Content       []byte
	Body          []byte
	ContentStart  int
	BodyStartLine int
}

// SplitFrontmatter separates a leading YAML frontmatter block from the body.
// A block opens with a "---" line and closes with the next "---" or "..."
// line. LF and CRLF are accepted, as is a closing fence at EOF. An unterminated
// opening fence is body text, not a partial block. The split happens before
// body preprocessing, so frontmatter values that resemble body syntax remain
// untouched.
//
// A byte-order mark before the opening fence is stepped over, the same reading
// charity already extended to a CRLF fence. Some editors write one; the note is
// then indistinguishable to a reader from any other, and refusing to see its
// fence cost it everything the fence carries — its title, its type, its place
// in the lifecycle — while every face reported the note as legally having no
// frontmatter at all. The mark is stepped over, never removed: the offsets
// below stay measured against the original bytes, so the writer that replaces
// one status line still leaves every other byte, this one included, as it was.
func SplitFrontmatter(data []byte) (FrontmatterSplit, bool) {
	block := FrontmatterSplit{Body: data, BodyStartLine: 1}
	opening, _ := bytes.CutPrefix(data, []byte("\xef\xbb\xbf"))
	rest, found := bytes.CutPrefix(opening, []byte("---\n"))
	if !found {
		if rest, found = bytes.CutPrefix(opening, []byte("---\r\n")); !found {
			return block, false
		}
	}
	contentStart := len(data) - len(rest)
	line := 1
	for offset := 0; offset < len(rest); {
		raw := rest[offset:]
		advance := len(raw)
		if nl := bytes.IndexByte(raw, '\n'); nl >= 0 {
			advance = nl + 1
		}
		line++
		switch string(bytes.TrimRight(raw[:advance], "\r\n")) {
		case "---", "...":
			return FrontmatterSplit{
				Content:       rest[:offset],
				Body:          rest[offset+advance:],
				ContentStart:  contentStart,
				BodyStartLine: line + 1,
			}, true
		}
		offset += advance
	}
	return block, false
}

// StatusLineSpan locates the single line beginning with "status:" inside
// data's frontmatter block and returns the byte range of the line's text in
// data — its trailing newline excluded, a carriage return before that newline
// included. It reports false when data has no frontmatter block or when the
// block holds any number of such lines other than one. The span is the one
// definition of where a note's status lives: the content identity excises
// exactly these bytes, and the surgical status write replaces exactly these
// bytes, so the two cannot disagree about which line the status is.
func StatusLineSpan(data []byte) (start, end int, ok bool) {
	block, found := SplitFrontmatter(data)
	if !found {
		return 0, 0, false
	}
	offset := 0
	for line := range bytes.SplitSeq(block.Content, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("status:")) {
			if ok {
				return 0, 0, false
			}
			start = block.ContentStart + offset
			end = start + len(line)
			ok = true
		}
		offset += len(line) + 1
	}
	return start, end, ok
}

// StatusValueSpan locates the scalar value inside the frontmatter status line
// and returns the byte range of the value's own text in data — the quotes
// around a quoted value excluded, so the quoting style is not part of it. It
// reports false when data has no single status line, or when the value is any
// shape whose meaning a byte replacement cannot preserve: absent, a sequence
// or mapping, an anchor, an alias, a tag, a block scalar that continues onto
// following lines, or a quoted value with no closing quote.
//
// This narrower span, not the whole line, is the one definition of where a
// note's status lives. The line also carries whatever else its author put
// there — a reason in a trailing comment, a chosen quoting, alignment — and
// that is content like any other byte of the note. Excising the whole line
// from the content identity left all of it outside the check a ruling is bound
// by, and replacing the whole line deleted it; both follow from the same
// span being one byte wider than the value it stands for.
func StatusValueSpan(data []byte) (start, end int, ok bool) {
	lineStart, lineEnd, ok := StatusLineSpan(data)
	if !ok {
		return 0, 0, false
	}
	line := bytes.TrimSuffix(data[lineStart:lineEnd], []byte("\r"))
	rest := line[len("status:"):]

	// Space and tab both separate a key from its value in YAML, so both
	// separate one here: the reader parses a tab-separated line, shows its
	// status and offers its transitions, and a writer that refused it closed
	// the write face on a note nothing else called wrong. A colon with nothing
	// after it does not open a mapping at all.
	spaces := 0
	for spaces < len(rest) && (rest[spaces] == ' ' || rest[spaces] == '\t') {
		spaces++
	}
	if spaces == 0 || spaces == len(rest) {
		return 0, 0, false
	}
	offset, width, ok := scalarValue(rest[spaces:])
	if !ok {
		return 0, 0, false
	}
	valueStart := lineStart + len("status:") + spaces + offset
	return valueStart, valueStart + width, true
}

// scalarValue reports where a status value's own text starts within value —
// the bytes following the separator — and how long it is, or false when the
// value is a shape no byte replacement can preserve.
func scalarValue(value []byte) (offset, width int, ok bool) {
	switch quote := value[0]; {
	case quote == '"' || quote == '\'':
		offset, width, ok = quotedScalar(value, quote)
	case isYAMLIndicator(quote):
		return 0, 0, false
	default:
		width, ok = plainScalar(value)
	}
	if !ok {
		return 0, 0, false
	}
	// A control byte inside the run is a line break the reader honours and
	// this line scan does not, so the two disagree about where the value even
	// ends. A carriage return ending the line is the ordinary Windows note and
	// was taken off before any of this; one in the middle is not that.
	for _, b := range value[offset : offset+width] {
		if b < 0x20 {
			return 0, 0, false
		}
	}
	return offset, width, true
}

// quotedScalar measures the text between a value's quotes. An escape inside
// them means those bytes are not the value, so the line is left to a human
// rather than spliced into.
func quotedScalar(value []byte, quote byte) (offset, width int, ok bool) {
	closing := bytes.IndexByte(value[1:], quote)
	if closing < 0 || !onlyCommentFollows(value[1+closing+1:]) {
		return 0, 0, false
	}
	if bytes.ContainsRune(value[1:1+closing], '\\') {
		return 0, 0, false
	}
	return 1, closing, true
}

// plainScalar measures an unquoted value: it starts where the separator left
// off, and runs up to a comment or the end of the line, with trailing spaces
// left outside it.
func plainScalar(value []byte) (width int, ok bool) {
	plain := value
	// A comment opens at a "#" that follows white space, and YAML counts a tab
	// as white space as readily as a space.
	if at := bytes.IndexAny(plain, " \t"); at >= 0 {
		for i := at; i < len(plain)-1; i++ {
			if (plain[i] == ' ' || plain[i] == '\t') && plain[i+1] == '#' {
				plain = plain[:i]
				break
			}
		}
	}
	plain = bytes.TrimRight(plain, " \t")
	if len(plain) == 0 || bytes.Contains(plain, []byte(": ")) {
		return 0, false
	}
	return len(plain), true
}

// onlyCommentFollows reports whether the tail after a closing quote is what a
// value may legally be followed by: nothing, white space, or white space then
// a comment.
func onlyCommentFollows(tail []byte) bool {
	trimmed := bytes.TrimLeft(tail, " \t")
	if len(trimmed) == 0 {
		return true
	}
	return len(trimmed) < len(tail) && trimmed[0] == '#'
}

// isYAMLIndicator reports whether c opens something other than a plain scalar:
// a collection, an anchor, an alias, a tag, a block scalar, a comment, or one
// of the characters YAML reserves. A status word never begins with one, so
// refusing them all costs nothing and keeps the scan from splicing into a
// value whose text is not the whole of its meaning.
func isYAMLIndicator(c byte) bool {
	return bytes.IndexByte([]byte("-?:,[]{}#&*!|>%@`\"'"), c) >= 0
}
