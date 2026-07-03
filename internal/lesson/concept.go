package lesson

import (
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/koopa0/kurodo/internal/vault"
)

// ConceptSubdir is where a vault keeps the grammar concept notes a lesson links
// to (the same convention as the reference vault). It is a path constant, not a
// schema fact: the concept sheet reads these notes, it never validates them.
const ConceptSubdir = "Concepts/japanese"

// ConceptIndex maps a vault-relative concept-note path to its stable sheet slug
// (a DOM id shared between a trigger's data-concept and its <template>'s id).
// Keying by path — the shape a rendered wikilink href decodes to — lets the
// render post-pass ask "is this link a concept?" with one lookup.
type ConceptIndex map[string]string

// slugDrop collapses every run of non-letter/non-digit to one hyphen, so a
// concept basename becomes a CJK-safe DOM id (letters and digits, including
// kana/kanji, survive; punctuation and spaces fold to '-').
var slugDrop = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// conceptSlug is the sheet id for a concept basename: lowercase, keep Unicode
// letters and digits, collapse the rest to hyphens, trim, fall back to
// "concept". It need only be stable and collision-checked within this index —
// it is not the note's slug field.
func conceptSlug(base string) string {
	s := strings.Trim(slugDrop.ReplaceAllString(strings.ToLower(base), "-"), "-")
	if s == "" {
		return "concept"
	}
	return s
}

// BuildConceptIndex scans the vault's concept directory and indexes each note by
// its vault-relative path. A missing directory yields an empty index, not an
// error: a vault with no concept notes simply has no sheets (fail-open, like the
// slot loader). Two basenames that slug to the same id are reported (the sheet
// would open the wrong note), never guessed — the vault dialect's rule.
func BuildConceptIndex(root string) (ConceptIndex, error) {
	dir := filepath.Join(root, filepath.FromSlash(ConceptSubdir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return ConceptIndex{}, nil
		}
		return nil, err
	}
	idx := make(ConceptIndex, len(entries))
	bySlug := make(map[string]string, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		base := strings.TrimSuffix(name, ".md")
		relPath := ConceptSubdir + "/" + base + ".md"
		slug := conceptSlug(base)
		if prev, dup := bySlug[slug]; dup {
			slog.Warn("concept slug collision; the sheet may open the wrong note",
				"slug", slug, "a", prev, "b", base)
		}
		bySlug[slug] = base
		idx[relPath] = slug
	}
	return idx, nil
}

// SlugForPath reports the sheet slug for a vault-relative path, and whether it
// names a concept note. It is the predicate the render post-pass calls to decide
// whether a wikilink becomes a sheet trigger.
func (x ConceptIndex) SlugForPath(relPath string) (string, bool) {
	s, ok := x[relPath]
	return s, ok
}

// ConceptDoc is one concept note rendered for the sheet: its stable slug (the
// <template> id), a human title, and the body HTML.
type ConceptDoc struct {
	Slug  string
	Title string
	HTML  string
}

// LoadConcept reads a concept note and renders its body into a sheet document.
// renderBody is the note renderer's body→HTML step (the handler passes the plain
// note pipeline, so a concept's own wikilinks stay ordinary links and never nest
// a second sheet — the sheet is exactly one level deep). The slug comes from the
// index so it matches the trigger the post-pass wrote; the title falls back to
// the basename when the note has none.
func LoadConcept(renderBody func(body string) string, idx ConceptIndex, root, relPath string) (ConceptDoc, bool) {
	slug, ok := idx.SlugForPath(relPath)
	if !ok {
		return ConceptDoc{}, false
	}
	n, err := vault.ReadNote(root, relPath)
	if err != nil {
		return ConceptDoc{}, false
	}
	return ConceptDoc{Slug: slug, Title: n.Title(), HTML: renderBody(n.Body)}, true
}
