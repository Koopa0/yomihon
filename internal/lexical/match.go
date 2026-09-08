package lexical

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Result is one search hit: the note's path, display title, optional status
// badge, and a snippet centered on the earliest matched-token offset. Status is
// empty when the hit is not metadata-capable.
type Result struct {
	RelPath string
	Title   string
	Status  string
	Snippet string

	// Alias is the name that answered the query when it was not the title, in
	// the note's own spelling, and empty otherwise. The row shows a title, so
	// without it a reader who typed another name sees no reason for the hit.
	Alias string

	// Topic is the declared subject that answered the query when title, alias
	// and body did not, in the note's own spelling, and empty otherwise. The
	// row shows a title, so without it a topic-only hit names nothing the
	// reader typed.
	Topic string

	// NoteType is the note's own declared type, carried beside Status because a
	// status value is declared per type. It is blanked with Status when the entry
	// may not answer metadata projections.
	NoteType string

	// File marks a hit that is not a note but a vault file read as characters.
	// It carries no lifecycle state and never will, so a surface that dresses a
	// hit in note furniture can tell the two apart.
	File bool

	// Landing is the first-block stretch a browser text directive can find
	// for a body hit, whitespace-collapsed the way a snippet is. Empty when
	// the row has no body match to point at, or when a crossing match has
	// nothing locatable in its first block.
	Landing string

	// LandingEnd is the last-block stretch of a crossing match. A directive
	// that names both ends can span blocks; a bare first-block term would
	// land on an earlier copy of the same word.
	LandingEnd string

	// BlockCrossing reports that the match continues past that first block,
	// so a directive built from the whole phrase would find nothing.
	BlockCrossing bool

	// Source reports that the deciding excerpt is a fenced block: the words
	// live only there. The row names that the way it names an alias or a
	// topic; the excerpt itself stays the fence's own lines.
	Source bool
}

const (
	// snippetBefore/snippetAfter bound the snippet window around the earliest
	// matched-token offset, counted in characters the reader sees. A byte budget
	// paid out by script, buying a reader of Chinese a third of the context.
	snippetBefore = 40
	snippetAfter  = 160
)

// SearchN runs a parsed query against the index and returns results in the final
// deterministic order, eight groups concatenated: a note's title hits, a note's
// body hits, a note's topic hits, the same three over vault files that are not
// notes, then the path-only hits, notes again before files. Each group keeps
// the vault's reading order, except that a fold-equal exact title leads the
// title-note group, and every text hit outranks every path-only hit.
//
// An empty query returns nothing and a pure-filter query lands every match in
// the title bucket. A metadata filter excludes non-instance artifacts, and
// returns ErrMetadataUnavailable when the artifact policy was declared and could
// not be honoured. At most limit results are materialized, a negative limit all
// of them; total counts every hit, and the tail beyond limit is never built.
func (idx *Index) SearchN(q *Query, limit int) (results []Result, total int, err error) {
	if len(q.tokens) == 0 && len(q.filters) == 0 {
		return nil, 0, nil
	}
	metadataAvailable := idx.policy.Trustworthy()
	requiresMetadata := q.RequiresMetadata()
	if requiresMetadata && !metadataAvailable {
		return nil, 0, idx.metadataUnavailableError()
	}
	var answers resultBuckets
	for _, e := range idx.entries {
		if requiresMetadata && !e.metadataCapable {
			continue
		}
		if !e.matchesFilters(q.filters) {
			continue
		}
		answers.place(e, q.tokens)
	}
	answers.raiseExactTitles(q.tokens)
	hits := answers.ordered()
	total = len(hits)
	if limit >= 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	results = make([]Result, len(hits))
	for i, h := range hits {
		results[i] = h.entry.result(q.tokens, h.bodyEvidence, metadataAvailable, h.alias, h.topic)
	}
	return results, total, nil
}

// hit is one matched entry before its Result is built. Buckets hold hits
// rather than Results so a bounded search can count every match while
// building — snippet included — only the results it will return.
type hit struct {
	entry        *entry
	bodyEvidence bool
	// alias is the name that answered the query when it was not the title, in
	// the note's own spelling, since the row itself shows a title.
	alias string
	// topic is the declared subject that answered the query when title, alias
	// and body did not.
	topic string
}

// bucket names one answer group. This declaration order is the result order and
// the only statement of it, so a group moves by moving its constant: notes before
// files under each kind of evidence, and title, body and topic before an address.
type bucket uint8

const (
	titleNote bucket = iota
	bodyNote
	topicNote
	titleFile
	bodyFile
	topicFile
	pathNote
	pathFile
	bucketCount
)

// resultBuckets keeps the answer groups apart while one pass fills them, so the
// final order is a concatenation rather than a sort of the whole answer.
type resultBuckets struct {
	groups [bucketCount][]hit
}

// place files one filter-matching entry into its answer group by what the
// tokens matched: the title, the body, a declared topic, or only the path.
func (b *resultBuckets) place(e *entry, tokens []string) {
	switch {
	case allContain(e.TitleFold, tokens):
		bodyEvidence := len(tokens) != 0 && allContain(e.PlainFold, tokens)
		b.add(titleNote, titleFile, hit{entry: e, bodyEvidence: bodyEvidence})
	case aliasAnswering(e, tokens) != "":
		// An alias stands with the title: a link written to one resolves and a
		// link written to a title does not.
		bodyEvidence := len(tokens) != 0 && allContain(e.PlainFold, tokens)
		b.add(titleNote, titleFile, hit{entry: e, bodyEvidence: bodyEvidence, alias: aliasAnswering(e, tokens)})
	case allContain(e.PlainFold, tokens):
		b.add(bodyNote, bodyFile, hit{entry: e, bodyEvidence: true})
	case topicAnswering(e, tokens) != "":
		// A topic is a name the note declared for retrieval. It ranks below
		// body so a mention in prose stays above the many notes that share a
		// subject.
		b.add(topicNote, topicFile, hit{entry: e, topic: topicAnswering(e, tokens)})
	case allContain(e.PathFold, tokens):
		b.add(pathNote, pathFile, hit{entry: e})
	}
}

// add files one hit under the group its evidence and its kind put it in. The hit
// carries which kind it is, so the caller names the pair and never chooses.
func (b *resultBuckets) add(note, file bucket, h hit) {
	g := note
	if h.entry.isFile {
		g = file
	}
	b.groups[g] = append(b.groups[g], h)
}

// raiseExactTitles is the one tie-break inside the title-note group: a title
// that is the query, under the same fold matching uses, leads every title that
// merely contains it. Hits that share that answer keep the vault's reading
// order. The other seven groups are not touched, and an empty token list is a
// pure-filter query whose every match already sits here.
func (b *resultBuckets) raiseExactTitles(tokens []string) {
	if len(tokens) == 0 {
		return
	}
	needle := strings.Join(tokens, " ")
	slices.SortStableFunc(b.groups[titleNote], func(left, right hit) int {
		return cmp.Compare(exactTitleRank(right.entry.TitleFold, needle), exactTitleRank(left.entry.TitleFold, needle))
	})
}

func exactTitleRank(titleFold, needle string) int {
	if titleFold == needle {
		return 1
	}
	return 0
}

// ordered flattens the groups into the answer, in the order the constants are
// declared in.
func (b *resultBuckets) ordered() []hit {
	return slices.Concat(b.groups[:]...)
}

// topicAnswering returns the note's own spelling of the first declared topic
// that holds every token, or empty when none does. Matching reads TopicFolds;
// the as-written Topics value is what the row shows. Empty tokens are a
// pure-filter query and already land in the title group.
func topicAnswering(e *entry, tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	for i, folded := range e.TopicFolds {
		if allContain(folded, tokens) {
			return e.Topics[i]
		}
	}
	return ""
}

// aliasAnswering returns the note's own spelling of the first alias that holds
// every token, or empty when none does. The first is taken because an author
// listing several puts the one they think of first at the front.
func aliasAnswering(e *entry, tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	for i, folded := range e.AliasFolds {
		if allContain(folded, tokens) {
			return e.Aliases[i]
		}
	}
	return ""
}

// allContain reports whether hay contains every token (AND). Tokens are folded
// and hay is a *Fold field, so this is a literal substring test with no wildcards,
// except that whitespace in a token matches any whitespace. Zero tokens is true.
func allContain(hay string, tokens []string) bool {
	for _, t := range tokens {
		if start, _ := phraseIndex(hay, t, 0); start < 0 {
			return false
		}
	}
	return true
}

// phraseIndex reports the byte range of the first occurrence of token in hay at or
// after from, treating every run of whitespace inside token as a match for any run
// of whitespace in hay; start is -1 when token does not occur. That flexibility is
// what lets a quoted phrase be answered at all, since this vault's prose is
// hard-wrapped and the indexed text keeps the breaks. Adjacency is still required,
// and one block is parted from the next by a break too, so a phrase can join them.
func phraseIndex(hay, token string, from int) (start, end int) {
	if from < 0 || from > len(hay) {
		return -1, -1
	}
	head := token[:wordRun(token, 0)]
	if len(head) == len(token) {
		i := strings.Index(hay[from:], token)
		if i < 0 {
			return -1, -1
		}
		return from + i, from + i + len(token)
	}
	for at := from; at <= len(hay); {
		i := strings.Index(hay[at:], head)
		if i < 0 {
			return -1, -1
		}
		i += at
		if stop, ok := phraseAt(hay, token, i); ok {
			return i, stop
		}
		at = i + 1
	}
	return -1, -1
}

// phraseAt matches token against hay starting at pos, taking each run of
// whitespace in token as a demand for whitespace rather than for those exact
// characters, and reports where the match ends.
func phraseAt(hay, token string, pos int) (end int, ok bool) {
	for t := 0; t < len(token); {
		if run := whitespaceRun(token, t); run > 0 {
			t += run
			run = whitespaceRun(hay, pos)
			if run == 0 {
				return 0, false
			}
			pos += run
			continue
		}
		word := token[t : t+wordRun(token, t)]
		if !strings.HasPrefix(hay[pos:], word) {
			return 0, false
		}
		pos += len(word)
		t += len(word)
	}
	return pos, true
}

// whitespaceRun reports the byte length of the run of whitespace beginning at
// pos, zero when nothing there is whitespace.
func whitespaceRun(s string, pos int) int {
	n := 0
	for pos+n < len(s) {
		r, size := utf8.DecodeRuneInString(s[pos+n:])
		if !unicode.IsSpace(r) {
			break
		}
		n += size
	}
	return n
}

// wordRun reports the byte length of the run of non-whitespace beginning at
// pos, zero when whitespace begins there.
func wordRun(s string, pos int) int {
	n := 0
	for pos+n < len(s) {
		r, size := utf8.DecodeRuneInString(s[pos+n:])
		if unicode.IsSpace(r) {
			break
		}
		n += size
	}
	return n
}

// matchesFilters reports whether e satisfies every filter (a repeated key is
// therefore an AND: two "type:" filters both must hold, so they are jointly
// unsatisfiable rather than last-wins).
func (e *entry) matchesFilters(filters []Filter) bool {
	for _, f := range filters {
		if !e.matchesFilter(f) {
			return false
		}
	}
	return true
}

// matchesFilter reports whether e satisfies one filter. type/status/domain/slug
// compare the folded copies; topic is folded membership of TopicFolds; folder is a
// folded rel_path prefix at a "/" boundary, so "folder:Writing" matches
// "Writing" and "Writing/x.md" and "writing/x.md", but never "Writing-old/x.md".
func (e *entry) matchesFilter(f Filter) bool {
	switch f.Key {
	case "type":
		return e.NoteTypeFold == f.Value
	case "status":
		return e.StatusFold == f.Value
	case "domain":
		return e.DomainFold == f.Value
	case "slug":
		return e.SlugFold == f.Value
	case "topic":
		return slices.Contains(e.TopicFolds, f.Value)
	case "folder":
		return e.PathFold == f.Value || strings.HasPrefix(e.PathFold, f.Value+"/")
	default:
		// A filter reaches this only where Parse recognized its key, and Parse
		// recognizes exactly the keys the grammar table holds; a Query keeps
		// its filters unexported, so nothing else can introduce one. What this
		// arm cannot catch is a key added to that table with no arm here: it
		// would parse, be offered on the page, and match nothing without
		// saying so. The two sets are compared in a test instead.
		return false
	}
}

// result builds a Result for e, with a snippet centered on the earliest
// matched-token offset.
func (e *entry) result(tokens []string, bodyEvidence, metadataAvailable bool, alias, topic string) Result {
	status, noteType := e.Status, e.NoteType
	if !metadataAvailable || !e.metadataCapable {
		status, noteType = "", ""
	}
	var bodySnippet, landing, landingEnd string
	var crossing, fromFence bool
	if bodyEvidence {
		var foldStart, foldEnd int
		foldStart, foldEnd, fromFence = earliestOffset(e.PlainFold, tokens, e.fenceFoldRanges)
		bodySnippet = snippetAt(e.PlainText, foldStart, foldEnd, e.fenceRanges)
		landing, landingEnd, crossing = e.landingAt(foldStart, foldEnd)
	}
	return Result{
		RelPath:       e.RelPath,
		Title:         e.Title,
		Status:        status,
		Snippet:       bodySnippet,
		Alias:         alias,
		Topic:         topic,
		NoteType:      noteType,
		File:          e.isFile,
		Landing:       landing,
		LandingEnd:    landingEnd,
		BlockCrossing: crossing,
		Source:        fromFence,
	}
}

// landingAt is the first and last block of one folded body match, and whether
// that match continues into a later block. The exclusive end is the last
// matched rune plus its length: the next kept rune can sit in a later block
// after a fold-dropped break, which is not itself a crossing. Without recorded
// block ends there is nothing to tell a wrap from a paragraph, so the row
// keeps the snippet's own words and does not claim a crossing.
func (e *entry) landingAt(foldStart, foldEnd int) (first, last string, crossing bool) {
	if foldStart < 0 || foldEnd <= foldStart || len(e.blockEnds) == 0 {
		return "", "", false
	}
	start := sourceOffsetOfFold(e.PlainText, foldStart)
	end := sourceEndOfFold(e.PlainText, foldEnd)
	if start >= end || end > len(e.PlainText) {
		return "", "", false
	}
	firstEnd := e.blockEndAfter(start)
	crossing = end > firstEnd
	firstStop := end
	if crossing {
		firstStop = firstEnd
	}
	first = collapseFields(e.PlainText[start:firstStop])
	if !crossing {
		return first, "", false
	}
	from := max(e.blockStartContaining(end-1), firstEnd)
	return first, collapseFields(e.PlainText[from:end]), true
}

func collapseFields(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func (e *entry) blockEndAfter(off int) int {
	for _, end := range e.blockEnds {
		if end > off {
			return end
		}
	}
	return len(e.PlainText)
}

func (e *entry) blockStartContaining(off int) int {
	prev := 0
	for _, end := range e.blockEnds {
		if end > off {
			return prev
		}
		prev = end
	}
	return prev
}

// runesBefore returns the byte offset n characters back from off, and the start
// of the text when there are fewer than n. It walks characters rather than
// subtracting bytes so the window is the same size to every reader.
func runesBefore(s string, off, n int) int {
	if off > len(s) {
		off = len(s)
	}
	for range n {
		if off <= 0 {
			return 0
		}
		_, size := utf8.DecodeLastRuneInString(s[:off])
		off -= size
	}
	return off
}

// runesAfter returns the byte offset n characters forward from off, and the end
// of the text when there are fewer than n.
func runesAfter(s string, off, n int) int {
	if off < 0 {
		off = 0
	}
	for range n {
		if off >= len(s) {
			return len(s)
		}
		_, size := utf8.DecodeRuneInString(s[off:])
		off += size
	}
	return off
}

// snippetAt returns a one-line window of plain around a folded match.
// Lowercasing does not preserve length, so the offsets come back through the
// fold's own mapping: used directly they drift until the window slides clear
// of the term it was placed around.
func snippetAt(plain string, foldStart, foldEnd int, fences [][2]int) string {
	if foldStart < 0 {
		foldStart, foldEnd = 0, 0
	}
	off := sourceOffsetOfFold(plain, foldStart)
	matchEnd := sourceEndOfFold(plain, foldEnd)
	matchEnd = max(matchEnd, off)
	matchEnd = min(matchEnd, len(plain))
	// Neither boundary may move past the match it was placed around: a match buried
	// deep in one unbroken run can be stepped over by both at once, reversing the
	// slice. The close is held at the match's exclusive end, not its first byte —
	// clamping to off made the half-open window exclude the hit (plain "0Z"+184×"0",
	// token "Z" → "0…"). The sentence-start reach runs first and the whole-word
	// adjustment last, because the second has to hold whatever the first leaves.
	// A fence hit skips the sentence reach: source is not a sentence, and walking
	// back through a preceding paragraph would present it as one.
	opening := runesBefore(plain, off, snippetBefore)
	inFence, fence := fenceAt(off, fences)
	if !inFence {
		opening = sentenceStart(plain, opening, off)
	}
	start := min(wholeWordStart(plain, opening), off)
	end := max(wholeWordEnd(plain, runesAfter(plain, off, snippetAfter)), matchEnd)
	if inFence {
		start = max(start, fence[0])
		end = min(end, fence[1])
		start = min(start, off)
		end = max(end, matchEnd)
	} else {
		start, end = clipFencesFromProse(plain, start, end, off, matchEnd, fences)
	}

	s := collapseFields(plain[start:end])
	if start > 0 {
		s = "…" + s
	}
	if end < len(plain) {
		s += "…"
	}
	return s
}

func snippet(plain, plainFold string, tokens []string) string {
	return snippetWithFences(plain, plainFold, tokens, nil)
}

func snippetWithFences(plain, plainFold string, tokens []string, fences [][2]int) string {
	foldStart, foldEnd, _ := earliestOffset(plainFold, tokens, foldRanges(plain, fences))
	return snippetAt(plain, foldStart, foldEnd, fences)
}

// earliestOffset returns the folded byte range of the token that should
// centre the excerpt: the earliest prose hit when the note has one, otherwise
// the earliest fence hit. start is < 0 when no token occurs. inFence is
// true only when the words live only in a fence.
func earliestOffset(fold string, tokens []string, foldFences [][2]int) (start, end int, inFence bool) {
	proseStart, proseEnd := -1, 0
	fenceStart, fenceEnd := -1, 0
	for _, t := range tokens {
		ps, pe, fs, fe := tokenWindows(fold, t, foldFences)
		if ps >= 0 && (proseStart < 0 || ps < proseStart) {
			proseStart, proseEnd = ps, pe
		}
		if fs >= 0 && (fenceStart < 0 || fs < fenceStart) {
			fenceStart, fenceEnd = fs, fe
		}
	}
	if proseStart >= 0 {
		return proseStart, proseEnd, false
	}
	return fenceStart, fenceEnd, fenceStart >= 0
}

// tokenWindows is one token's earliest prose hit and earliest fence hit.
// A later occurrence of the same token cannot sit earlier than the first
// of each kind, so the scan stops once both are known or the text ends.
// Membership is tested in fold space against spans tabulated at index time.
func tokenWindows(fold, token string, foldFences [][2]int) (proseStart, proseEnd, fenceStart, fenceEnd int) {
	proseStart, fenceStart = -1, -1
	for at := 0; at <= len(fold); {
		i, stop := phraseIndex(fold, token, at)
		if i < 0 {
			return proseStart, proseEnd, fenceStart, fenceEnd
		}
		if len(foldFences) > 0 && inFenceRange(i, foldFences) {
			if fenceStart < 0 {
				fenceStart, fenceEnd = i, stop
			}
		} else {
			proseStart, proseEnd = i, stop
			return proseStart, proseEnd, fenceStart, fenceEnd
		}
		at = max(stop, i+1)
	}
	return proseStart, proseEnd, fenceStart, fenceEnd
}

func inFenceRange(off int, fences [][2]int) bool {
	found, _ := fenceAt(off, fences)
	return found
}

func fenceAt(off int, fences [][2]int) (found bool, span [2]int) {
	for _, f := range fences {
		if off >= f[0] && off < f[1] {
			return true, f
		}
	}
	return false, [2]int{}
}

// clipFencesFromProse keeps a prose window from swallowing a fence: a fence
// that overlaps the opening side pushes start forward, and one that overlaps
// the close pulls end back. The match itself stays inside.
func clipFencesFromProse(plain string, start, end, off, matchEnd int, fences [][2]int) (clippedStart, clippedEnd int) {
	for _, f := range fences {
		if f[1] <= start || f[0] >= end {
			continue
		}
		if f[1] <= off {
			after := f[1]
			if after < len(plain) && plain[after] == '\n' {
				after++
			}
			if after > start {
				start = after
			}
			continue
		}
		if f[0] >= matchEnd && f[0] < end {
			end = f[0]
		}
	}
	return min(start, off), max(end, matchEnd)
}

// wordEdgeBudget bounds how far a boundary may move to keep a word whole. A
// date or an identifier fits inside it; a run longer than this is not a word
// anyone is reading as one, and the window matters more than it does.
const wordEdgeBudget = 24

// wholeWordStart moves a snippet's opening boundary back to the start of a word
// the window cut into, so a date arrives as 2026-07-30 rather than 026-07-30. Only
// runs of ASCII letters and digits are treated this way, since CJK prose has no
// word boundary to respect. It moves outward, since forward leaves a fragment.
func wholeWordStart(s string, i int) int {
	if i <= 0 || i >= len(s) {
		return i
	}
	if !isWordByte(s[i-1]) || !isWordByte(s[i]) {
		return i
	}
	j := i
	for j > 0 && isWordByte(s[j-1]) && i-j < wordEdgeBudget {
		j--
	}
	if j > 0 && isWordByte(s[j-1]) {
		// The run outran the budget, so it is not a word worth keeping whole;
		// step off its tail instead of dragging it in.
		for i < len(s) && isWordByte(s[i]) {
			i++
		}
		return i
	}
	return j
}

// wholeWordEnd is the same adjustment at the closing boundary.
func wholeWordEnd(s string, i int) int {
	if i <= 0 || i >= len(s) {
		return i
	}
	if !isWordByte(s[i-1]) || !isWordByte(s[i]) {
		return i
	}
	j := i
	for j < len(s) && isWordByte(s[j]) && j-i < wordEdgeBudget {
		j++
	}
	if j < len(s) && isWordByte(s[j]) {
		for i > 0 && isWordByte(s[i-1]) {
			i--
		}
		return i
	}
	return j
}

// isWordByte reports whether b belongs to a run the reader reads as one thing.
// Hyphen and underscore are in it because the runs that arrive mangled are dates
// and identifiers, which splitting at the punctuation would cut to their tail.
func isWordByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '-' || b == '_'
}

// sentenceTerminators end a sentence in the scripts this corpus is written in; a
// comma of either width is not among them. The unambiguous ones are a subset:
// a full-width stop appears inside no token and ends a sentence wherever it
// stands, while an ASCII stop is the character inside vault-schema.toml and
// 3.14159, so it ends one only with white space or the end of the text after
// it. The two sets are all that is needed — every terminator outside the
// unambiguous set is by definition one of the ASCII ones, and lastSentenceEnd
// asks for the white space there.
const (
	sentenceTerminators    = "。！？；.!?;\n"
	unambiguousTerminators = "。！？；\n"
)

// snippetBeforeMax bounds the whole opening side of the window: the boundary may
// travel back this far to reach the start of its sentence and no further, so a
// note written without terminators cannot turn one result row into the note.
const snippetBeforeMax = 120

// sentenceStart moves a snippet's opening boundary back to the beginning of the
// sentence it landed inside, when that beginning is within reach. The words a
// sentence opens with decide what it means — 不得, 本段僅限, "this is not
// recommended" — and cutting them leaves the predicate reading as the instruction.
// Where no terminator is in reach the window is left where it was.
func sentenceStart(plain string, start, off int) int {
	if start <= 0 {
		return start
	}
	limit := runesBefore(plain, off, snippetBeforeMax)
	if limit >= start {
		return start
	}
	i, ok := lastSentenceEnd(plain, limit, start)
	if !ok {
		return start
	}
	for i < start {
		r, width := utf8.DecodeRuneInString(plain[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += width
	}
	return i
}

// lastSentenceEnd reports the byte just past the last sentence-ending punctuation
// in plain[limit:start], and whether there was one. An ASCII stop counts only when
// white space or the end of the text follows, which excludes a filename's dot.
func lastSentenceEnd(plain string, limit, start int) (int, bool) {
	for at := start; at > limit; {
		j := strings.LastIndexAny(plain[limit:at], sentenceTerminators)
		if j < 0 {
			return 0, false
		}
		i := limit + j
		r, size := utf8.DecodeRuneInString(plain[i:])
		after := i + size
		if strings.ContainsRune(unambiguousTerminators, r) {
			return after, true
		}
		if after >= len(plain) {
			return after, true
		}
		if next, _ := utf8.DecodeRuneInString(plain[after:]); unicode.IsSpace(next) {
			return after, true
		}
		at = i
	}
	return 0, false
}

// foldWithSourceOffsets folds s and maps every byte position of the folded
// copy, one past its end included, back to the byte offset in s of the character
// it came from. It applies the walk half of the index's fold alone, which
// reproduces that fold provided s is already NFC, as every snippet is.
func foldWithSourceOffsets(s string) (fold string, src []int) {
	var folded strings.Builder
	folded.Grow(len(s))
	src = make([]int, 0, len(s)+1)
	foldRunes(s, func(r rune, at int) {
		n := folded.Len()
		folded.WriteRune(r)
		for ; n < folded.Len(); n++ {
			src = append(src, at)
		}
	})
	return folded.String(), append(src, len(s))
}

// sourceOffsetOfFold maps one byte offset in the folded copy of s back to the
// byte offset in s of the character that produced it. It answers exactly what
// foldWithSourceOffsets tabulates, walked to one position rather than materialized
// because its caller measures a whole note. A test holds the two to one answer.
func sourceOffsetOfFold(s string, foldOff int) int {
	folded, at := 0, len(s)
	found := false
	foldRunes(s, func(r rune, i int) {
		if found {
			return
		}
		next := folded + utf8.RuneLen(r)
		if next > foldOff {
			at, found = i, true
			return
		}
		folded = next
	})
	return at
}

// sourceEndOfFold maps the exclusive end of a folded match back to the exclusive
// source offset of its last rune. sourceOffsetOfFold at that same fold end would
// name the next kept rune, which can sit past a dropped break and is not the
// match.
func sourceEndOfFold(s string, foldEnd int) int {
	if foldEnd <= 0 {
		return 0
	}
	folded, end := 0, len(s)
	found := false
	foldRunes(s, func(r rune, i int) {
		if found {
			return
		}
		next := folded + utf8.RuneLen(r)
		if next >= foldEnd {
			_, size := utf8.DecodeRuneInString(s[i:])
			end, found = i+size, true
			return
		}
		folded = next
	})
	return end
}

// HitRun is one stretch of a piece of text and whether the query matched it.
// The runs of one text cover it exactly once, in order, so a caller can render
// them straight through without consulting an offset.
type HitRun struct {
	Text string
	Hit  bool
}

// MarkHits cuts a piece of text into the stretches that matched and those that did
// not, so a page can show why a result is here. Nothing in it is particular to a
// snippet: the same cut serves the body excerpt, the path, and any other name a
// note can match by. Matching is done on the folded form while the runs carry
// slices of the original text, and overlapping matches merge into one mark.
func MarkHits(snippet string, tokens []string) []HitRun {
	if snippet == "" || len(tokens) == 0 {
		return nil
	}
	fold, src := foldWithSourceOffsets(snippet)
	covered := make([]bool, len(snippet))
	found := false
	for _, t := range tokens {
		if t == "" {
			continue
		}
		for at := 0; at <= len(fold); {
			i, stop := phraseIndex(fold, t, at)
			if i < 0 {
				break
			}
			for j := src[i]; j < src[stop]; j++ {
				covered[j] = true
			}
			found = true
			at = max(stop, i+1)
		}
	}
	if !found {
		return nil
	}
	var runs []HitRun
	start := 0
	for i := 1; i <= len(snippet); i++ {
		if i < len(snippet) && covered[i] == covered[start] {
			continue
		}
		runs = append(runs, HitRun{Text: snippet[start:i], Hit: covered[start]})
		start = i
	}
	return runs
}
