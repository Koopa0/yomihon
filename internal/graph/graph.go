// Package graph builds the wikilink resolution index: every markdown
// note's filename, path, and alias forms, normalized identically at
// index-build time and lookup time. What it implements is Obsidian's
// observed resolution behavior for [[wikilinks]] in this vault, pinned
// case by case by this package's tests: trim -> Unicode NFC -> lowercase
// normalization, four key forms per note (filename stem, filename, path
// stem, path) plus frontmatter aliases, and never the frontmatter title —
// the single most important correctness property of this package. A link
// written against a note's title, not its filename or an alias, silently
// fails to resolve in real Obsidian, and this resolver must reproduce
// that exact failure mode rather than paper over it by being more lenient
// than Obsidian actually is.
// Ambiguous names (two files sharing a normalized key) are reported in
// full and sorted, never guessed at.
package graph

import (
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// NormalizeKey is the single normalization every resolution key passes
// through, applied identically at index-build time and lookup time: trim,
// Unicode NFC, lowercase. It calls vault.NormalizeNFC for the NFC step so there
// is exactly one NFC definition in the repo. NFC matters because
// this vault's CJK filenames can arrive NFC or NFD (macOS itself stores
// filenames NFD on disk, independent of how a note's frontmatter aliases were
// typed).
//
// It is exported because a caller that builds its own name index — a title
// index, an alias index, a set of planned concept names — has to fold names
// the same way this resolver does, or the two disagree about what a written
// name matches. Callers share this function rather than reproducing its three
// steps, since a second copy agreeing today is agreement by maintenance
// accident rather than by construction.
func NormalizeKey(name string) string {
	return strings.ToLower(vault.NormalizeNFC(strings.TrimSpace(name)))
}

// Kind distinguishes the three possible outcomes of Resolve.
type Kind int

// Resolution kinds distinguish absence, one exact answer, and ambiguity.
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
// the vault-relative path(s) it names. Read-only once built.
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

// New builds an Index from notes and non-Markdown resources that have already
// been captured from the vault. Alias extraction stays in graph so callers do
// not need to duplicate Obsidian's resolution vocabulary. The inputs are read
// only during construction; the returned index owns its derived keys.
func New(notes []*vault.Note, resources []string) *Index {
	inputs := make([]NoteInput, 0, len(notes))
	for _, note := range notes {
		if note == nil {
			continue
		}
		inputs = append(inputs, NoteInput{
			Path:    note.RelPath,
			Aliases: aliases(note),
		})
	}
	return BuildFromNotes(inputs, resources)
}

// BuildFromNotes builds an Index directly from already-loaded note and
// resource data, with no disk access. It accepts the already-extracted alias
// projection used by corpus judges that do not retain vault.Note values.
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
	members := idx.names[NormalizeKey(name)]
	switch len(members) {
	case 0:
		return Resolution{Kind: Unresolved}
	case 1:
		return Resolution{Kind: Unique, Path: members[0]}
	default:
		return Resolution{Kind: Ambiguous, Candidates: slices.Clone(members)}
	}
}

// add inserts key (normalized) -> path, skipping an empty key and
// deduplicating per path. Extra keys only ever add resolutions, never
// remove one — the index deliberately prefers false positives over false
// negatives (better to flag a resolvable-but-uncertain link than to
// silently miss a real one).
func (idx *Index) add(key, path string) {
	normKey := NormalizeKey(key)
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
// shape that isn't a plain list of strings — a malformed aliases
// field costs that note its alias keys, not the whole index build.
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
