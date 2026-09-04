package snapshot

import (
	"crypto/sha256"
	"time"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// Reading is the immutable projection of one markdown file captured in a Generation,
// as a reading page shows it. It holds no map, slice, handle or live contract
// reference, so returning it by value cannot mutate a published generation.
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
	// StatusNotText is true where the note wrote a status the reader did not
	// take as text, so Status above is empty for a reason a page can name.
	StatusNotText bool
	// Updated is the note's own declared update date, zero when the frontmatter
	// carries none; the page then falls back to the recorded change time.
	Updated time.Time
	// ContentIdentity is vault.ContentIdentity over the source bytes this
	// projection was captured from. The page embeds it in each transition form
	// so the write face can refuse a ruling read against content the disk no
	// longer carries.
	ContentIdentity [sha256.Size]byte
	// Searchable is false for a note too large to hold in the index. It still
	// renders, but a search that cannot reach it has to say so.
	Searchable bool
	// Stale is true for a copy this generation could not re-read and took from
	// the generation before it, so a page can say the file was not readable.
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
		StatusNotText:      parsed.StatusNotText(),
		Updated:            parsed.Updated(),
		ContentIdentity:    vault.ContentIdentity(data),
		Searchable:         searchable,
	}
}
