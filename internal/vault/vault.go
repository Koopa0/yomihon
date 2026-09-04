// Package vault is the note model: what a markdown file in the Obsidian vault
// means once its bytes are in hand. It splits frontmatter from body, reads
// frontmatter values, locates the spans the status write face replaces, and
// holds the one order vault paths sort by. It opens nothing, and it reads
// fault-tolerantly: broken frontmatter yields a diagnostic, never an error.
package vault

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Note is one markdown file, frontmatter split from body.
type Note struct {
	RelPath     string
	Frontmatter map[string]any
	// FMDiagnostic is non-empty when the frontmatter block exists but is not
	// valid YAML. Display-only: yomihon reports, a human edits the file.
	FMDiagnostic string
	Body         string
	// BodyLine is the 1-based file line Body begins on — 1 for a note with no
	// frontmatter — so a body parser cites the lines an editor shows.
	BodyLine int
}

// Parse splits raw file bytes into frontmatter and body and decodes the
// frontmatter into a map. rel is stored as given, in slash form. Broken YAML
// becomes the note's FMDiagnostic rather than an error.
func Parse(rel string, data []byte) *Note {
	n := &Note{RelPath: rel}
	block, found := SplitFrontmatter(data)
	n.Body = string(block.Body)
	n.BodyLine = block.BodyStartLine
	if !found {
		return n
	}
	content := block.Content
	// The yaml parser numbers lines from the first byte it is handed, so the
	// newlines preceding the block go in front to make a fault cite the file's
	// own line. Leading blank lines carry no YAML meaning.
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

// Text reads one frontmatter value the vault writes as text, reporting
// whether the note wrote that key as text at all — a declared but blank field
// is a different state from an absent one. Any other shape answers "", false,
// so a malformed field costs that field and nothing else.
func (n *Note) Text(key string) (string, bool) {
	s, ok := n.Frontmatter[key].(string)
	return s, ok
}

// Strings reads one frontmatter value the vault writes as a list of text, in
// declaration order, dropping a member that is not text. It answers nil when
// the key is absent or holds no list, and an empty slice for a list of no text.
func (n *Note) Strings(key string) []string {
	raw, ok := n.Frontmatter[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Title is the frontmatter title, falling back to the filename stem.
func (n *Note) Title() string {
	if t, _ := n.Text("title"); t != "" {
		return t
	}
	base := filepath.Base(filepath.FromSlash(n.RelPath))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Status is the frontmatter status, empty when absent, which is legal.
func (n *Note) Status() string {
	s, _ := n.Text("status")
	return s
}

// StatusNotText reports that the note wrote a single status value and the YAML
// reader did not hand it back as text: an unquoted date, number or boolean.
// Status is empty in that case, the same as for a note that wrote none, and the
// two are not the same thing to tell a reader — one has nothing to fix and the
// other has a value that needs quoting.
//
// A list or a mapping is deliberately not one of these. The judging side quotes
// a scalar and says nothing about either of those shapes, so a page claiming
// they carry a value the diagnostic below would name is a page contradicting the
// panel under it. They read as no single value, which is what they are.
//
// The judging commands read the same file through their own reader, which
// takes a scalar as the characters the author typed, so they name a value this
// side cannot. A page that says the status could not be read while the panel
// under it quotes the status back is describing one field twice, and this is
// what lets it say which of the two happened instead.
func (n *Note) StatusNotText() bool {
	return n.wroteNonText("status")
}

// wroteNonText reports that key holds one value and that value is not text.
//
// Three shapes answer false, each for its own reason. An absent key wrote
// nothing to misread. An empty one — "status:" with nothing after it — reads as
// null, which is the same nothing, and a reader told to add quotation marks
// there would have nothing to put them around. A list or a mapping did write
// something, but it is not one value, and the sentence this feeds is about a
// value a reader can put quotes around.
func (n *Note) wroteNonText(key string) bool {
	value, present := n.Frontmatter[key]
	if !present || value == nil {
		return false
	}
	switch value.(type) {
	case string, []any, map[string]any:
		return false
	}
	return true
}

// Type is the frontmatter type, empty when absent.
func (n *Note) Type() string {
	t, _ := n.Text("type")
	return t
}

// Domain is the frontmatter domain, empty when absent.
func (n *Note) Domain() string {
	d, _ := n.Text("domain")
	return d
}

// Aliases are the other names this note answers to, in declaration order, and
// nothing when it declared none. Link resolution and search both read these.
func (n *Note) Aliases() []string {
	return n.Strings("aliases")
}

// Updated is the note's declared update date, or the zero time when the
// frontmatter carries none or a shape no date reads from. YAML hands an
// unquoted date over as a time and a quoted one as text, so both are read.
func (n *Note) Updated() time.Time {
	switch v := n.Frontmatter["updated"].(type) {
	case time.Time:
		return v
	case string:
		for _, layout := range []string{time.DateOnly, time.RFC3339} {
			if t, err := time.Parse(layout, v); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// Slug is the frontmatter slug, empty when absent: a lesson's stable identity
// and the join key to its slot sidecar, which the filename never is.
func (n *Note) Slug() string {
	s, _ := n.Text("slug")
	return s
}

// IsMarkdown reports whether relPath ends in the exact extension ".md". The
// match is case-sensitive, so "Note.MD" names a resource rather than a note.
func IsMarkdown(relPath string) bool {
	return strings.HasSuffix(relPath, ".md")
}

// FrontmatterSplit is the byte-level split of one note. Content is the YAML
// between the fence lines; ContentStart locates it in the original input so a
// status write replaces one line without rebuilding a delimiter or a newline.
type FrontmatterSplit struct {
	Content       []byte
	Body          []byte
	ContentStart  int
	BodyStartLine int
}

// SplitFrontmatter separates a leading YAML frontmatter block from the body. A
// block opens with a "---" line and closes with the next "---" or "..." line;
// LF and CRLF are accepted, as is a closing fence at EOF, and an unterminated
// opening fence is body text. A byte-order mark before the fence — some editors
// write one — is stepped over but never removed, so the offsets stay measured
// against the original bytes and a status write disturbs nothing else.
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

// StatusLineSpan locates the single line beginning with "status:" inside data's
// frontmatter block and returns the byte range of the line's text — its
// trailing newline excluded, a carriage return before that newline included. It
// reports false when data has no frontmatter block, or when the block holds any
// number of such lines other than one.
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
// and returns the byte range of the value's own text — the quotes around a
// quoted value excluded, so the quoting style is not part of it. It is the one
// definition of where a note's status lives, narrower than the line so that
// everything else its author wrote there, a trailing comment included, stays
// content. It reports false when data has no single status line, or the value
// is a shape a byte replacement cannot preserve: absent, a sequence or mapping,
// an anchor, an alias, a tag, a multi-line block scalar, an unclosed quote, or
// a value carrying a character the reader ends the line at.
func StatusValueSpan(data []byte) (start, end int, ok bool) {
	lineStart, lineEnd, ok := StatusLineSpan(data)
	if !ok {
		return 0, 0, false
	}
	line := bytes.TrimSuffix(data[lineStart:lineEnd], []byte("\r"))
	rest := line[len("status:"):]

	// A tab separates a key from its value in YAML as readily as a space, and
	// a colon with nothing after it opens no mapping.
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

// scalarValue reports where a status value's own text starts within the bytes
// following the separator and how long it is.
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
	// A line break inside the run is one the YAML reader honours and this line
	// scan does not. The reader ends a line at more than the control bytes:
	// U+0085, U+2028 and U+2029 end one too. In an unquoted value that is a
	// disagreement about where the value stops, and a replacement would reach
	// into what the reader reads as the next line, deleting a key or a comment
	// line of the author's own. Inside quotes the reader ends no line and this
	// refuses anyway: it folds U+0085 to a space there, so the value it reports
	// is not the bytes on the line, and none of the three shows in an editor or
	// a review. A value nobody can check by looking is one to leave alone
	// rather than to rewrite on a guess about which reading was meant.
	for _, r := range string(value[offset : offset+width]) {
		if r < 0x20 || r == '\u0085' || r == '\u2028' || r == '\u2029' {
			return 0, 0, false
		}
	}
	return offset, width, true
}

// quotedScalar measures the text between a value's quotes. An escape inside
// them means those bytes are not the value, so the line is refused.
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

// plainScalar measures an unquoted value: from the separator up to a comment
// or the end of the line, trailing spaces left outside it.
func plainScalar(value []byte) (width int, ok bool) {
	plain := value
	// A comment opens at a "#" that follows white space, tab included.
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

// onlyCommentFollows reports whether the tail after a closing quote is nothing,
// white space, or white space then a comment.
func onlyCommentFollows(tail []byte) bool {
	trimmed := bytes.TrimLeft(tail, " \t")
	if len(trimmed) == 0 {
		return true
	}
	return len(trimmed) < len(tail) && trimmed[0] == '#'
}

// isYAMLIndicator reports whether c opens something other than a plain scalar.
// A status word never begins with one, so refusing them all costs nothing.
func isYAMLIndicator(c byte) bool {
	return bytes.IndexByte([]byte("-?:,[]{}#&*!|>%@`\"'"), c) >= 0
}
