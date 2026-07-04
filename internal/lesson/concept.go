package lesson

import (
	"log/slog"
	"regexp"
	"strings"

	"github.com/koopa0/kurodo/internal/vault"
)

// ConceptSubdir is the root of the vault's grammar-concept notes; the vault
// organizes them one level down by domain (Concepts/japanese, Concepts/golang,
// Concepts/rust, …), and a lesson of any domain links to its own concepts — so
// the index scans the whole tree, not one language. It is a path constant, not a
// schema fact: the concept sheet reads these notes, it never validates them.
const ConceptSubdir = "Concepts"

// ConceptIndex maps a vault-relative concept-note path to its stable sheet slug
// (a DOM id shared between a trigger's data-concept and its <template>'s id).
// Keying by path — the shape a rendered wikilink href decodes to — lets the
// render post-pass ask "is this link a concept?" with one lookup.
type ConceptIndex map[string]string

// slugDrop collapses every run of non-letter/non-digit to one hyphen, so a
// concept basename becomes a CJK-safe DOM id (letters and digits, including
// kana/kanji, survive; punctuation and spaces fold to '-').
var slugDrop = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// conceptSlug is the sheet id for a concept, derived from its path under
// ConceptSubdir (e.g. "golang/Go Array" → "golang-go-array"): lowercase, keep
// Unicode letters and digits, collapse the rest — including the domain
// separator — to hyphens, trim, fall back to "concept". Deriving it from the
// full sub-path rather than the bare basename keeps the id unique across domains,
// so two same-named concepts in different domains never share a <template> id. It
// is an opaque DOM join key (a trigger's data-concept ↔ its template's id), not
// the note's slug field.
func conceptSlug(subPath string) string {
	s := strings.Trim(slugDrop.ReplaceAllString(strings.ToLower(subPath), "-"), "-")
	if s == "" {
		return "concept"
	}
	return s
}

// BuildConceptIndex indexes every concept note (Concepts/<domain>/…) by its
// vault-relative path — the shape a rendered wikilink href decodes to. It reads
// the vault through vault.List, the same NFC-normalizing walk graph/nav/search
// use, so a concept key is byte-identical to the resolver path the render
// post-pass looks it up by even when the filesystem stores the name NFD (macOS);
// a bespoke walk here would be the one reader that drops the fold (predictable
// mistakes #2/#6 — one NFC definition, one way to be correct). A vault with no
// concept notes yields an empty index, not an error (fail-open, like the slot
// loader). Two notes whose sub-paths slug to the same id are reported (the sheet
// would open the wrong note), never guessed — the vault dialect's rule.
func BuildConceptIndex(root string) (ConceptIndex, error) {
	paths, err := vault.List(root)
	if err != nil {
		return nil, err
	}
	idx := ConceptIndex{}
	bySlug := map[string]string{}
	for _, relPath := range paths {
		subPath, ok := conceptSubPath(relPath)
		if !ok {
			continue
		}
		slug := conceptSlug(subPath)
		if prev, dup := bySlug[slug]; dup {
			slog.Warn("concept slug collision; the sheet may open the wrong note",
				"slug", slug, "a", prev, "b", relPath)
		}
		bySlug[slug] = relPath
		idx[relPath] = slug
	}
	return idx, nil
}

// conceptSubPath reports the domain-scoped sub-path of a concept note
// ("Concepts/golang/Go Array.md" → "golang/Go Array", ok), or ok=false when the
// path is not a concept: not under Concepts/, not a .md, or sitting directly at
// the Concepts/ root. Every real concept lives one level down under its domain,
// so a bare Concepts/README.md (the folder's own index) is not a grammar concept
// and must not become a sheet.
func conceptSubPath(relPath string) (string, bool) {
	sub, ok := strings.CutPrefix(relPath, ConceptSubdir+"/")
	if !ok || !strings.HasSuffix(sub, ".md") {
		return "", false
	}
	sub = strings.TrimSuffix(sub, ".md")
	if !strings.Contains(sub, "/") {
		return "", false // directly under Concepts/, not inside a domain
	}
	return sub, true
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
