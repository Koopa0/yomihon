package snapshot

import (
	"crypto/sha256"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Note is the immutable reading projection of one Markdown file captured in a
// View. It contains no map, slice, filesystem handle, or live contract
// reference, so returning it by value cannot mutate a published generation.
type Note struct {
	RelPath            string
	Title              string
	Body               string
	Type               string
	Status             string
	Slug               string
	FMDiagnostic       string
	Language           string
	LanguageDiagnostic string
	HasFrontmatter     bool
	ContentHash        [sha256.Size]byte
	// Searchable is false for a note too large to hold in the index. It still
	// renders — every file in the folder stays readable — but a search that
	// cannot reach it must say so, or the answer "no results" is a false
	// statement about the folder.
	Searchable bool
}

func captureNote(parsed *vault.Note, data []byte, languages schema.ArticleLanguage, searchable bool) Note {
	if parsed == nil {
		return Note{}
	}
	language, err := languages.Resolve(parsed.Frontmatter)
	diagnostic := ""
	if err != nil {
		diagnostic = err.Error()
	}
	return Note{
		RelPath:            parsed.RelPath,
		Title:              parsed.Title(),
		Body:               parsed.Body,
		Type:               parsed.Type(),
		Status:             parsed.Status(),
		Slug:               parsed.Slug(),
		FMDiagnostic:       parsed.FMDiagnostic,
		Language:           language,
		LanguageDiagnostic: diagnostic,
		HasFrontmatter:     parsed.Frontmatter != nil,
		ContentHash:        sha256.Sum256(data),
		Searchable:         searchable,
	}
}
