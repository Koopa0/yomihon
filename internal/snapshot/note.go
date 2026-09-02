package snapshot

import (
	"crypto/sha256"
	"time"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Reading is the immutable projection of one Markdown file captured in a View,
// as a reading page shows it. It contains no map, slice, filesystem handle, or
// live contract reference, so returning it by value cannot mutate a published
// generation. It is named for what it is rather than for the file, because the
// parsed source of that same file is a vault.Note and a build loop holds both.
type Reading struct {
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
	// Updated is the note's own declared update date, zero when the
	// frontmatter carries none a date can be read from. The reading page
	// shows it, falling back to the file's recorded change time — a
	// different claim, which the page labels differently.
	Updated time.Time
	// ContentIdentity is vault.ContentIdentity over the source bytes this
	// projection was captured from: everything but the status line, which the
	// reading page reads live and binds separately. The page embeds it in each
	// transition form so the write face can refuse a ruling read against
	// content the disk no longer carries.
	ContentIdentity [sha256.Size]byte
	// Searchable is false for a note too large to hold in the index. It still
	// renders — every file in the folder stays readable — but a search that
	// cannot reach it must say so, or the answer "no results" is a false
	// statement about the folder.
	Searchable bool
	// Stale is true for a copy this generation could not re-read and took from
	// the generation before it. The note renders its last-known content, which
	// is the only content there is; a reader deciding what the file says now
	// has to be told that the file itself was not readable when the page was
	// built.
	Stale bool
}

func newReading(parsed *vault.Note, data []byte, languages schema.ArticleLanguage, searchable bool) Reading {
	if parsed == nil {
		return Reading{}
	}
	language, err := languages.Resolve(parsed.Frontmatter)
	diagnostic := ""
	if err != nil {
		diagnostic = err.Error()
	}
	return Reading{
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
		Updated:            parsed.Updated(),
		ContentIdentity:    vault.ContentIdentity(data),
		Searchable:         searchable,
	}
}
