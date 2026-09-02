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
	"strconv"
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
	KindUnresolved Kind = iota
	KindUnique
	KindAmbiguous
)

// Resolution is the outcome of resolving one wikilink target.
type Resolution struct {
	Kind Kind
	// RelPath holds the resolved vault-relative path when Kind is
	// KindUnique.
	RelPath string
	// Candidates holds every candidate path, sorted, when Kind is
	// KindAmbiguous — never guessed at, the caller decides how to present
	// it.
	Candidates []string
}

// String names a resolution kind for a diagnostic, a log line or a panic. A
// kind outside the three constants is a programming error, and it prints as a
// number here because this is the method every other site prints through.
func (k Kind) String() string {
	switch k {
	case KindUnresolved:
		return "unresolved"
	case KindUnique:
		return "unique"
	case KindAmbiguous:
		return "ambiguous"
	default:
		panic("graph: unknown Kind: " + strconv.Itoa(int(k)))
	}
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
	RelPath string
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
			RelPath: note.RelPath,
			Aliases: note.Aliases(),
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
		for _, key := range noteKeys(n.RelPath) {
			idx.add(key, n.RelPath)
		}
		for _, alias := range n.Aliases {
			idx.add(alias, n.RelPath)
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
		return Resolution{Kind: KindUnresolved}
	case 1:
		return Resolution{Kind: KindUnique, RelPath: members[0]}
	default:
		return Resolution{Kind: KindAmbiguous, Candidates: slices.Clone(members)}
	}
}

// Collisions reports every name more than one file answers to, each mapped to
// the paths claiming it. A name in this answer resolves to nothing: the index
// refuses to choose between the files holding it, so a citation written
// against it fails whether or not anyone has written that citation yet. The
// map and its slices belong to the caller, so reading this cannot disturb the
// index.
func (idx *Index) Collisions() map[string][]string {
	out := make(map[string][]string)
	for name, members := range idx.names {
		if len(members) < 2 {
			continue
		}
		out[name] = slices.Clone(members)
	}
	return out
}

// DistinctCollisions reports the index's collisions with the restated ones
// dropped, for a caller that describes every file it found.
func (idx *Index) DistinctCollisions() map[string][]string {
	return WithoutRestatedNames(idx.Collisions())
}

// WithoutRestatedNames drops from a collision map every name that only restates
// another: a filename goes when its extension-less form claims exactly the same
// files, since two files sharing "Foo.md" necessarily share "Foo", and one
// repair stated twice reads as two. A name ending in the extension whose
// claimants differ from the stem's is a separate collision and stays.
//
// It takes the map rather than reading the index so that a caller which may not
// describe every claimant asks over the files it may describe. Deciding on the
// index's own membership and narrowing afterwards would split one repair into
// two rows whenever the narrowing is what made the two sets equal.
//
// One function serves both, because every surface that counts collisions has to
// count the same ones: a reader looking at the diagnostics page and a pipeline
// gated on the judge's findings comparing two different totals for the same
// folder would have no way to tell which of them was wrong.
func WithoutRestatedNames(byName map[string][]string) map[string][]string {
	out := make(map[string][]string, len(byName))
	for name, members := range byName {
		if stem, ok := strings.CutSuffix(name, ".md"); ok && slices.Equal(byName[stem], members) {
			continue
		}
		out[name] = members
	}
	return out
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
