package lesson

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// conceptSubdir is the root of the vault's grammar-concept notes; the vault
// organizes them one level down by domain (Concepts/japanese, Concepts/golang,
// Concepts/rust, …), and a lesson of any domain links to its own concepts — so
// the captured input covers the whole tree, not one language. It is a path
// constant, not a schema fact: the concept sheet projects these notes, it never
// validates them.
const conceptSubdir = "Concepts"

// ConceptIndex is the immutable lookup index from a vault-relative concept-note
// path to its stable sheet ID and captured sheet content. Keying by path — the
// shape a rendered wikilink href decodes to — lets the render post-pass ask "is
// this link a concept?" with one lookup. The zero value is an empty index.
type ConceptIndex struct {
	byPath map[string]conceptSource
}

type conceptSource struct {
	id    string
	title string
	body  string
}

// conceptID encodes the full NFC vault-relative path as an injective URL-safe
// DOM join key. It does not fold case or punctuation, so distinct normalized
// paths cannot collapse onto the same concept sheet.
func conceptID(relPath string) string {
	normalized := vault.NormalizeNFC(relPath)
	return "path-" + base64.RawURLEncoding.EncodeToString([]byte(normalized))
}

// NewConceptIndex indexes parsed concept notes captured by one vault
// generation. Construction copies only each concept's canonical path, title,
// and body; it retains neither the notes nor their frontmatter maps. A vault
// with no concept notes yields an empty index. IDs encode the complete
// normalized path rather than a lossy display slug, so every indexed concept
// owns a distinct sheet join key.
func NewConceptIndex(notes []*vault.Note) (ConceptIndex, error) {
	idx := ConceptIndex{byPath: make(map[string]conceptSource, len(notes))}
	for ordinal, note := range notes {
		if note == nil {
			return ConceptIndex{}, fmt.Errorf("concept note %d is nil", ordinal)
		}
		relPath := vault.NormalizeNFC(note.RelPath)
		if !isConceptPath(relPath) {
			continue
		}
		if _, exists := idx.byPath[relPath]; exists {
			return ConceptIndex{}, fmt.Errorf("concept path %q appears more than once", relPath)
		}
		idx.byPath[relPath] = conceptSource{
			id:    conceptID(relPath),
			title: note.Title(),
			body:  note.Body,
		}
	}
	return idx, nil
}

// isConceptPath reports whether relPath names a domain-scoped concept note. A
// path is not a concept when it is outside Concepts/, is not Markdown, or sits
// directly at the Concepts/ root. Every real concept lives one level down under
// its domain, so a bare Concepts/README.md (the folder's own index) is not a
// grammar concept and must not become a sheet.
func isConceptPath(relPath string) bool {
	sub, ok := strings.CutPrefix(relPath, conceptSubdir+"/")
	if !ok || !strings.HasSuffix(sub, ".md") {
		return false
	}
	sub = strings.TrimSuffix(sub, ".md")
	return strings.Contains(sub, "/")
}

// IDForPath reports the sheet ID for a vault-relative path, and whether it
// names a concept note. It is the predicate the render post-pass calls to decide
// whether a wikilink becomes a sheet trigger.
func (x ConceptIndex) IDForPath(relPath string) (string, bool) {
	source, ok := x.byPath[vault.NormalizeNFC(relPath)]
	return source.id, ok
}

// Len reports the number of indexed concept notes.
func (x ConceptIndex) Len() int {
	return len(x.byPath)
}

// ConceptDoc is one concept note rendered for the sheet: its stable ID (the
// <template> id), a human title, and the body HTML.
type ConceptDoc struct {
	ID    string
	Title string
	HTML  string
}

// Document renders one captured concept into a sheet document. renderBody is
// the note renderer's body-to-HTML step (the handler passes the plain note
// pipeline, so a concept's own wikilinks stay ordinary links and never nest a
// second sheet). It receives the concept's own path along with its body,
// because the sheet opens over a different note and a body rendered against
// the reader's location would address the wrong files. The ID comes from the
// index so it matches the trigger the post-pass wrote.
func (x ConceptIndex) Document(renderBody func(relPath, body string) string, relPath string) (ConceptDoc, bool) {
	key := vault.NormalizeNFC(relPath)
	source, ok := x.byPath[key]
	if !ok || renderBody == nil {
		return ConceptDoc{}, false
	}
	return ConceptDoc{ID: source.id, Title: source.title, HTML: renderBody(key, source.body)}, true
}
