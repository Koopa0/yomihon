package judge

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/koopa0/kurodo/internal/vault"
)

// note is one markdown file as the diagnostics see it: its vault-relative
// NFC path, whether it carries a frontmatter block at all, whether that
// block was present but failed to parse, and — when it parsed — every
// frontmatter key with its raw value. The frontmatter checks need the whole
// key set to flag unknown fields and the raw scalar text to validate enums,
// beyond the handful of typed fields the reader surfaces.
type note struct {
	path           string
	noFrontmatter  bool
	badFrontmatter bool
	frontmatter    map[string]fmValue
}

// fmValue is a raw frontmatter value: either a scalar kept as the exact text
// the author wrote, or a list of such texts. The distinction is load-bearing
// for the checks — an enum check reads a scalar and skips a value written as
// a list, and a required-field check treats an empty scalar or an empty list
// as absent.
type fmValue struct {
	scalar string
	list   []string
	isList bool
}

// asScalar reports the scalar text, or false when the value is a list.
func (v fmValue) asScalar() (string, bool) {
	if v.isList {
		return "", false
	}
	return v.scalar, true
}

// present reports whether the value counts as filled in: a non-empty scalar,
// or a non-empty list.
func (v fmValue) present() bool {
	if v.isList {
		return len(v.list) > 0
	}
	return v.scalar != ""
}

// collectNotes walks the vault and parses every markdown file into a note.
// It reuses the shared walk, so the scan boundary is identical to the
// resolver's: dot-prefixed directories are skipped, a .gitignore is never
// consulted (a gitignored attachment is still a live link target), and every
// path is NFC-normalized. A ".md" extension is the boundary between a note
// and a linkable resource; only notes are returned.
func collectNotes(root string) ([]note, error) {
	paths, err := vault.List(root)
	if err != nil {
		return nil, err
	}
	notes := make([]note, 0, len(paths))
	for _, rel := range paths {
		if filepath.Ext(rel) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- rel is a vault-relative path from the shared walk over the operator's own local vault, not untrusted input
		if err != nil {
			return nil, fmt.Errorf("read note %s: %w", rel, err)
		}
		notes = append(notes, parseNote(rel, data))
	}
	return notes, nil
}

// parseNote splits a file's leading frontmatter and reads it into the note
// model. A file with no frontmatter block is legal (a raw transcript, scanned
// but never faulted). A block that is present but does not parse yields a note
// flagged bad, which the frontmatter check reports as a single fault rather
// than a cascade of "field missing" for fields that may sit above a syntax
// error.
func parseNote(rel string, data []byte) note {
	fm, _ := vault.SplitFrontmatter(data)
	if fm == nil {
		return note{path: rel, noFrontmatter: true}
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(fm, &doc); err != nil {
		return note{path: rel, badFrontmatter: true}
	}
	return note{path: rel, frontmatter: buildFrontmatter(&doc)}
}

// buildFrontmatter reads a parsed frontmatter document into the flat key/value
// map the checks consume. Only string keys are kept, matching how the author's
// mapping is read; a non-mapping document (a bare scalar or list at the top)
// contributes no keys.
func buildFrontmatter(doc *yaml.Node) map[string]fmValue {
	m := make(map[string]fmValue)
	root := doc
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return m
		}
		root = doc.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return m
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			continue
		}
		m[key.Value] = nodeValue(root.Content[i+1])
	}
	return m
}

// nodeValue converts one frontmatter value node into an fmValue: a sequence
// becomes a list, a scalar keeps its resolved text, and any other shape (a
// nested mapping) collapses to an empty scalar, so the checks see the same
// shape the resolver does. An alias is followed to the value it points at.
func nodeValue(n *yaml.Node) fmValue {
	n = resolveAlias(n)
	switch n.Kind {
	case yaml.SequenceNode:
		items := make([]string, 0, len(n.Content))
		for _, item := range n.Content {
			items = append(items, scalarText(item))
		}
		return fmValue{list: items, isList: true}
	case yaml.ScalarNode:
		return fmValue{scalar: scalarText(n)}
	default:
		return fmValue{}
	}
}

// resolveAlias follows an alias node to the node it refers to, so an aliased
// value reads as the value it points at.
func resolveAlias(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		return n.Alias
	}
	return n
}

// scalarText is a scalar value's text as the frontmatter checks read it. A
// quoted or block scalar is taken verbatim. An unquoted scalar is resolved
// under the YAML core schema — independent of how the parser happens to tag
// the token — so a boolean normalizes to lowercase, an integer to its decimal
// form, and a null (empty, "~", or "null") to an empty string, while every
// other token (including the 1.1 boolean words yes/no/on/off, an out-of-range
// number, and any real number) keeps exactly what the author wrote.
func scalarText(n *yaml.Node) string {
	n = resolveAlias(n)
	if n.Kind != yaml.ScalarNode {
		return ""
	}
	if !isPlain(n) {
		return n.Value
	}
	return resolvePlainScalar(n.Value)
}

// isPlain reports whether a scalar was written unquoted and unblocked; a
// quoted or block scalar is always its literal text.
func isPlain(n *yaml.Node) bool {
	const quotedOrBlock = yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle | yaml.LiteralStyle | yaml.FoldedStyle
	return n.Style&quotedOrBlock == 0
}

// resolvePlainScalar maps an unquoted scalar's text to the string the wire
// format carries for it under the YAML core schema: a hexadecimal (0x…),
// octal (0o…), explicitly-signed, or plain decimal integer collapses to its
// decimal form; the null and boolean words collapse as noted; anything the
// core schema does not read as an integer, boolean, or null — a real number,
// a number too large for a signed 64-bit integer, or plain text — is kept
// exactly as written.
func resolvePlainScalar(v string) string {
	switch {
	case strings.HasPrefix(v, "0x"):
		if i, err := strconv.ParseInt(v[2:], 16, 64); err == nil {
			return strconv.FormatInt(i, 10)
		}
	case strings.HasPrefix(v, "0o"):
		if i, err := strconv.ParseInt(v[2:], 8, 64); err == nil {
			return strconv.FormatInt(i, 10)
		}
	case strings.HasPrefix(v, "+"):
		if i, err := strconv.ParseInt(v[1:], 10, 64); err == nil {
			return strconv.FormatInt(i, 10)
		}
	}
	switch v {
	case "", "~", "null":
		return ""
	case "true", "True", "TRUE":
		return "true"
	case "false", "False", "FALSE":
		return "false"
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return strconv.FormatInt(i, 10)
	}
	return v
}
