// Package vault is the vault on disk: path identity, the note model, and the
// read capability that pins one directory. It splits frontmatter from body,
// reads frontmatter values, and holds the one order vault paths sort by. The read face answers what is
// under the pin, what each file was when observed, and what its bytes are now;
// it never writes. Every read descends the recorded path component by
// component and refuses the moment an object stops being the one observed, so
// a rename under a reader's feet costs the read rather than yielding bytes.
// Broken frontmatter is recorded on the note itself rather than returned as
// an error.
package vault

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Note is one markdown file, frontmatter split from body.
type Note struct {
	RelPath string
	// HasFrontmatter is whether the bytes opened a frontmatter block. An empty
	// fence pair still counts: the block is there even when YAML decodes to no
	// fields and Frontmatter stays nil.
	HasFrontmatter bool
	Frontmatter    map[string]any
	// Domain is the first folder below a root the contract derives domains
	// from, empty for a note outside every root. A note never writes it, so
	// Parse leaves it empty and whoever reads a vault against its contract
	// fills it in.
	Domain string
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
// becomes the note's FMDiagnostic rather than an error. A found block sets
// HasFrontmatter even when the map stays nil.
func Parse(rel string, data []byte) *Note {
	n := &Note{RelPath: rel}
	block, found := SplitFrontmatter(data)
	n.Body = string(block.Body)
	n.BodyLine = block.BodyStartLine
	if !found {
		return n
	}
	n.HasFrontmatter = true
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

// Type is the frontmatter type, empty when absent.
func (n *Note) Type() string {
	t, _ := n.Text("type")
	return t
}

// Aliases are the other names this note answers to, in declaration order, and
// nothing when it declared none. Link resolution and search both read these.
func (n *Note) Aliases() []string {
	return n.Strings("aliases")
}

// Created is the note's declared creation date, or the zero time when the
// frontmatter carries none or a shape no date reads from. YAML hands an
// unquoted date over as a time and a quoted one as text, so both are read.
func (n *Note) Created() time.Time {
	return n.declaredDate("created")
}

// VerifiedOn is the day the author wrote down as the one they last checked the
// note's claims on, read as Created reads its date: the zero time when the
// frontmatter carries none or a shape no date reads from. It is the author's
// statement and nothing here confirms it.
func (n *Note) VerifiedOn() time.Time {
	return n.declaredDate("verified_on")
}

func (n *Note) declaredDate(key string) time.Time {
	switch v := n.Frontmatter[key].(type) {
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
// diagnostic cites the file's own line.
type FrontmatterSplit struct {
	Content       []byte
	Body          []byte
	ContentStart  int
	BodyStartLine int
}

// SplitFrontmatter separates a leading YAML frontmatter block from the body. A
// block opens with a "---" line and closes with the next "---" or "..." line;
// LF and CRLF are accepted, as is a closing fence at EOF, and an unterminated
// opening fence is body text. One leading byte-order mark belongs to the file,
// so both readers step over it. The input stays untouched and offsets stay
// measured against its original bytes.
func SplitFrontmatter(data []byte) (FrontmatterSplit, bool) {
	opening, _ := bytes.CutPrefix(data, []byte("\xef\xbb\xbf"))
	block := FrontmatterSplit{Body: opening, BodyStartLine: 1}
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
