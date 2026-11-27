package lexical

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Result is one search hit: the note's path, display title, whether it is a
// draft, the passage shown under it, and where its link lands. Draft is false
// when the hit is not metadata-capable.
type Result struct {
	RelPath string
	Title   string
	Draft   bool
	// NoteType is the type the note declares, as written, for a page that
	// names a result's kind. Empty for a note that declares none.
	NoteType string
	// Lead is the passage a result page shows under the title; see Lead.
	Lead Lead

	// Alias is the name that answered the query when it was not the title, in
	// the note's own spelling, and empty otherwise. The row shows a title, so
	// without it a reader who typed another name sees no reason for the hit.
	Alias string

	// Tag is the frontmatter tag that answered when stronger evidence did not,
	// in the author's spelling. Tag-only hits have no body excerpt or landing.
	Tag string

	// Landing is the earliest body match in its first block, with whitespace
	// collapsed and its tail grown to a word boundary. Empty when
	// the row has no body match to point at, or when a crossing match has
	// nothing locatable in its first block.
	Landing string

	// LandingBare is Landing grown out to the edges of the words it lies
	// inside, for a directive that names nothing ahead of it: a browser finds
	// such a term only where a word begins and ends. An edge in a script that
	// parts no words with spaces stays where the match left it.
	LandingBare string

	// LandingEnd is the last-block stretch of a crossing match, grown the way
	// LandingBare is, since a browser holds it to both edges whether it
	// closes a range or travels alone. A bare first-block term would land on
	// an earlier copy of the same word.
	LandingEnd string

	// LandingPrefix is a few words the match follows inside its first block,
	// whitespace collapsed, so a browser that walks the whole page in order
	// does not answer a common word with an earlier copy, such as one in the
	// navigation. Empty where the match opens its block.
	LandingPrefix string

	// LandingSuffix is the run after the last term a directive names, up to
	// the nearest certain boundary, which releases that term from ending
	// where a word does: in a script that parts no words with spaces that
	// boundary is a dictionary's. Empty where the term already stands against
	// one, or where nothing vouches that the page keeps the run beside it.
	LandingSuffix string

	// BlockCrossing reports that the match continues past that first block,
	// so a directive built from the whole phrase would find nothing.
	BlockCrossing bool

	// FromFence reports that the landing is in a fenced block: the words
	// live only there. The row names that the way it names an alias.
	FromFence bool

	// Language is the note's own declared BCP 47 tag when the contract gave
	// the field authority and the frontmatter carried a valid value. Empty
	// otherwise, so a listing inherits the page language rather than guessing.
	Language string
}

// Answer is what one pass over the index found for a query: the results the
// caller asked to have built, and how many hits there truly were.
type Answer struct {
	Results []Result
	Total   int
}

// Search runs a parsed query against the index and returns results in the final
// deterministic order, four groups concatenated: title hits, body hits, tag hits,
// then the path-only hits. Each group keeps the vault's reading order, except
// that a fold-equal exact title leads the title group, a knowledge-layer hit
// leads an outside-knowledge hit inside the same group, and every text hit
// outranks every path-only hit.
//
// An empty query returns nothing and a pure-filter query lands every match in
// the title bucket. A metadata filter excludes non-instance artifacts, and
// returns ErrMetadataUnavailable when the artifact policy was declared and could
// not be honoured. At most limit results are materialized, a negative limit all
// of them; Total counts every hit and the tail beyond limit is never built.
func (idx *Index) Search(q *Query, limit int) (Answer, error) {
	if len(q.tokens) == 0 && len(q.filters) == 0 {
		return Answer{}, nil
	}
	metadataAvailable := idx.policy.Trustworthy()
	requiresMetadata := q.requiresMetadata()
	if requiresMetadata && !metadataAvailable {
		return Answer{}, idx.metadataUnavailableError()
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
	answers.raiseKnowledge()
	answers.raiseExactTitles(q.tokens)
	hits := answers.ordered()
	answer := Answer{Total: len(hits)}
	if limit >= 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	answer.Results = make([]Result, len(hits))
	for i, h := range hits {
		answer.Results[i] = h.entry.result(q.tokens, h.bodyEvidence, metadataAvailable, h.alias, h.tag)
	}
	return answer, nil
}

// hit is one matched entry before its Result is built. Buckets hold hits
// rather than Results so a bounded search can count every match while
// building — lead and landing included — only the results it will return.
type hit struct {
	entry        *entry
	bodyEvidence bool
	// alias is the name that answered the query when it was not the title, in
	// the note's own spelling, since the row itself shows a title.
	alias string
	tag   string
}

// bucket names one answer group. This declaration order is the result order and
// the only statement of it, so a group moves by moving its constant: title and body
// before a tag, and every text hit before an address.
type bucket uint8

const (
	bucketTitle bucket = iota
	bucketBody
	bucketTag
	bucketPath
	bucketCount
)

// resultBuckets keeps the answer groups apart while one pass fills them, so the
// final order is a concatenation rather than a sort of the whole answer.
type resultBuckets struct {
	groups [bucketCount][]hit
}

// place files one filter-matching entry into its answer group by what the
// tokens matched: the title, the body, a declared tag, or only the path. An
// entry that passed every filter and whose words answer none of the terms is not
// a hit, and is filed nowhere.
func (b *resultBuckets) place(e *entry, tokens []string) {
	if allContain(e.titleFold, tokens) {
		bodyEvidence := len(tokens) != 0 && allContain(e.plainFold, tokens)
		b.groups[bucketTitle] = append(b.groups[bucketTitle], hit{entry: e, bodyEvidence: bodyEvidence})
		return
	}
	if alias := aliasAnswering(e, tokens); alias != "" {
		// An alias stands with the title: a link written to one resolves and a
		// link written to a title does not.
		bodyEvidence := len(tokens) != 0 && allContain(e.plainFold, tokens)
		b.groups[bucketTitle] = append(b.groups[bucketTitle], hit{entry: e, bodyEvidence: bodyEvidence, alias: alias})
		return
	}
	if allContain(e.plainFold, tokens) {
		b.groups[bucketBody] = append(b.groups[bucketBody], hit{entry: e, bodyEvidence: true})
		return
	}
	if tag := tagAnswering(e, tokens); tag != "" {
		b.groups[bucketTag] = append(b.groups[bucketTag], hit{entry: e, tag: tag})
		return
	}
	if allContain(e.pathFold, tokens) {
		b.groups[bucketPath] = append(b.groups[bucketPath], hit{entry: e})
	}
}

// raiseExactTitles is the one tie-break inside the title-note group: a title
// that is the query, under the same fold matching uses, leads every title that
// merely contains it. Hits that share that answer keep the vault's reading
// order. The other groups are not touched, and an empty token list is a
// pure-filter query whose every match already sits here.
func (b *resultBuckets) raiseExactTitles(tokens []string) {
	if len(tokens) == 0 {
		return
	}
	needle := strings.Join(tokens, " ")
	slices.SortStableFunc(b.groups[bucketTitle], func(left, right hit) int {
		return cmp.Compare(exactTitleRank(right.entry.titleFold, needle), exactTitleRank(left.entry.titleFold, needle))
	})
}

func exactTitleRank(titleFold, needle string) int {
	if titleFold == needle {
		return 1
	}
	return 0
}

// raiseKnowledge is the flatten-time tie-break inside every group: a hit the
// contract placed in the knowledge layer leads a hit it placed outside, and
// hits that share that answer keep the order they already had. An undeclared
// layer marks nothing, so the groups stay in the vault's reading order.
// raiseExactTitles then runs, so the layer is a tie-break among hits of the
// same strength and an exact title still leads a containing one.
func (b *resultBuckets) raiseKnowledge() {
	for i := range b.groups {
		slices.SortStableFunc(b.groups[i], func(left, right hit) int {
			return cmp.Compare(knowledgeRank(left.entry), knowledgeRank(right.entry))
		})
	}
}

func knowledgeRank(e *entry) int {
	if e.outsideKnowledge {
		return 1
	}
	return 0
}

// ordered flattens the groups into the answer, in the order the constants are
// declared in.
func (b *resultBuckets) ordered() []hit {
	return slices.Concat(b.groups[:]...)
}

// tagTokens accepts one leading hash as tag shorthand without changing the
// literal query used by titles, aliases, bodies and paths.
func tagTokens(tokens []string) []string {
	out := make([]string, len(tokens))
	for i, token := range tokens {
		out[i] = strings.TrimPrefix(token, "#")
		if out[i] == "" {
			return nil
		}
	}
	return out
}

func tagAnswering(e *entry, tokens []string) string {
	if len(e.tagFolds) == 0 {
		return ""
	}
	terms := tagTokens(tokens)
	if len(terms) == 0 {
		return ""
	}
	for i, folded := range e.tagFolds {
		if allContain(folded, terms) {
			return e.tags[i]
		}
	}
	return ""
}

// MarkTagHits uses the same shorthand as tag matching, so a hash-prefixed
// query still marks the author's tag rather than hiding the reason for a hit.
func MarkTagHits(tag string, tokens []string) []HitRun {
	if tag == "" {
		return nil
	}
	return MarkHits(tag, tagTokens(tokens))
}

// aliasAnswering returns the note's own spelling of the first alias that holds
// every token, or empty when none does. The first is taken because an author
// listing several puts the one they think of first at the front.
func aliasAnswering(e *entry, tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	for i, folded := range e.aliasFolds {
		if allContain(folded, tokens) {
			return e.aliases[i]
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

// matchesFilter reports whether e satisfies one filter. type/domain/slug
// compare the folded copies; draft asks the flag, which Parse reads only as
// true or false; folder is
// a rel_path prefix taken one directory name at a time, so "folder:Writing"
// matches "Writing" and "Writing/x.md" and "writing/x.md", but never
// "Writing-old/x.md".
func (e *entry) matchesFilter(f Filter) bool {
	switch f.Key {
	case filterType:
		return e.noteTypeFold == f.Value
	case filterDraft:
		return e.draft == (f.Value == "true")
	case filterDomain:
		return e.domainFold == f.Value
	case filterSlug:
		return e.slugFold == f.Value
	case filterFolder:
		return withinFolder(e.relPath, f.Value)
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

// withinFolder reports whether relPath is folder itself or sits under it. Both
// sides are cut at their own "/" first and the directory names are folded one
// by one, so case and width still meet inside a name while a name that carries
// a fullwidth solidus stays one directory rather than reading as a parent and a
// child. Folding the two paths whole and looking for a "/" cannot tell those
// apart, since the fold narrows that character to a separator the vault never
// wrote.
func withinFolder(relPath, folder string) bool {
	names := strings.Split(relPath, "/")
	want := strings.Split(folder, "/")
	if len(want) > len(names) {
		return false
	}
	for i, name := range want {
		if Fold(name) != Fold(names[i]) {
			return false
		}
	}
	return true
}

// result builds a Result for e. Only a hit with body evidence has a lead and
// a landing.
func (e *entry) result(tokens []string, bodyEvidence, metadataAvailable bool, alias, tag string) Result {
	draft := e.draft && metadataAvailable && e.metadataCapable
	var terms landingTerms
	var fromFence bool
	var lead Lead
	if bodyEvidence {
		terms, fromFence = e.landing(tokens)
		lead = e.lead(tokens)
	}
	noteType := ""
	if metadataAvailable && e.metadataCapable {
		noteType = e.noteType
	}
	return Result{
		RelPath:       e.relPath,
		Title:         e.title,
		NoteType:      noteType,
		Lead:          lead,
		Draft:         draft,
		Alias:         alias,
		Tag:           tag,
		Landing:       terms.first,
		LandingBare:   terms.bare,
		LandingEnd:    terms.last,
		LandingPrefix: terms.prefix,
		LandingSuffix: terms.suffix,
		BlockCrossing: terms.crossing,
		FromFence:     fromFence,
		Language:      e.language,
	}
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

// earliestOffset returns the folded byte range of the token a result lands on:
// the earliest prose hit when the note has one, otherwise the earliest fence
// hit. start is < 0 when no token occurs. inFence is
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

// wordEdgeBudget bounds how far a boundary may move to keep a word whole. A
// date or an identifier fits inside it; a run longer than this is not a word
// anyone is reading as one, and the window matters more than it does.
const wordEdgeBudget = 24

// wholeWordStart moves a passage's opening boundary back to the start of a word
// the cut fell into, so a date arrives as 2026-07-30 rather than 026-07-30. Only
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
	return isASCIIAlnum(b) || b == '-' || b == '_'
}

// HitRun is one stretch of a piece of text and whether the query matched it.
// The runs of one text cover it exactly once, in order, so a caller can render
// them straight through without consulting an offset.
type HitRun struct {
	Text string
	Hit  bool
}

// MarkHits cuts a piece of text into the stretches that matched and those that did
// not, so a page can show why a result is here. The same cut serves the lead,
// the title, the path, and any other name a note can match by. Matching is
// done on the folded form while the runs carry slices of the original text,
// and overlapping matches merge into one mark.
func MarkHits(text string, tokens []string) []HitRun {
	if text == "" || len(tokens) == 0 {
		return nil
	}
	fold, src := foldWithSourceOffsets(text)
	covered := make([]bool, len(text))
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
	for i := 1; i <= len(text); i++ {
		if i < len(text) && covered[i] == covered[start] {
			continue
		}
		runs = append(runs, HitRun{Text: text[start:i], Hit: covered[start]})
		start = i
	}
	return runs
}
