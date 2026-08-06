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
	if !found {
		return n
	}
	var fields map[string]any
	if err := yaml.Unmarshal(block.Content, &fields); err != nil {
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

// Frontmatter is the byte-level split of one note. Content is the YAML between
// the fence lines, including the newline before the closing fence when one is
// present. ContentStart locates that slice in the original input so the status
// write face can replace one line without rebuilding any delimiter or newline.
type Frontmatter struct {
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
func SplitFrontmatter(data []byte) (Frontmatter, bool) {
	block := Frontmatter{Body: data, BodyStartLine: 1}
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
			return Frontmatter{
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
