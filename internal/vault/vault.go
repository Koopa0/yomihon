// Package vault reads notes from the Obsidian vault on disk. The vault is
// the single source of truth; nothing in this package ever writes to it.
//
// Reading is fault-tolerant by contract (wall 4): a broken frontmatter block
// yields a note with a diagnostic attached, never an error that would stop
// rendering.
package vault

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Note is one markdown file, frontmatter split from body.
type Note struct {
	RelPath     string
	Frontmatter map[string]any
	// FMDiagnostic is non-empty when the frontmatter block exists but is not
	// valid YAML. Display-only: kurodo reports, kura judges, humans edit.
	FMDiagnostic string
	Body         string
}

// ReadNote loads one note by vault-relative path. The path must stay inside
// root; anything else is an error even on this trusted, local corpus.
func ReadNote(root, rel string) (*Note, error) {
	rel = filepath.FromSlash(rel)
	if !filepath.IsLocal(rel) {
		return nil, fmt.Errorf("read note: path %q escapes vault root", rel)
	}
	data, err := os.ReadFile(filepath.Join(root, rel)) // #nosec G304 -- rel is validated local by filepath.IsLocal above; root is the operator's own vault
	if err != nil {
		return nil, fmt.Errorf("read note %s: %w", rel, err)
	}
	return Parse(filepath.ToSlash(rel), data), nil
}

// Parse splits raw file bytes into frontmatter and body and decodes the
// frontmatter. rel is stored on the returned Note as-is (callers pass a
// slash-form vault-relative path). This is the one place that decides what
// a note's frontmatter means; both ReadNote and the status package's write
// path (which must never disagree with a reader about the current status)
// call it.
func Parse(rel string, data []byte) *Note {
	n := &Note{RelPath: rel}
	fm, body := SplitFrontmatter(data)
	n.Body = string(body)
	if fm == nil {
		return n
	}
	var fields map[string]any
	if err := yaml.Unmarshal(fm, &fields); err != nil {
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

// SplitFrontmatter separates a leading ----fenced YAML block from the body.
// It returns a nil frontmatter slice when the file has no block. Splitting
// happens before any body preprocessing so that wikilink-looking values
// (e.g. based_on: "[[...]]") are never corrupted by later passes.
//
// Exported so the status package can locate the raw frontmatter block for
// its surgical status-line rewrite without re-implementing this split.
func SplitFrontmatter(data []byte) (fm, body []byte) {
	rest, found := bytes.CutPrefix(data, []byte("---\n"))
	if !found {
		return nil, data
	}
	fm, body, found = bytes.Cut(rest, []byte("\n---\n"))
	if !found {
		// Unterminated block: treat the whole file as body and let the
		// YAML diagnostic surface via the (missing) frontmatter instead.
		return nil, data
	}
	return fm, body
}
