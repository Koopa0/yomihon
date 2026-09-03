package lesson

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

// conceptSubdir is the root of the vault's concept notes, which the vault
// organizes one level down by domain. It is a path constant, not a schema
// fact: the concept sheet projects these notes and never validates them.
const conceptSubdir = "Concepts"

// ConceptIndex is the immutable lookup from a vault-relative concept-note path
// to its stable sheet ID and captured content — the shape a rendered wikilink
// href decodes to. The zero value is an empty index.
type ConceptIndex struct {
	byPath map[string]conceptSource
}

type conceptSource struct {
	id    string
	title string
	body  string
}

// conceptID encodes the full NFC path as an injective URL-safe join key. It
// folds neither case nor punctuation, so distinct paths cannot collapse.
func conceptID(relPath string) string {
	normalized := vault.NormalizeNFC(relPath)
	return "path-" + base64.RawURLEncoding.EncodeToString([]byte(normalized))
}

// NewConceptIndex indexes parsed concept notes captured by one vault
// generation, copying only each concept's path, title and body. A vault with
// no concept notes yields an empty index.
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

// isConceptPath reports whether relPath names a domain-scoped concept note.
// Every concept lives one level down under its domain, so a Markdown file at
// the Concepts/ root — the folder's own index — is not one.
func isConceptPath(relPath string) bool {
	sub, ok := strings.CutPrefix(relPath, conceptSubdir+"/")
	if !ok || !vault.IsMarkdown(sub) {
		return false
	}
	sub = strings.TrimSuffix(sub, ".md")
	return strings.Contains(sub, "/")
}

// IDForPath reports the sheet ID for a vault-relative path, and whether it
// names a concept note.
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

// Document renders one captured concept into a sheet document. renderBody
// receives the concept's own path along with its body, because the sheet opens
// over a different note and a body rendered against the reader's location
// would address the wrong files. A nil renderBody is a wiring fault and panics.
func (x ConceptIndex) Document(renderBody func(relPath, body string) string, relPath string) (ConceptDoc, bool) {
	if renderBody == nil {
		panic("lesson: Document requires a non-nil renderBody")
	}
	key := vault.NormalizeNFC(relPath)
	source, ok := x.byPath[key]
	if !ok {
		return ConceptDoc{}, false
	}
	return ConceptDoc{ID: source.id, Title: source.title, HTML: renderBody(key, source.body)}, true
}
