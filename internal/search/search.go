// Package search is the vault's in-memory search index and query engine. It
// holds one entry per note — only the fields search actually reads, nothing
// speculative — and answers a deterministic, NFC-folded substring query plus
// six structured filters. It owns the index and the query; it does NOT own
// freshness: the index is one projection in the shared reading generation,
// rebuilt from the vault on change. There is no database and no persistent
// state — the truth is the vault files, the index only accelerates.
package search

import (
	"cmp"
	"errors"
	"path"
	"slices"
	"strings"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// fold is the single definer of "what counts as a match": NFC then lowercase,
// applied identically to stored text and to a query token. Case folding lives
// only here — vault.NormalizeNFC supplies the shared NFC step so there is no
// second, subtly divergent normalization in the repo.
func fold(s string) string {
	return strings.ToLower(vault.NormalizeNFC(s))
}

// Document is the pure, disk-free input to NewIndex: everything the index needs
// about one vault entry, with PlainText already extracted (render.PlainText for
// a note, the file's own characters for anything else). Keeping the build pure
// makes it unit-testable without a vault on disk.
type Document struct {
	RelPath   string
	Title     string
	NoteType  string
	Domain    string
	Status    string
	Slug      string
	Topics    []string
	PlainText string

	// File marks an entry that is not a note: a vault file yomihon shows as
	// characters. It carries no frontmatter, so it answers no metadata
	// projection, and it sorts after every note in a result list — someone
	// searching a reading tool is looking for what they wrote before they are
	// looking at what sits beside it.
	File bool

	// FrontmatterUnreadable marks a note whose frontmatter was there and could
	// not be parsed, which is a different thing from a note that declares
	// nothing. Both arrive here with every field empty, and a count that
	// cannot tell them apart has to describe them with one sentence that is
	// wrong about one of them.
	FrontmatterUnreadable bool
}

// entry is one indexed note. The *Fold copies double the note's text in memory
// (a few MB across the corpus) to buy a zero-config, allocation-free match at
// query time — Title/PlainText keep their display form (NFC, original case),
// the *Fold fields are fold()ed for matching, and the structured field values
// are stored NFC-but-case-preserving so a filter is an exact selection of a
// canonical value. There is no content hash: change detection is the
// scanner's job by mtime.
type entry struct {
	RelPath string
	// PathFold is the note's own location, folded the way the text is. A reader
	// who types the name of a folder standing in their own sidebar and is told
	// there is nothing there reads that as the search being broken, and three
	// of them did. Their location is something they wrote; it belongs in what
	// they can find their notes by.
	PathFold        string
	Title           string
	TitleFold       string
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

// Index is the whole in-memory search index: entries kept sorted by RelPath so
// each result bucket is naturally rel_path-ordered without a sort call.
// Read-only once built. Each entry records whether its frontmatter is instance
// metadata, while the index records whether metadata projections are available
// at all. Entries are held by pointer so a query iterates 8-byte pointers rather
// than copying each ~180-byte entry.
type Index struct {
	entries []*entry
	policy  schema.ArtifactPolicy
}

// ErrMetadataUnavailable identifies a query or aggregate that requires
// instance metadata while the artifact policy is unavailable.
var ErrMetadataUnavailable = errors.New("search metadata unavailable")

// metadataUnavailableError carries the whole declaration outcome, not a
// finished sentence: the reason and the loader's own error are what let the
// page say why in the language its reader chose. Error stays the operator's
// line, for a log and for a caller that only prints.
type metadataUnavailableError struct {
	claim schema.Claim
}

func (e metadataUnavailableError) Error() string {
	return e.claim.Diagnostic()
}

func (e metadataUnavailableError) Unwrap() error {
	return ErrMetadataUnavailable
}

func (idx *Index) metadataUnavailableError() error {
	return metadataUnavailableError{claim: idx.policy.Claim()}
}

// NewIndex builds an Index from already-extracted note data and a startup-
// derived artifact policy, with no disk access. Every document remains in the
// text and folder corpus; policy marks which entries may answer metadata
// projections. Entries are sorted by RelPath at build time, which is the sole
// source of result ordering. docs is expected to carry one entry per RelPath;
// the sort is stable so that if a caller ever violates that and passes two
// documents sharing a RelPath, their relative order is at least their input
// order rather than an unspecified one.
func NewIndex(docs []Document, policy schema.ArtifactPolicy) *Index {
	entries := make([]*entry, 0, len(docs))
	for i := range docs {
		e := entryFromDocument(&docs[i], policy)
		entries = append(entries, &e)
	}
	slices.SortStableFunc(entries, func(a, b *entry) int {
		return cmp.Compare(a.RelPath, b.RelPath)
	})
	return &Index{entries: entries, policy: policy}
}

// WithArtifactPolicy returns a read-only functional copy bound to policy. The
// caller must supply a point-in-time capture of the same artifact authority
// from which idx was built; the entries remain shared because they are
// immutable. Snapshot uses this to bind every metadata query in one request to
// the exact authority capture used by that request's other projections.
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
	return entry{
		RelPath:   d.RelPath,
		PathFold:  fold(vault.NormalizeNFC(d.RelPath)),
		Title:     title,
		TitleFold: fold(title),
		NoteType:  vault.NormalizeNFC(d.NoteType),
		Domain:    vault.NormalizeNFC(d.Domain),
		Status:    vault.NormalizeNFC(d.Status),
		Slug:      vault.NormalizeNFC(d.Slug),
		Topics:    topics,
		PlainText: plain,
		PlainFold: fold(plain),
		isFile:    d.File,
		// An unclaimed policy excludes nothing, so every readable note answers
		// metadata queries over its own raw frontmatter. Whether that raw value
		// may be *presented* as a lifecycle state is a separate question, asked
		// by the surface that renders it.
		//
		// A file has no frontmatter to be an instance of, so it answers no
		// metadata projection under any policy. That is also what keeps a tally
		// over a governed vault the same tally it was before files were indexed.
		metadataCapable:       !d.File && policy.Trustworthy() && !policy.IsNonInstance(d.RelPath),
		frontmatterUnreadable: d.FrontmatterUnreadable,
	}
}

// DocumentFromNote extracts a Document from a parsed note: the structured fields from
// frontmatter and PlainText from the render AST. A note with malformed
// frontmatter (Frontmatter == nil) simply contributes empty structured fields;
// its body text is still indexed.
func DocumentFromNote(n *vault.Note) Document {
	return Document{
		RelPath:   n.RelPath,
		Title:     n.Title(),
		NoteType:  n.Type(),
		Domain:    stringField(n, "domain"),
		Status:    n.Status(),
		Slug:      stringField(n, "slug"),
		Topics:    stringsField(n, "topics"),
		PlainText: render.PlainText(n.Body),
		// A diagnostic here means the block was present and did not parse. A
		// note that simply carries no frontmatter has none, and is not this.
		FrontmatterUnreadable: n.FMDiagnostic != "",
	}
}

// DocumentFromFile builds the index entry for a vault file that is not a note.
// Its title is the file's own name, which is the only name it has, and its whole
// text is its body — exactly the characters its page shows, so a term found here
// is a term the reader can find again on the page this hit opens.
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

// CountByStatus tallies indexed notes by their canonical (NFC) status in a
// single pass; notes with no status land in the "" bucket. It is the primitive
// Home's Lifecycle block uses to show a live count beside each schema
// status, instead of running a full Search per status value. The status
// vocabulary the caller displays still comes from the schema contract;
// this only counts metadata-capable entries the index already holds. An
// artifact policy that was declared and could not be honoured returns
// ErrMetadataUnavailable with its contract diagnostic instead of a misleading
// empty map; one that was never declared excludes nothing and counts normally.
func (idx *Index) CountByStatus() (map[string]int, error) {
	if !idx.policy.Trustworthy() {
		return nil, idx.metadataUnavailableError()
	}
	counts := make(map[string]int, len(idx.entries))
	for _, e := range idx.entries {
		if !e.metadataCapable {
			continue
		}
		counts[e.Status]++
	}
	return counts, nil
}

// TypeStatus is a note's (type, status) pair. Which onward transitions a status
// allows depends on the note type, so a caller that weighs those transitions
// needs both together — something CountByStatus, keyed on status alone, cannot
// supply.
type TypeStatus struct {
	Type   string
	Status string
}

// CountByTypeStatus tallies metadata-capable notes by their (type, status) pair
// in a single pass. It is the primitive the reading page uses to weigh each note's
// onward transitions against the schema contract without re-reading the vault:
// the transition rules key on type as well as status, so the flatter
// CountByStatus does not carry enough. A note missing either field lands in that
// field's "" bucket. An artifact policy that was declared and could not be
// honoured returns ErrMetadataUnavailable with its contract diagnostic.

// CountUnreadableFrontmatter reports how many of the notes this index counts
// had a frontmatter block that could not be parsed.
//
// It answers over exactly the entries CountByTypeStatus answers over, because
// its whole purpose is to divide that tally's empty bucket: a note whose
// frontmatter could not be read and a note that simply declares no status both
// land there, and they are not the same thing to a reader — one has to be
// repaired before anything about it can be judged, the other may be perfectly
// legal for its kind. Counting them here rather than at the caller is what
// keeps the two numbers describing the same population.
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
// contract rules on. It is the row form of CountByTypeStatus: the same notes,
// named rather than tallied, for a caller that has to reach them and not only
// count them.
type StatusHolder struct {
	RelPath string
	Type    string
	Status  string
}

// StatusHolders lists every metadata-capable note carrying a status, in the
// index's own RelPath order, so a caller can rule on each one against the
// contract and reach the file it found. It applies exactly the tests
// CountByTypeStatus applies, and returns the same notes that tally counts —
// a page deriving both a number and a list from one call cannot state a
// number the list below it does not fill.
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

// stringField reads a string frontmatter value, empty when absent or not a
// string (a malformed field costs that field, never the build).
func stringField(n *vault.Note, key string) string {
	if v, ok := n.Frontmatter[key].(string); ok {
		return v
	}
	return ""
}

// stringsField reads a list-of-strings frontmatter value (e.g. topics),
// tolerating any non-list-of-strings shape by returning nil.
func stringsField(n *vault.Note, key string) []string {
	raw, ok := n.Frontmatter[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
