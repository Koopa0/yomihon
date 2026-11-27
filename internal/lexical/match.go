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

	// LandingBare is the stretch to name where the directive carries no words
	// ahead of it: Landing grown out to the edges of the words it lies
	// inside. A browser looks for a run introduced by nothing only where a
	// word begins and where one ends, so a match opening or closing inside a
	// word is asked for at a place the page has nowhere, and the note stays
	// at the top. It equals Landing wherever the match already lies on those
	// edges, which is most matches. Each edge is decided on its own, and one
	// resting inside a script that parts no words with spaces stays where the
	// match left it: the edges there are a segmentation nothing here can
	// find, and the sentence around the match is not them.
	LandingBare string

	// LandingEnd is the last-block stretch of a crossing match, grown to the
	// edges of the words it lies inside the way LandingBare is. A directive
	// that names both ends can span blocks; a bare first-block term would
	// land on an earlier copy of the same word.
	//
	// A browser holds this stretch to both edges whichever of its two places
	// it takes — closing a range, or travelling alone because the first block
	// had nothing locatable — so a match stopping inside a word is asked for
	// at a place the page has nowhere, and the whole directive is abandoned.
	// The stretch opens where its block does, which is already such an edge,
	// so only the far end can move. An end resting inside a script that parts
	// no words with spaces stays where the match left it, as in LandingBare.
	LandingEnd string

	// LandingPrefix is the run of words the match follows inside the block
	// its first stretch opens, with whitespace collapsed and cut to a
	// few words. A browser looks for the term it is given by walking the
	// whole page in order, so a start stretch of one common word is answered
	// by the first copy anywhere on it — the reader's own navigation is drawn
	// before the article and spells some of those words too. Naming what the
	// match follows tells the two apart. Empty where the match opens its
	// block, which is where there is nothing to say.
	LandingPrefix string

	// LandingSuffix is the run of characters after the last term a directive
	// names, up to the nearest certain boundary: white space, a line break,
	// punctuation, or the end of the block. A browser holds a term with nothing
	// behind it to end where a word does, and in a script that parts no words with spaces
	// that is a dictionary boundary this index has no model of. A term the
	// directive follows with a run that itself ends at a certain boundary is
	// released from the requirement, so the match can end anywhere. Empty where
	// the term already stands against such a boundary, and where neither the
	// whole block nor a local context range vouches for the term, the run and
	// its ending boundary staying adjacent on the page.
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

const (
	// landingPrefixWords and landingPrefixRunes bound the run of words named
	// ahead of a match, in words and in characters the reader sees. The
	// budget is small deliberately: the run is written for a browser to find
	// again in rendered text, and every further word is another chance for
	// the two spellings of the same sentence to part.
	landingPrefixWords = 3
	landingPrefixRunes = 30
)

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
	requiresMetadata := q.RequiresMetadata()
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
	switch {
	case allContain(e.TitleFold, tokens):
		bodyEvidence := len(tokens) != 0 && allContain(e.PlainFold, tokens)
		b.groups[bucketTitle] = append(b.groups[bucketTitle], hit{entry: e, bodyEvidence: bodyEvidence})
	case aliasAnswering(e, tokens) != "":
		// An alias stands with the title: a link written to one resolves and a
		// link written to a title does not.
		bodyEvidence := len(tokens) != 0 && allContain(e.PlainFold, tokens)
		b.groups[bucketTitle] = append(b.groups[bucketTitle], hit{entry: e, bodyEvidence: bodyEvidence, alias: aliasAnswering(e, tokens)})
	case allContain(e.PlainFold, tokens):
		b.groups[bucketBody] = append(b.groups[bucketBody], hit{entry: e, bodyEvidence: true})
	case tagAnswering(e, tokens) != "":
		b.groups[bucketTag] = append(b.groups[bucketTag], hit{entry: e, tag: tagAnswering(e, tokens)})
	case allContain(e.PathFold, tokens):
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
		return cmp.Compare(exactTitleRank(right.entry.TitleFold, needle), exactTitleRank(left.entry.TitleFold, needle))
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
	if len(e.TagFolds) == 0 {
		return ""
	}
	terms := tagTokens(tokens)
	if len(terms) == 0 {
		return ""
	}
	for i, folded := range e.TagFolds {
		if allContain(folded, terms) {
			return e.Tags[i]
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

// matchesFilter reports whether e satisfies one filter. type/domain/slug
// compare the folded copies; draft asks the flag, which Parse reads only as
// true or false; folder is
// a rel_path prefix taken one directory name at a time, so "folder:Writing"
// matches "Writing" and "Writing/x.md" and "writing/x.md", but never
// "Writing-old/x.md".
func (e *entry) matchesFilter(f Filter) bool {
	switch f.Key {
	case "type":
		return e.NoteTypeFold == f.Value
	case DraftFilterKey:
		return e.Draft == (f.Value == "true")
	case "domain":
		return e.DomainFold == f.Value
	case "slug":
		return e.SlugFold == f.Value
	case "folder":
		return withinFolder(e.RelPath, f.Value)
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
		if fold(name) != fold(names[i]) {
			return false
		}
	}
	return true
}

// result builds a Result for e. Only a hit with body evidence has a lead and
// a landing.
func (e *entry) result(tokens []string, bodyEvidence, metadataAvailable bool, alias, tag string) Result {
	draft := e.Draft && metadataAvailable && e.metadataCapable
	var terms landingTerms
	var fromFence bool
	var lead Lead
	if bodyEvidence {
		terms, fromFence = e.landing(tokens)
		lead = e.lead(tokens)
	}
	noteType := ""
	if metadataAvailable && e.metadataCapable {
		noteType = e.NoteType
	}
	return Result{
		RelPath:       e.RelPath,
		Title:         e.Title,
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

// landing places a result's link on its earliest body match: the earliest the
// page shows, where the note carries text the page hides, and otherwise the
// earliest prose match, or a fence's where the words live only there. A match
// within its first block is taken together with every match of any term that
// overlaps or touches it, which is the stretch the page marks as one.
func (e *entry) landing(tokens []string) (terms landingTerms, fromFence bool) {
	if len(e.displaySpans) > 0 {
		centre := e.shownExcerptCentre(tokens)
		if !centre.found {
			return landingTerms{}, false
		}
		return e.landingAtSource(centre.start, centre.end), centre.fromFence
	}
	foldStart, foldEnd, fromFence := earliestOffset(e.PlainFold, tokens, e.fenceFoldRanges)
	if foldStart < 0 {
		return landingTerms{}, false
	}
	terms = e.landingAt(foldStart, foldEnd)
	if !terms.crossing {
		foldEnd = mergedMatchEnd(e.PlainFold, tokens, foldStart, foldEnd)
		start, end := sourceOffsetOfFold(e.PlainText, foldStart), sourceEndOfFold(e.PlainText, foldEnd)
		// A phrase quoted with white space at its ends matches that white
		// space too, and the page marks the words, not the space around them.
		matched := e.PlainText[start:end]
		start += len(matched) - len(strings.TrimLeftFunc(matched, unicode.IsSpace))
		end -= len(matched) - len(strings.TrimRightFunc(matched, unicode.IsSpace))
		terms = e.landingAtSource(start, end)
	}
	return terms, fromFence
}

// mergedMatchEnd extends the folded match [start, end) over every match of any
// token that begins inside it or where it ends, until none does.
func mergedMatchEnd(fold string, tokens []string, start, end int) int {
	for grown := true; grown; {
		grown = false
		for _, t := range tokens {
			for at := start; t != "" && at <= end; {
				i, stop := phraseIndex(fold, t, at)
				if i < 0 || i > end {
					break
				}
				if stop > end {
					end, grown = stop, true
				}
				at = i + 1
			}
		}
	}
	return end
}

// landingTerms is everything one body match gives a browser text directive:
// the stretch to arrive at, that same stretch grown to whole words for a
// directive that names nothing ahead of it, the run of words the match
// follows, the stretch its far end sits in grown the same way, the run after
// the last of those stretches, and whether it left its block to reach that
// end. They travel together because a directive is assembled from all of them
// at once, and five loose strings are five a caller can pair up the wrong way
// round.
type landingTerms struct {
	prefix   string
	suffix   string
	first    string
	bare     string
	last     string
	crossing bool
}

// landingAt is the terms one folded body match offers. The exclusive end is
// the last matched rune plus its length: the next kept rune can sit in a later
// block after a fold-dropped break, which is not itself a crossing. Without
// recorded block ends there is nothing to tell a wrap from a paragraph, so it
// offers nothing and does not claim a crossing.
//
// The preceding run is cut at the start of the match's own block, because a
// directive may only name one where it sits beside the term it introduces. A
// match that opens its block is preceded by nothing and says so.
//
// So is a match in a block the page does not reproduce as written. Naming what
// a match follows asks a browser to find the two side by side in what it is
// showing, and on those blocks they are not: a ruby reading is written here
// after the sentence it is spoken inside, and a footnote's mark is on the page
// and not in this text at all. Such a run would match nothing and the note
// would open at the top, which is worse than the bare term it replaced.
//
// Those are the blocks whose stretch has to stand on its own, which is why the
// grown form is measured for every match rather than only for some: a block
// that offers no run of words ahead of the match has nothing else to offer.
// The growth stays inside the block, so a word cannot be assembled out of two
// paragraphs the page draws apart.
//
// A match that crossed has a second stretch in its last block, and that one is
// grown whatever the first block offered: a run of words is read as one only
// beside a term from its own block, and the run belongs to the block the match
// opened in, so this stretch has only ever stood on its own. Its own edges are
// still asked the same question, and one in a script that parts no words with
// spaces is left where the match put it.
func (e *entry) landingAt(foldStart, foldEnd int) landingTerms {
	if foldStart < 0 || foldEnd <= foldStart || len(e.blocks) == 0 {
		return landingTerms{}
	}
	start := sourceOffsetOfFold(e.PlainText, foldStart)
	end := sourceEndOfFold(e.PlainText, foldEnd)
	if start >= end || end > len(e.PlainText) {
		return landingTerms{}
	}
	return e.landingAtSource(start, end)
}

// landingAtSource places a selected source span on the directive's word edges.
//
// The run after the match is measured from the end of the last stretch a
// directive can name, wherever growth left that end, and asked of the block
// that stretch sits in: for a crossing match that is the last block, whose own
// reproduction decides whether a run beside the term can be read as one.
func (e *entry) landingAtSource(start, end int) landingTerms {
	var terms landingTerms
	if e.landingBlocked(start, end) {
		return terms
	}
	blockStart, verbatim := e.blockAt(start)
	blockStart, firstLimit := e.insertionBounds(start, blockStart, e.blockEndAfter(start))
	if verbatim {
		terms.prefix = landingPrefix(e.PlainText[blockStart:start])
	}
	firstEnd := e.blockEndAfter(start)
	terms.crossing = end > firstEnd
	firstStop := end
	if terms.crossing {
		firstStop = firstEnd
	}
	bareStart, bareEnd := wordEdges(e.PlainText, start, firstStop, blockStart, firstLimit)
	terms.first = collapseFields(e.PlainText[start:bareEnd])
	terms.bare = collapseFields(e.PlainText[bareStart:bareEnd])
	if !terms.crossing {
		if headingStart, before, after := e.sectionContext(firstEnd); firstLimit == firstEnd && terms.first != "" && after != "" {
			// The stretch runs on to the heading's end so the run after it
			// is the section's own opening, which the contents list's copy
			// of the heading is not followed by.
			terms.first = collapseFields(e.PlainText[start:firstEnd])
			terms.bare = collapseFields(e.PlainText[bareStart:firstEnd])
			terms.suffix = after
			return terms
		} else if terms.first != "" && before != "" && blockStart <= headingStart {
			// A run named ahead of a term is read only where it ends right
			// at the term, so the stretch is the whole heading, from the
			// edge the previous block's words stand against to its end.
			terms.prefix = before
			terms.first = collapseFields(e.PlainText[headingStart:firstEnd])
			terms.bare = terms.first
			return terms
		}
		if terms.first != "" && (verbatim || e.contextSuffixAt(start, bareEnd, firstLimit)) {
			terms.suffix = landingSuffix(e.PlainText, bareEnd, firstLimit)
		}
		return terms
	}
	return e.finishCrossingLanding(end, firstEnd, &terms)
}

// finishCrossingLanding measures the last stretch and its following run inside
// the block that holds the match's far end.
func (e *entry) finishCrossingLanding(end, firstEnd int, terms *landingTerms) landingTerms {
	lastStart, lastVerbatim := e.blockAt(end - 1)
	from := max(lastStart, firstEnd)
	lastEnd := e.blockEndAfter(end - 1)
	from, lastEnd = e.insertionBounds(end-1, from, lastEnd)
	// The far end is grown for the reason the first stretch's standalone form
	// is, and inside its own block for the reason that one is: a word cannot
	// be assembled out of two blocks the page draws apart. This stretch opens
	// where that block opens, which is a word's edge already, so that same
	// offset serves as the floor and the only growth is outward.
	_, lastStop := wordEdges(e.PlainText, from, end, from, lastEnd)
	terms.last = collapseFields(e.PlainText[from:lastStop])
	if lastVerbatim && terms.last != "" {
		terms.suffix = landingSuffix(e.PlainText, lastStop, lastEnd)
	}
	return *terms
}

// sectionContext is the run a directive names beside a match inside a
// heading the page shows as written, so that the body's copy of the heading
// answers it and the contents list's copy does not. The list repeats the
// heading's own words, and on a narrow page it stands above the prose, so any
// run taken from inside the heading is answered by the list's copy first.
//
// The first words of the next block are preferred: the list's copy is
// followed by the next entry's name, or its last entry is followed by reading
// aids and then the body's first block. The last words of the previous block
// provide context when a known preceding contents entry cannot answer them.
// Each run comes from adjacent page text, and only where the list cannot
// answer it too. Both empty means the hit keeps the terms any other block
// gives it. headingStart is where the heading's text begins.
func (e *entry) sectionContext(headingEnd int) (headingStart int, before, after string) {
	for i, b := range e.blocks {
		if b.End != headingEnd {
			headingStart = b.End
			continue
		}
		if !b.Heading || e.hasInsertion(headingStart, headingEnd) {
			return headingStart, "", ""
		}
		if i+1 < len(e.blocks) && e.shownAsWritten(i+1) {
			_, openingEnd := e.insertionBounds(headingEnd, headingEnd, e.blocks[i+1].End)
			opening := landingOpening(e.PlainText[headingEnd:openingEnd])
			if opening != "" && !e.opensAnotherBlock(opening, i+1) {
				return headingStart, "", opening
			}
		}
		if i > 0 && e.shownAsWritten(i-1) {
			closingStart, _ := e.insertionBounds(headingStart, e.blockStart(i-1), headingStart)
			closing := landingPrefix(e.PlainText[closingStart:headingStart])
			if closing != "" && e.closingFollowsAnEntry(closing, i-1) {
				return headingStart, closing, ""
			}
		}
		return headingStart, "", ""
	}
	return 0, "", ""
}

// shownAsWritten reports that block i's corpus words reach the page in this
// order. Insertions are clipped separately before the words name context.
func (e *entry) shownAsWritten(i int) bool {
	return e.blocks[i].Verbatim
}

// blockStart is where block i's text begins.
func (e *entry) blockStart(i int) int {
	if i == 0 {
		return 0
	}
	return e.blocks[i-1].End
}

// opensAnotherBlock reports that the text from the start of some block other
// than block own reads, the way a browser compares (white space as one space,
// letters in either case), as opening. The next entry's name starts a block,
// and so does the body's first block, which the last entry's copy is
// followed by once the page's other reading aids have been passed; an
// opening that also starts a block could be answered beside the list's copy.
// What those aids say is the page's, not the note's, so it is not asked.
func (e *entry) opensAnotherBlock(opening string, own int) bool {
	words := strings.Fields(opening)
	for i := range e.blocks {
		if i != own && readsAs(e.PlainText[e.blockStart(i):], words) {
			return true
		}
	}
	return false
}

// readsAs reports that text opens with words, white space between them read
// as one space and case ignored. The last word need only open a word of
// text, which only ever counts more texts as a match.
func readsAs(text string, words []string) bool {
	for i, w := range words {
		text = strings.TrimLeftFunc(text, unicode.IsSpace)
		if len(text) < len(w) || !strings.EqualFold(text[:len(w)], w) {
			return false
		}
		text = text[len(w):]
		if i < len(words)-1 {
			if r, _ := utf8.DecodeRuneInString(text); text == "" || !unicode.IsSpace(r) {
				return false
			}
		}
	}
	return true
}

// closingFollowsAnEntry reports that a contents entry the closing words of
// block prev cannot end is known to stand before the heading's own entry.
// The walk back from prev passes only over blocks the page shows as written,
// none of which adds an entry, to the heading that names the section prev
// sits in; that heading's words are the ones the list writes ahead of the
// heading's copy. Reaching the note's start, or a block that could hide an
// entry — a heading the page changes, an embed — leaves the entry before it
// unknown, as is the first entry's, which follows the list's own label.
func (e *entry) closingFollowsAnEntry(closing string, prev int) bool {
	for i := prev - 1; i >= 0; i-- {
		b := e.blocks[i]
		if b.Heading {
			name := strings.ToLower(collapseFields(e.PlainText[e.blockStart(i):b.End]))
			lower := strings.ToLower(closing)
			return !strings.HasSuffix(lower, name) && !strings.HasSuffix(name, lower)
		}
		if !b.Verbatim {
			return false
		}
	}
	return false
}

// landingOpening is the first few words of a block, stopping where a run
// ends at a certain boundary: before a hard line break, and within the word
// and character budget a preceding run keeps. A first word over that budget
// is cut at its first certain boundary instead, the way a following run is.
func landingOpening(block string) string {
	block = strings.TrimLeftFunc(block, unicode.IsSpace)
	if i := strings.IndexByte(block, '\n'); i >= 0 {
		block = block[:i]
	}
	words := strings.Fields(block)
	if len(words) > landingPrefixWords {
		words = words[:landingPrefixWords]
	}
	for len(words) > 0 && utf8.RuneCountInString(strings.Join(words, " ")) > landingPrefixRunes {
		words = words[:len(words)-1]
	}
	if len(words) > 0 {
		return strings.Join(words, " ")
	}
	return landingSuffix(block, 0, len(block))
}

// contextSuffixAt vouches for the match and its following run in one local
// range. A removed delimiter is not a word boundary, so the boundary after
// the run must also be reproduced, unless it is the actual block end.
func (e *entry) contextSuffixAt(start, from, blockEnd int) bool {
	suffix := landingSuffix(e.PlainText, from, blockEnd)
	if suffix == "" {
		return false
	}
	stop := from + len(suffix)
	if stop < blockEnd {
		_, size := utf8.DecodeRuneInString(e.PlainText[stop:blockEnd])
		stop += size
	}
	for _, b := range e.blocks {
		if b.End < blockEnd {
			continue
		}
		for _, span := range b.ContextRanges {
			if span[0] <= start && stop <= span[1] {
				return true
			}
		}
		return false
	}
	return false
}

// wordEdges grows [start, end) out to the edges of the words it lies inside,
// never leaving [low, high). Only an end whose own character is one a word
// joins can be inside a word at all; every other end already stands on a
// boundary and is left alone. Each end is asked separately. That one condition
// is what keeps a stretch of white space from drawing in the word beside it,
// and what leaves an end resting inside a script that parts no words with
// spaces exactly where it was — the edges there are a segmentation this vault
// has no way to find, and the sentence around the match is not them.
func wordEdges(s string, start, end, low, high int) (grownStart, grownEnd int) {
	grownStart, grownEnd = start, end
	if opening, _ := utf8.DecodeRuneInString(s[start:end]); joinsAWord(opening) {
		for grownStart > low {
			r, size := utf8.DecodeLastRuneInString(s[low:grownStart])
			if !joinsAWord(r) {
				break
			}
			grownStart -= size
		}
	}
	if closing, _ := utf8.DecodeLastRuneInString(s[:end]); joinsAWord(closing) {
		for grownEnd < high {
			r, size := utf8.DecodeRuneInString(s[grownEnd:high])
			if !joinsAWord(r) {
				break
			}
			grownEnd += size
		}
	}
	return grownStart, grownEnd
}

// joinsAWord reports whether r is joined to the character beside it into one
// word, the way Unicode text segmentation (UAX #29) joins them: a letter or a
// digit, in a script that parts its words with spaces. Every other character
// carries a boundary on both sides, so a stretch that begins or ends against
// one begins or ends where a word does.
//
// It is neither of the two runs already measured in this file. wordRun is
// bounded by white space alone, so in "今日は晴れ、cobaltine" it answers the
// whole sentence, and a browser handed that sentence as one term finds nothing
// — twice over, because the page draws a ruby reading in among the characters
// it is spoken over, so a stretch reaching across one is not text the page
// carries in one piece at all. isWordByte keeps a date or an identifier whole
// for a reader and so holds on to the hyphen, which a browser breaks at.
func joinsAWord(r rune) bool {
	return (unicode.IsLetter(r) || unicode.IsDigit(r)) && !writesWithoutSpaces(r)
}

// landingPrefix cuts a block's run of words before a match down to what one
// term of a directive can carry. Only whole words come off the front, and a
// last word still over the budget comes off with them.
//
// A browser finds a leading run only where it begins at a word boundary, and
// the boundaries this index can be certain of are the ones every segmenter
// agrees on: white space, punctuation, and the start of the block. A cut made
// anywhere else in a script that parts no words with spaces divides them by
// dictionary, which nothing here has a model of, so it falls inside a word
// almost every time — and a run that begins inside a word is not merely
// weaker: it is found nowhere, and the browser abandons the whole directive.
//
// Where the last word is over the budget and is written in such a script, the
// run therefore begins after the earliest certain boundary that leaves it
// inside the budget, and where there is none, after the nearest one before
// it. A last word in any other script is dropped instead: the term that
// follows it is grown to the edges of its own word, so it stands alone
// without one.
func landingPrefix(run string) string {
	words := strings.Fields(run)
	if len(words) > landingPrefixWords {
		words = words[len(words)-landingPrefixWords:]
	}
	for len(words) > 0 && utf8.RuneCountInString(strings.Join(words, " ")) > landingPrefixRunes {
		words = words[1:]
	}
	if len(words) > 0 {
		return strings.Join(words, " ")
	}
	return unspacedPrefix(run)
}

// unspacedPrefix is the run a directive names ahead of a match whose last word
// is over the budget, or empty where that word is not written without spaces.
// It never reaches back over a hard line break: the page draws a break as a
// break, not as the white space it becomes once the run is collapsed, so a
// run spanning one is text the page does not carry in one piece.
func unspacedPrefix(run string) string {
	if i := strings.LastIndex(run, "\n"); i >= 0 {
		run = run[i+1:]
	}
	runes := []rune(collapseFields(run))
	if len(runes) == 0 {
		return ""
	}
	lastWord := runes
	for i, r := range slices.Backward(runes) {
		if r == ' ' {
			lastWord = runes[i+1:]
			break
		}
	}
	if !slices.ContainsFunc(lastWord, writesWithoutSpaces) {
		return ""
	}
	last := len(runes) - 1
	if isCertainBoundary(runes, last) {
		return ""
	}
	for i := max(1, len(runes)-landingPrefixRunes); i <= last; i++ {
		if isCertainBoundary(runes, i-1) {
			return string(runes[i:])
		}
	}
	for i := last - 1; i >= 0; i-- {
		if isCertainBoundary(runes, i) {
			return string(runes[i+1:])
		}
	}
	return string(runes)
}

// landingSuffix cuts the block after a term, text[from:to], down to the
// nearest certain boundary, which is the run itself ending where the browser
// agrees a word ends. A hard line break is one, so the run never reaches over
// it, and with no other the rest of the block is the run, whose end is one.
// Empty where the term already stands against a boundary. The character
// before from is read too, because whether a mark joins a word depends on
// what stands on both sides of it.
func landingSuffix(text string, from, to int) string {
	runes := []rune(text[from:to])
	offset := 0
	if from > 0 {
		before, _ := utf8.DecodeLastRuneInString(text[:from])
		runes = append([]rune{before}, runes...)
		offset = 1
	}
	for i := offset; i < len(runes); i++ {
		if isCertainBoundary(runes, i) {
			return collapseFields(string(runes[offset:i]))
		}
	}
	return collapseFields(string(runes[offset:]))
}

// isCertainBoundary reports whether the character at i, being white space or
// punctuation, is one every text segmenter breaks a word at. A few marks only
// join: an apostrophe or a full stop inside a word, a comma inside a number.
// They are skipped where a letter or digit stands on both sides, and are
// boundaries otherwise, since beside characters of a script without spaces
// they join nothing. Two more join whatever stands beside them, katakana
// included, and are never a boundary.
func isCertainBoundary(runes []rune, i int) bool {
	r := runes[i]
	switch {
	case unicode.IsSpace(r):
		return true
	case !unicode.IsPunct(r), strings.ContainsRune(alwaysJoiningMarks, r):
		return false
	case !strings.ContainsRune(joiningMarks, r):
		return true
	}
	return i == 0 || i == len(runes)-1 || !joinsAWord(runes[i-1]) || !joinsAWord(runes[i+1])
}

// joiningMarks are the punctuation characters that, between two letters or two
// digits, are part of the word around them rather than a break in it.
const joiningMarks = "'\u2018\u2019.\u2024:,;\u00b7\u2027\uff07\uff0e\uff1a\uff0c\uff1b"

// alwaysJoiningMarks are the punctuation characters a word continues through
// on both sides: the katakana middle dot, which katakana words are written
// with, and the low line, which identifiers are.
const alwaysJoiningMarks = "\u30fb_\uff3f"

func collapseFields(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func (e *entry) blockEndAfter(off int) int {
	for _, b := range e.blocks {
		if b.End > off {
			return b.End
		}
	}
	return len(e.PlainText)
}

// blockAt is where the block holding off begins, and whether the reading page
// reproduces that block as written. An offset past every block this entry
// knows about is reproduced by nothing anyone here has looked at.
func (e *entry) blockAt(off int) (start int, verbatim bool) {
	prev := 0
	for _, b := range e.blocks {
		if b.End > off {
			return prev, b.Verbatim
		}
		prev = b.End
	}
	return prev, false
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
	return b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '-' || b == '_'
}

// foldWithSourceOffsets folds s and maps every byte position of the folded
// copy, one past its end included, back to the byte offset in s of the character
// it came from. It applies the walk half of the index's fold alone, which
// reproduces that fold provided s is already NFC, as every indexed text is.
func foldWithSourceOffsets(s string) (fold string, src []int) {
	var folded strings.Builder
	folded.Grow(len(s))
	src = make([]int, 0, len(s)+1)
	for at, r := range foldRunes(s) {
		n := folded.Len()
		folded.WriteRune(r)
		for ; n < folded.Len(); n++ {
			src = append(src, at)
		}
	}
	return folded.String(), append(src, len(s))
}

// sourceOffsetOfFold maps one byte offset in the folded copy of s back to the
// byte offset in s of the character that produced it, and len(s) past the end.
// It answers exactly what foldWithSourceOffsets tabulates, walked to one
// position rather than materialized because its caller measures a whole note.
// A test holds the two to one answer.
func sourceOffsetOfFold(s string, foldOff int) int {
	cursor := foldCursor{s: s}
	return cursor.source(foldOff)
}

// sourceEndOfFold maps the exclusive end of a folded match back to the exclusive
// source offset of its last rune. sourceOffsetOfFold at that same fold end would
// name the next kept rune, which can sit past a dropped break and is not the
// match.
func sourceEndOfFold(s string, foldEnd int) int {
	if foldEnd <= 0 {
		return 0
	}
	at := sourceOffsetOfFold(s, foldEnd-1)
	_, size := utf8.DecodeRuneInString(s[at:])
	return at + size
}

// foldCursor maps offsets in the folded copy of s back to s for offsets that
// never decrease: each walk resumes on the character the last one stopped at,
// so mapping every match of a term in a note walks the note once, and only as
// far as the last match asked about.
type foldCursor struct {
	s string
	// src is a character the fold keeps and fold the offset its folded form
	// begins at; both are zero before the first walk.
	src, fold int
}

// source answers sourceOffsetOfFold(c.s, foldOff) for a foldOff no smaller
// than the last one asked.
func (c *foldCursor) source(foldOff int) int {
	folded := c.fold
	for at, r := range foldRunes(c.s[c.src:]) {
		next := folded + utf8.RuneLen(r)
		if next > foldOff {
			c.src, c.fold = c.src+at, folded
			return c.src
		}
		folded = next
	}
	c.src, c.fold = len(c.s), folded
	return c.src
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

// landingBlocked reports that inserted page text interrupts a source span.
// Normalization barriers also withhold any overlapping span.
func (e *entry) landingBlocked(start, end int) bool {
	for _, at := range e.insertions {
		if start < at && at < end {
			return true
		}
	}
	for _, span := range e.insertionBarriers {
		if start < span[1] && end > span[0] {
			return true
		}
	}
	return false
}

// insertionBounds clips a run to the adjacent text the page keeps on the
// same side of every inserted label. An edge at the match belongs to its side.
func (e *entry) insertionBounds(at, low, high int) (floor, ceiling int) {
	for _, edge := range e.insertions {
		if edge <= at {
			low = max(low, edge)
		} else {
			high = min(high, edge)
		}
	}
	return low, high
}

func (e *entry) hasInsertion(start, end int) bool {
	for _, edge := range e.insertions {
		if start < edge && edge <= end {
			return true
		}
	}
	return false
}
