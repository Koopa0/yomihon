// Package lexical is the vault's in-memory search index and query engine. It
// holds one entry per note and answers a deterministic, NFC-folded substring
// query plus six structured filters. There is no database: the truth is the
// vault files and the index only accelerates. It stays reachable without the
// reading interface, so the search page depends on it and never the reverse.
package lexical

import (
	"errors"
	"path"
	"slices"
	"strings"
	"unicode"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// fold is the single definer of "what counts as a match": NFC, then the walk
// below, applied identically to stored text and to a query token. Case folding
// lives only here; vault.NormalizeNFC supplies the shared NFC step.
func fold(s string) string {
	var out strings.Builder
	out.Grow(len(s))
	foldRunes(vault.NormalizeNFC(s), func(r rune, _ int) {
		out.WriteRune(r)
	})
	return out.String()
}

// foldRunes walks already-NFC text and hands the caller each rune the fold keeps,
// with the byte offset in s it came from. It is the whole of what folding does, so
// the folded string and the folded string mapped back to its source cannot
// disagree about what a match is. A line break is dropped only when the characters
// on both sides belong to scripts that do not divide their words with spaces.
func foldRunes(s string, emit func(r rune, at int)) {
	var prev rune
	for i, r := range s {
		if r == '\n' && writesWithoutSpaces(prev) && writesWithoutSpaces(nextRune(s, i+1)) {
			continue
		}
		emit(unicode.ToLower(r), i)
		prev = r
	}
}

// nextRune returns the first rune at or after i, or zero at the end of s.
func nextRune(s string, i int) rune {
	for _, r := range s[i:] {
		return r
	}
	return 0
}

// writesWithoutSpaces reports whether a character belongs to a script that does
// not part its words with spaces, the distinction Unicode text segmentation
// (UAX #29) draws. Han, hiragana and katakana carry it; Hangul does not, because
// modern Korean divides its words with spaces.
func writesWithoutSpaces(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r)
}

// Document is the disk-free input to NewIndex: everything the index needs about
// one vault entry, with PlainText already extracted — render.PlainText for a
// note, the file's own characters for anything else.
type Document struct {
	RelPath  string
	Title    string
	NoteType string
	Domain   string
	Status   string
	Slug     string
	Topics   []string
	// Aliases are the other names the note declared. They are the names a
	// wikilink resolves by, so leaving them out made this program findable by
	// fewer names than it follows.
	Aliases   []string
	PlainText string

	// File marks an entry that is not a note: a vault file shown as characters.
	// It carries no frontmatter, so it answers no metadata projection, and it
	// sorts after every note in a result list.
	File bool

	// FrontmatterUnreadable marks a note whose frontmatter was there and could
	// not be parsed, which is not the same as a note that declares nothing. Both
	// arrive with every field empty, and only this tells them apart.
	FrontmatterUnreadable bool
}

// entry is one indexed note. Title and PlainText keep their display form and the
// *Fold copies are folded for matching, a few extra MB for an allocation-free
// match; the structured values are NFC but case-preserving, so a filter is exact.
type entry struct {
	RelPath string
	// PathFold is the note's own location, folded the way the text is: a reader
	// wrote their folder names and expects to find notes by them.
	PathFold  string
	Title     string
	TitleFold string
	// Aliases are the note's other declared names, held as written and folded for
	// matching. A link resolves by these and not by the title, so a hit on one
	// ranks with a title hit rather than below a mention in prose.
	Aliases         []string
	AliasFolds      []string
	NoteType        string
	Domain          string
	Status          string
	Slug            string
	Topics          []string
	PlainText       string
	PlainFold       string
	isFile          bool
	metadataCapable bool
	// frontmatterUnreadable records that this note had a frontmatter block that
	// could not be parsed, so a tally can separate it from a note that declared
	// nothing.
	frontmatterUnreadable bool
}

// Index is the whole in-memory search index, entries kept in the vault's reading
// order so each result bucket inherits it without a sort call, read-only once
// built. It records whether metadata projections are available at all.
type Index struct {
	entries []*entry
	policy  schema.ArtifactPolicy
}

// ErrMetadataUnavailable identifies a query or aggregate that requires
// instance metadata while the artifact policy is unavailable.
var ErrMetadataUnavailable = errors.New("search metadata unavailable")

// metadataUnavailableError carries the declaration outcome rather than a
// finished sentence, so a page can say why in its reader's language. Error stays
// the operator's line, for a log and a caller that only prints.
type metadataUnavailableError struct {
	claim schema.Claim
}

func (e metadataUnavailableError) Error() string {
	return e.claim.Diagnostic()
}

func (e metadataUnavailableError) Unwrap() error {
	return ErrMetadataUnavailable
}

// MetadataClaim reports the authority claim behind a metadata refusal, so a
// surface can write the reason in its reader's language. It answers false for
// any other error, a metadata refusal carrying no claim included.
func MetadataClaim(err error) (schema.Claim, bool) {
	unavailable, ok := errors.AsType[metadataUnavailableError](err)
	if !ok {
		return schema.Claim{}, false
	}
	return unavailable.claim, true
}

func (idx *Index) metadataUnavailableError() error {
	return metadataUnavailableError{claim: idx.policy.Claim()}
}

// NewIndex builds an Index from already-extracted note data and a startup-derived
// artifact policy, with no disk access. Every document stays in the text and
// folder corpus; policy marks which entries may answer metadata projections.
// Entries are sorted into the vault's reading order here, the sole source of
// result ordering, and the sort is stable for a caller repeating a RelPath.
func NewIndex(docs []Document, policy schema.ArtifactPolicy) *Index {
	entries := make([]*entry, 0, len(docs))
	for i := range docs {
		e := entryFromDocument(&docs[i], policy)
		entries = append(entries, &e)
	}
	slices.SortStableFunc(entries, func(a, b *entry) int {
		return vault.ComparePaths(a.RelPath, b.RelPath)
	})
	return &Index{entries: entries, policy: policy}
}

// WithArtifactPolicy returns a read-only copy bound to policy, which has to be a
// point-in-time capture of the same artifact authority idx was built from, so one
// request's metadata queries answer to the capture its projections used.
func (idx *Index) WithArtifactPolicy(policy schema.ArtifactPolicy) *Index {
	if idx == nil {
		return nil
	}
	bound := *idx
	bound.policy = policy
	return &bound
}

// entryFromDocument derives one entry from a Document, applying the storage rules:
// Title/PlainText stored NFC (display), the *Fold copies fold()ed, the
// structured field values stored NFC (case-preserving).
func entryFromDocument(d *Document, policy schema.ArtifactPolicy) entry {
	title := vault.NormalizeNFC(d.Title)
	plain := vault.NormalizeNFC(d.PlainText)
	topics := make([]string, len(d.Topics))
	for i, t := range d.Topics {
		topics[i] = vault.NormalizeNFC(t)
	}
	aliases := make([]string, len(d.Aliases))
	aliasFolds := make([]string, len(d.Aliases))
	for i, a := range d.Aliases {
		aliases[i] = vault.NormalizeNFC(a)
		aliasFolds[i] = fold(aliases[i])
	}
	return entry{
		RelPath:    d.RelPath,
		PathFold:   fold(vault.NormalizeNFC(d.RelPath)),
		Title:      title,
		TitleFold:  fold(title),
		Aliases:    aliases,
		AliasFolds: aliasFolds,
		NoteType:   vault.NormalizeNFC(d.NoteType),
		Domain:     vault.NormalizeNFC(d.Domain),
		Status:     vault.NormalizeNFC(d.Status),
		Slug:       vault.NormalizeNFC(d.Slug),
		Topics:     topics,
		PlainText:  plain,
		PlainFold:  fold(plain),
		isFile:     d.File,
		// An unclaimed policy excludes nothing, so every readable note answers over
		// its own raw frontmatter. A file has no frontmatter, so it answers no
		// metadata projection under any policy.
		metadataCapable:       !d.File && policy.Trustworthy() && !policy.IsNonInstance(d.RelPath),
		frontmatterUnreadable: d.FrontmatterUnreadable,
	}
}

// DocumentFromNote extracts a Document from a parsed note: the structured fields
// from frontmatter and PlainText from the render AST. A note with malformed
// frontmatter contributes empty structured fields; its body text is still indexed.
func DocumentFromNote(n *vault.Note) Document {
	return Document{
		RelPath:   n.RelPath,
		Title:     n.Title(),
		NoteType:  n.Type(),
		Domain:    n.Domain(),
		Status:    n.Status(),
		Slug:      n.Slug(),
		Topics:    n.Strings("topics"),
		Aliases:   n.Aliases(),
		PlainText: render.PlainText(n.Body),
		// A diagnostic here means the block was present and did not parse. A
		// note that simply carries no frontmatter has none, and is not this.
		FrontmatterUnreadable: n.FMDiagnostic != "",
	}
}

// DocumentFromFile builds the index entry for a vault file that is not a note:
// its title is the file's own name and its body is its whole text, exactly the
// characters its page shows.
func DocumentFromFile(relPath string, data []byte) Document {
	return Document{
		RelPath:   relPath,
		Title:     path.Base(relPath),
		PlainText: string(data),
		File:      true,
	}
}

// Len reports how many entries are indexed.
func (idx *Index) Len() int {
	return len(idx.entries)
}

// TypeStatus is a note's (type, status) pair. Which onward transitions a status
// allows depends on the note type, so a caller that weighs those transitions
// needs both together — a tally keyed on status alone cannot supply it.
type TypeStatus struct {
	Type   string
	Status string
}

// CountUnreadableFrontmatter reports how many of the notes this index counts had
// a frontmatter block that could not be parsed. It answers over exactly the
// entries CountByTypeStatus answers over, because its purpose is to divide that
// tally's empty bucket, where an unreadable note and a note with no status meet.
func (idx *Index) CountUnreadableFrontmatter() (int, error) {
	if !idx.policy.Trustworthy() {
		return 0, idx.metadataUnavailableError()
	}
	unreadable := 0
	for _, e := range idx.entries {
		if e.metadataCapable && e.frontmatterUnreadable {
			unreadable++
		}
	}
	return unreadable, nil
}

// CountByTypeStatus tallies metadata-capable notes by their (type, status) pair
// in one pass; the pair is the form of the question, because transition rules key
// on type as well as status. A note missing either field lands in that field's ""
// bucket. A declared but unhonourable artifact policy returns ErrMetadataUnavailable.
func (idx *Index) CountByTypeStatus() (map[TypeStatus]int, error) {
	if !idx.policy.Trustworthy() {
		return nil, idx.metadataUnavailableError()
	}
	counts := make(map[TypeStatus]int, len(idx.entries))
	for _, e := range idx.entries {
		if !e.metadataCapable {
			continue
		}
		counts[TypeStatus{Type: e.NoteType, Status: e.Status}]++
	}
	return counts, nil
}

// StatusHolder is one indexed note's identity beside the lifecycle fields a
// contract rules on: the row form of CountByTypeStatus, the same notes named
// rather than tallied.
type StatusHolder struct {
	RelPath string
	Type    string
	Status  string
}

// StatusHolders lists every metadata-capable note carrying a status, in the
// index's own reading order. It returns exactly the notes CountByTypeStatus
// tallies, so a page showing both cannot state a number its list does not fill.
func (idx *Index) StatusHolders() ([]StatusHolder, error) {
	if !idx.policy.Trustworthy() {
		return nil, idx.metadataUnavailableError()
	}
	out := make([]StatusHolder, 0, len(idx.entries))
	for _, e := range idx.entries {
		if !e.metadataCapable || e.Status == "" {
			continue
		}
		out = append(out, StatusHolder{
			RelPath: e.RelPath,
			Type:    e.NoteType,
			Status:  e.Status,
		})
	}
	return out, nil
}
