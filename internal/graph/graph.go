// Package graph builds the wikilink resolution index: every markdown
// note's filename, path, and alias forms, normalized identically to
// kura's (~/rust/github.com/koopa0/kura) graph.rs and wikilink.rs — the
// validated reference for exactly how Obsidian resolves [[wikilinks]] in
// this vault. kurodo reimplements the OBSERVABLE BEHAVIOR of that
// reference, not its code (decisions D04): trim -> NFC -> lowercase
// normalization, four key forms per note (filename stem, filename, path
// stem, path) plus frontmatter aliases, and never the frontmatter title —
// the single most important correctness property of this package. A link
// written against a note's title, not its filename or an alias, silently
// fails to resolve in real Obsidian, and this resolver must reproduce
// that exact failure mode rather than paper over it by being more lenient
// than Obsidian actually is (this is kura's link.title_not_alias rule).
// Ambiguous names (two files sharing a normalized key) are reported in
// full and sorted, never guessed at.
package graph

import (
	"fmt"
	"slices"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/kurodo/internal/vault"
)

// normalize is the one normalization graph.rs performs, applied
// identically at index-build time and lookup time: trim, Unicode NFC,
// lowercase. NFC matters because this vault's CJK filenames can arrive
// NFC or NFD (macOS itself stores filenames NFD on disk, independent of
// how a note's frontmatter aliases were typed).
func normalize(name string) string {
	return strings.ToLower(norm.NFC.String(strings.TrimSpace(name)))
}

// Kind distinguishes the three possible outcomes of Resolve, mirroring
// kura's Resolution enum.
type Kind int

const (
	Unresolved Kind = iota
	Unique
	Ambiguous
)

// Resolution is the outcome of resolving one wikilink target.
type Resolution struct {
	Kind Kind
	// Path holds the resolved vault-relative path when Kind == Unique.
	Path string
	// Candidates holds every candidate path, sorted, when Kind ==
	// Ambiguous — never guessed at, the caller decides how to present it.
	Candidates []string
}

// Index maps every normalized name a note or resource is resolvable by to
// the vault-relative path(s) it names. Read-only once built (see Build
// and BuildFromNotes).
type Index struct {
	names map[string][]string
}

// NoteInput is the minimal shape BuildFromNotes needs for one markdown
// note: its vault-relative path and its frontmatter aliases (already
// extracted — deliberately not its title, see the package doc).
type NoteInput struct {
	Path    string
	Aliases []string
}

// Build walks root (via vault.List) and indexes every markdown note and
// every other regular file. A note whose frontmatter fails to parse
// contributes no alias keys but is still indexed by its path-derived keys
// (wall 4: one bad file must not narrow what the rest of the index can
// resolve). Building the index walks and parses the entire vault; it
// runs once at process startup, not per request, so this favors
// simplicity over speed.
func Build(root string) (*Index, error) {
	paths, err := vault.List(root)
	if err != nil {
		return nil, fmt.Errorf("graph: build index: %w", err)
	}

	var notes []NoteInput
	var resources []string
	for _, p := range paths {
		if !strings.HasSuffix(p, ".md") {
			resources = append(resources, p)
			continue
		}
		n := NoteInput{Path: p}
		if note, readErr := vault.ReadNote(root, p); readErr == nil {
			n.Aliases = aliases(note)
		}
		// readErr != nil: the file vault.List just found is now
		// unreadable (race, permissions) or, more likely, its
		// frontmatter is malformed — vault.ReadNote itself already
		// tolerates bad YAML (returns a Note with FMDiagnostic set, no
		// error); an actual error here means the read itself failed.
		// Either way this note keeps its path-derived keys, it just
		// loses its aliases.
		notes = append(notes, n)
	}
	return BuildFromNotes(notes, resources), nil
}

// BuildFromNotes builds an Index directly from already-loaded note and
// resource data, with no disk access — the pure indexing logic Build (the
// disk-walking entry point) delegates to. Mirrors kura's
// SymbolTable::build(notes, resources), which is likewise disk-agnostic
// (kura::vault::load walks the disk separately).
func BuildFromNotes(notes []NoteInput, resources []string) *Index {
	idx := &Index{names: make(map[string][]string)}
	for _, n := range notes {
		for _, key := range noteKeys(n.Path) {
			idx.add(key, n.Path)
		}
		for _, alias := range n.Aliases {
			idx.add(alias, n.Path)
		}
	}
	for _, res := range resources {
		for _, key := range resourceKeys(res) {
			idx.add(key, res)
		}
	}
	for key, members := range idx.names {
		slices.Sort(members)
		idx.names[key] = members
	}
	return idx
}

// Resolve looks up name (typically a wikilink's stripped target, see
// SplitWikilink) against the index. Anchors are never verified by this
// package or its caller: [[X#heading]] resolves as long as X exists.
func (idx *Index) Resolve(name string) Resolution {
	members := idx.names[normalize(name)]
	switch len(members) {
	case 0:
		return Resolution{Kind: Unresolved}
	case 1:
		return Resolution{Kind: Unique, Path: members[0]}
	default:
		return Resolution{Kind: Ambiguous, Candidates: members}
	}
}

// add inserts key (normalized) -> path, skipping an empty key and
// deduplicating per path. Extra keys only ever ADD resolutions, never
// remove one — preserving kura's documented false-positive-over-false-
// negative bias (better to flag a resolvable-but-uncertain link than
// silently miss a real one).
func (idx *Index) add(key, path string) {
	normKey := normalize(key)
	if normKey == "" {
		return
	}
	members := idx.names[normKey]
	if !slices.Contains(members, path) {
		idx.names[normKey] = append(members, path)
	}
}

// noteKeys are the names Obsidian resolves a markdown note by: filename
// stem, filename, vault-relative path stem, and full path — never the
// frontmatter title (see the package doc).
func noteKeys(path string) []string {
	return []string{filenameStem(path), filename(path), pathStem(path), path}
}

// resourceKeys are the names Obsidian resolves a non-markdown file by:
// filename and path, both keeping the extension (Obsidian requires the
// extension to link a non-note resource).
func resourceKeys(path string) []string {
	return []string{filename(path), path}
}

func filename(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

func filenameStem(path string) string {
	return strings.TrimSuffix(filename(path), ".md")
}

func pathStem(path string) string {
	return strings.TrimSuffix(path, ".md")
}

// aliases extracts a note's frontmatter aliases list, tolerating any
// shape that isn't a plain list of strings (wall 4: a malformed aliases
// field costs that note its alias keys, not the whole index build).
func aliases(n *vault.Note) []string {
	raw, ok := n.Frontmatter["aliases"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
