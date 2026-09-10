// Package lexical is the vault's in-memory search index and query engine. It
// holds one entry per note and answers a deterministic, folded substring
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
	"unicode/utf8"

	"golang.org/x/text/width"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// fold is the single definer of "what counts as a match": NFC, then the walk
// below, applied identically to stored text and to a query token. Case and
// width folding live only here; vault.NormalizeNFC supplies the shared NFC step.
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
		emit(foldRune(r), i)
		prev = r
	}
}

// fullwidthASCIIMin and fullwidthASCIIMax bound the block foldRune narrows.
// Each of those runes has a one-rune halfwidth counterpart, so the fold's
// source-offset walk stays a walk of one emitted rune per source rune.
// Halfwidth katakana is outside it: a voiced mark beside one folds two runes
// to one and is a separate decision.
const (
	fullwidthASCIIMin = 0xFF01
	fullwidthASCIIMax = 0xFF5E
)

// foldRune is the per-character half of fold: the fullwidth ASCII block
// narrows to its halfwidth counterpart, then simple lowercase. Width first so
// a fullwidth letter and its ASCII counterpart meet before either is lowered.
func foldRune(r rune) rune {
	if r >= fullwidthASCIIMin && r <= fullwidthASCIIMax {
		if n := width.LookupRune(r).Narrow(); n != 0 {
			r = n
		}
	}
	return unicode.ToLower(r)
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
// (UAX #29) draws. Han, hiragana and katakana carry it. Hangul does not, and
// its exclusion is that script property rather than a case left for later:
// modern Korean divides its words with spaces, so closing a wrapped seam
// would fuse two words.
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
	// Topics are the subjects the note declared. A bare token reaches them
	// the way it reaches an alias: leaving them out made a note findable by
	// fewer names than it follows.
	Topics []string
	// Aliases are the other names the note declared. They are the names a
	// wikilink resolves by, so leaving them out made this program findable by
	// fewer names than it follows.
	Aliases   []string
	PlainText string

	// BlockEnds are the exclusive end offsets of each block-level contribution
	// in PlainText, as render.PlainBlocks reports them. Empty when the caller
	// built the document from already-extracted text and did not know.
	BlockEnds []int

	// FenceRanges are the half-open [start, end) spans in PlainText that came
	// from a fenced code block, as render.PlainBlocks reports them. Empty when
	// the caller did not know, which treats every hit as prose.
	FenceRanges [][2]int

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
// match. The structured values stay NFC and case-preserving for display; their
// *Fold copies are what a filter compares, so a reader who types a value the
// way they type a word still reaches it.
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
	Aliases      []string
	AliasFolds   []string
	NoteType     string
	NoteTypeFold string
	Domain       string
	DomainFold   string
	Status       string
	StatusFold   string
	Slug         string
	SlugFold     string
	// Topics are the note's declared subjects, held as written. TopicFolds
	// is what matching reads; Topics is what the result row shows, so a
	// declared Colour Theory is not rewritten as colour theory.
	Topics          []string
	TopicFolds      []string
	PlainText       string
	PlainFold       string
	blockEnds       []int
	fenceRanges     [][2]int
	fenceFoldRanges [][2]int
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
// structured field values stored NFC (case-preserving) with a folded copy
// beside each so a filter compares the way text does.
func entryFromDocument(d *Document, policy schema.ArtifactPolicy) entry {
	title := vault.NormalizeNFC(d.Title)
	plain := vault.NormalizeNFC(d.PlainText)
	// Block ends and fence bounds share one NFC walk: the two remaps used
	// to normalise the same slices twice, and a save rebuilds the index.
	// Fold-space spans are recorded on the same foldRunes walk that
	// builds PlainFold, so a query can classify a hit without walking
	// the note once per fence occurrence.
	blockEnds, fenceRanges := remapPlainOffsets(d.PlainText, d.BlockEnds, d.FenceRanges)
	plainFold, fenceFoldRanges := foldPlain(plain, fenceRanges)
	noteType := vault.NormalizeNFC(d.NoteType)
	domain := vault.NormalizeNFC(d.Domain)
	status := vault.NormalizeNFC(d.Status)
	slug := vault.NormalizeNFC(d.Slug)
	topics := make([]string, len(d.Topics))
	topicFolds := make([]string, len(d.Topics))
	for i, t := range d.Topics {
		topics[i] = vault.NormalizeNFC(t)
		topicFolds[i] = fold(topics[i])
	}
	aliases := make([]string, len(d.Aliases))
	aliasFolds := make([]string, len(d.Aliases))
	for i, a := range d.Aliases {
		aliases[i] = vault.NormalizeNFC(a)
		aliasFolds[i] = fold(aliases[i])
	}
	return entry{
		RelPath:         d.RelPath,
		PathFold:        fold(vault.NormalizeNFC(d.RelPath)),
		Title:           title,
		TitleFold:       fold(title),
		Aliases:         aliases,
		AliasFolds:      aliasFolds,
		NoteType:        noteType,
		NoteTypeFold:    fold(noteType),
		Domain:          domain,
		DomainFold:      fold(domain),
		Status:          status,
		StatusFold:      fold(status),
		Slug:            slug,
		SlugFold:        fold(slug),
		Topics:          topics,
		TopicFolds:      topicFolds,
		PlainText:       plain,
		PlainFold:       plainFold,
		blockEnds:       blockEnds,
		fenceRanges:     fenceRanges,
		fenceFoldRanges: fenceFoldRanges,
		isFile:          d.File,
		// An unclaimed policy excludes nothing, so every readable note answers over
		// its own raw frontmatter. A file has no frontmatter, so it answers no
		// metadata projection under any policy.
		metadataCapable:       !d.File && policy.Trustworthy() && !policy.IsNonInstance(d.RelPath),
		frontmatterUnreadable: d.FrontmatterUnreadable,
	}
}

// remapPlainOffsets maps exclusive block ends and half-open fence spans
// from raw onto NFC(raw) in one left-to-right pass. Each slice is
// normalised on its own and the lengths are accumulated; the caller
// stores the NFC body, so these offsets name characters there.
// Pairing reads the mapped fence slice in the same start/end order the
// ranges were flattened, so a repeated bound must not be dropped.
func remapPlainOffsets(raw string, ends []int, fences [][2]int) (blockEnds []int, fenceRanges [][2]int) {
	if len(ends) == 0 && len(fences) == 0 {
		return nil, nil
	}
	cur := newNFCCursor(raw, sortedFenceBounds(raw, fences))
	if len(ends) > 0 {
		blockEnds = make([]int, 0, len(ends)+1)
	}
	prevRaw := 0
	for _, end := range ends {
		if end < prevRaw {
			continue
		}
		if end > len(raw) {
			end = len(raw)
		}
		cur.advanceTo(end)
		blockEnds = appendUniqueEnd(blockEnds, cur.n)
		prevRaw = end
	}
	if len(ends) == 0 || prevRaw < len(raw) {
		cur.advanceTo(len(raw))
		if len(ends) > 0 {
			blockEnds = appendUniqueEnd(blockEnds, cur.n)
		}
	}
	cur.finish()
	if len(blockEnds) == 0 {
		blockEnds = nil
	}
	if len(fences) > 0 {
		fenceRanges = pairedSpans(cur.mapped)
	}
	return blockEnds, fenceRanges
}

type rawBound struct {
	off int
	i   int
}

func sortedFenceBounds(raw string, fences [][2]int) []rawBound {
	bounds := make([]rawBound, 0, 2*len(fences))
	for _, r := range fences {
		bounds = append(bounds,
			rawBound{off: clampOff(r[0], len(raw)), i: len(bounds)},
			rawBound{off: clampOff(r[1], len(raw)), i: len(bounds) + 1},
		)
	}
	slices.SortStableFunc(bounds, func(a, b rawBound) int {
		if a.off < b.off {
			return -1
		}
		if a.off > b.off {
			return 1
		}
		return 0
	})
	return bounds
}

// nfcCursor walks raw once, handing back the NFC length at each cut and
// recording fence bounds as it passes them.
type nfcCursor struct {
	raw           string
	prev, n, next int
	bounds        []rawBound
	mapped        []int
}

func newNFCCursor(raw string, bounds []rawBound) nfcCursor {
	return nfcCursor{raw: raw, bounds: bounds, mapped: make([]int, len(bounds))}
}

func (c *nfcCursor) advanceTo(off int) {
	if off > len(c.raw) {
		off = len(c.raw)
	}
	if off < c.prev {
		return
	}
	for c.next < len(c.bounds) && c.bounds[c.next].off < off {
		c.recordBound()
	}
	if off > c.prev {
		c.n += len(vault.NormalizeNFC(c.raw[c.prev:off]))
		c.prev = off
	}
	for c.next < len(c.bounds) && c.bounds[c.next].off == off {
		c.mapped[c.bounds[c.next].i] = c.n
		c.next++
	}
}

func (c *nfcCursor) recordBound() {
	b := c.bounds[c.next]
	if b.off > c.prev {
		c.n += len(vault.NormalizeNFC(c.raw[c.prev:b.off]))
		c.prev = b.off
	}
	c.mapped[b.i] = c.n
	c.next++
}

func (c *nfcCursor) finish() {
	for c.next < len(c.bounds) {
		c.mapped[c.bounds[c.next].i] = c.n
		c.next++
	}
}

func appendUniqueEnd(out []int, n int) []int {
	if n > 0 && (len(out) == 0 || out[len(out)-1] != n) {
		return append(out, n)
	}
	return out
}

func clampOff(off, n int) int {
	if off < 0 {
		return 0
	}
	if off > n {
		return n
	}
	return off
}

// fenceRangesOnNormalized maps half-open fence spans from raw onto NFC(raw)
// through the one remap production uses.
func fenceRangesOnNormalized(raw string, ranges [][2]int) [][2]int {
	_, fences := remapPlainOffsets(raw, nil, ranges)
	return fences
}

// foldPlain folds already-NFC plain and maps fence spans onto that
// folded copy in the same walk. A second foldRunes pass used to walk
// the note again just for the bounds, and kept walking past the last one.
func foldPlain(plain string, ranges [][2]int) (folded string, foldFences [][2]int) {
	var out strings.Builder
	out.Grow(len(plain))
	if len(ranges) == 0 {
		foldRunes(plain, func(r rune, _ int) {
			out.WriteRune(r)
		})
		return out.String(), nil
	}
	type bound struct {
		src int
		i   int
	}
	bounds := make([]bound, 0, 2*len(ranges))
	for _, r := range ranges {
		bounds = append(bounds,
			bound{src: r[0], i: len(bounds)},
			bound{src: r[1], i: len(bounds) + 1},
		)
	}
	slices.SortStableFunc(bounds, func(a, b bound) int {
		if a.src < b.src {
			return -1
		}
		if a.src > b.src {
			return 1
		}
		return 0
	})
	mapped := make([]int, len(bounds))
	next, foldPos := 0, 0
	foldRunes(plain, func(r rune, at int) {
		for next < len(bounds) && bounds[next].src <= at {
			mapped[bounds[next].i] = foldPos
			next++
		}
		out.WriteRune(r)
		foldPos += utf8.RuneLen(r)
	})
	for next < len(bounds) {
		mapped[bounds[next].i] = foldPos
		next++
	}
	return out.String(), pairedSpans(mapped)
}

// foldRanges maps source-space half-open spans onto the folded copy of
// plain. Production records those spans on the foldPlain walk; this
// keeps the same mapping for a snippet that was not built through NewIndex.
func foldRanges(plain string, ranges [][2]int) [][2]int {
	_, foldFences := foldPlain(plain, ranges)
	return foldFences
}

func pairedSpans(offs []int) [][2]int {
	out := make([][2]int, 0, len(offs)/2)
	for i := 0; i+1 < len(offs); i += 2 {
		if offs[i+1] > offs[i] {
			out = append(out, [2]int{offs[i], offs[i+1]})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// DocumentFromNote extracts a Document from a parsed note: the structured fields
// from frontmatter and PlainText from the render AST. A note with malformed
// frontmatter contributes empty structured fields; its body text is still indexed.
func DocumentFromNote(n *vault.Note) Document {
	text, ends, fences := render.PlainBlocks(n.Body)
	return Document{
		RelPath:     n.RelPath,
		Title:       n.Title(),
		NoteType:    n.Type(),
		Domain:      n.Domain(),
		Status:      n.Status(),
		Slug:        n.Slug(),
		Topics:      n.Strings("topics"),
		Aliases:     n.Aliases(),
		PlainText:   text,
		BlockEnds:   ends,
		FenceRanges: fences,
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
