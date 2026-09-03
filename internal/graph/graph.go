// Package graph resolves [[wikilinks]] the way Obsidian resolves them in this
// vault: four keys per note (filename stem, filename, path stem, path) plus
// frontmatter aliases, each folded by NormalizeKey — never the frontmatter
// title, which Obsidian does not resolve either. Two files sharing a key are
// reported in full and never guessed between.
package graph

import (
	"slices"
	"strconv"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// NormalizeKey folds a resolution key the way index build and lookup both fold
// it: trim, Unicode NFC, lowercase. A caller building its own name index folds
// through this, so the two agree about which written names match.
func NormalizeKey(name string) string {
	return strings.ToLower(vault.NormalizeNFC(strings.TrimSpace(name)))
}

// Kind distinguishes the three possible outcomes of Resolve.
type Kind int

const (
	KindUnresolved Kind = iota
	KindUnique
	KindAmbiguous
)

// Resolution is the outcome of resolving one wikilink target.
type Resolution struct {
	Kind Kind
	// RelPath is the resolved vault-relative path when Kind is KindUnique.
	RelPath string
	// Candidates lists every claimant, sorted, when Kind is KindAmbiguous;
	// the index never chooses between them.
	Candidates []string
}

// String names a resolution kind for a diagnostic or a log line.
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

// Index maps every normalized name a note or resource is resolvable by to the
// vault-relative paths holding it. Read-only once built.
type Index struct {
	names map[string][]string
}

// NoteInput is what BuildFromNotes needs of one markdown note: its
// vault-relative path and its frontmatter aliases, never its title.
type NoteInput struct {
	RelPath string
	Aliases []string
}

// New builds an Index over notes and non-markdown resources already captured
// from the vault, extracting each note's aliases itself.
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

// BuildFromNotes builds an Index from note paths, their already-extracted
// aliases, and resource paths, touching no disk.
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

// Resolve looks name up against the index. An anchor is never verified here:
// [[X#heading]] resolves as long as X exists.
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

// Collisions reports every name more than one file answers to, mapped to the
// paths claiming it. Such a name resolves to nothing — the index refuses to
// choose. The returned map and slices belong to the caller.
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

// DistinctCollisions is Collisions with the restated names dropped.
func (idx *Index) DistinctCollisions() map[string][]string {
	return WithoutRestatedNames(idx.Collisions())
}

// WithoutRestatedNames drops from a collision map every name that only restates
// another: a filename goes when its extension-less form claims exactly the same
// files, so one repair is not counted twice. It takes the map rather than the
// index so a caller that describes only some claimants asks over those.
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

// add records a normalized key -> path, ignoring an empty key.
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

// noteKeys are the names Obsidian resolves a markdown note by, title excluded.
func noteKeys(path string) []string {
	return []string{filenameStem(path), filename(path), pathStem(path), path}
}

// resourceKeys are the names Obsidian resolves a non-markdown file by; both
// keep the extension, which Obsidian requires to link a non-note resource.
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
