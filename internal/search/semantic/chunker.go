package semantic

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/render"
)

const (
	// ChunkTokenCap is the current document-submission budget: 90 percent of
	// the model's 8192-token input limit under the conservative local proxy.
	ChunkTokenCap              = 7372
	documentPromptTitlePrefix  = "title: "
	documentPromptTextJoin     = " | text: "
	documentTitleFallback      = "none"
	documentHeadingSeparator   = " › "
	documentContinuationPrefix = " — part "
	documentContinuationJoin   = "/"
	chunkerDescriptorVersion   = "yomihon-heading-bounded-chunker-v1"
)

// ChunkFailureKind is a stable local classification for a section the chunker
// could not represent without violating its cap.
type ChunkFailureKind string

const (
	// ChunkFailurePrefixConsumesCap means the title/heading/continuation context
	// leaves no room for even one body rune.
	ChunkFailurePrefixConsumesCap ChunkFailureKind = "prefix-consumes-cap"
	// ChunkFailureInvalidCap means the caller supplied no positive token budget.
	ChunkFailureInvalidCap ChunkFailureKind = "invalid-cap"
)

// ChunkFailure identifies one heading-bounded section that could not produce a
// chunk. Any failure makes the later embedding epoch incomplete.
type ChunkFailure struct {
	Section  int
	Headings []string
	Reason   ChunkFailureKind
}

// TextChunk is one bounded document submission. Submitted is the exact provider
// input; Body remains separate for current-vault evidence and snippets.
type TextChunk struct {
	Ordinal     int
	Headings    []string
	Part        int
	Parts       int
	Body        string
	Submitted   string
	ProxyTokens int
}

type continuation struct {
	part  int
	total int
}

// Chunking holds every kept chunk and every loud section failure.
type Chunking struct {
	Chunks   []TextChunk
	Failures []ChunkFailure
}

// ProxyTokens applies the exact local proxy: each Han, Hiragana, Katakana,
// CJK-symbol, or fullwidth rune counts one; every four remaining runes count
// one, rounded up.
func ProxyTokens(s string) int {
	var cjk, other int
	for _, r := range s {
		if unicode.Is(unicode.Han, r) ||
			unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) ||
			(r >= 0x3000 && r <= 0x303f) ||
			(r >= 0xff00 && r <= 0xffef) {
			cjk++
			continue
		}
		other++
	}
	return cjk + (other+3)/4
}

// ChunkNote extracts heading-bounded plain-text sections and enforces cap
// with the fixed ladder: top-level block/paragraph boundaries, then line
// boundaries, then rune boundaries. Empty or markup-only sections are dropped.
func ChunkNote(title, body string, tokenCap int) Chunking {
	sections := render.PlainSections(body)
	result := Chunking{}
	for sectionIndex, section := range sections {
		if section.Text == "" {
			continue
		}
		base := documentPrefix(title, section.Headings, 1, 1)
		if tokenCap <= 0 {
			result.Failures = append(result.Failures, newFailure(sectionIndex, section.Headings, ChunkFailureInvalidCap))
			continue
		}
		if ProxyTokens(base) >= tokenCap {
			result.Failures = append(result.Failures, newFailure(sectionIndex, section.Headings, ChunkFailurePrefixConsumesCap))
			continue
		}
		if submitted := base + section.Text; ProxyTokens(submitted) <= tokenCap {
			result.Chunks = append(result.Chunks, newChunk(
				len(result.Chunks), section.Headings, continuation{part: 1, total: 1}, section.Text, submitted,
			))
			continue
		}

		parts, ok := splitSection(title, section, tokenCap)
		if !ok {
			result.Failures = append(result.Failures, newFailure(sectionIndex, section.Headings, ChunkFailurePrefixConsumesCap))
			continue
		}
		for i, part := range parts {
			prefix := documentPrefix(title, section.Headings, i+1, len(parts))
			submitted := prefix + part
			result.Chunks = append(result.Chunks, newChunk(
				len(result.Chunks), section.Headings, continuation{part: i + 1, total: len(parts)}, part, submitted,
			))
		}
	}
	return result
}

func newFailure(section int, headings []string, reason ChunkFailureKind) ChunkFailure {
	return ChunkFailure{Section: section, Headings: slices.Clone(headings), Reason: reason}
}

func newChunk(ordinal int, headings []string, position continuation, body, submitted string) TextChunk {
	return TextChunk{
		Ordinal:     ordinal,
		Headings:    slices.Clone(headings),
		Part:        position.part,
		Parts:       position.total,
		Body:        body,
		Submitted:   submitted,
		ProxyTokens: ProxyTokens(submitted),
	}
}

func documentContext(title string, headings []string) string {
	if title == "" {
		title = documentTitleFallback
	}
	context := make([]string, 0, len(headings)+1)
	context = append(context, title)
	for _, heading := range headings {
		if heading != "" {
			context = append(context, heading)
		}
	}
	return strings.Join(context, documentHeadingSeparator)
}

// documentPrefix is the exact Gemini Embedding 2 document-prompt structure.
// Continuation identity belongs to the provider title, while Body remains the
// verbatim chunk text following the fixed text label.
func documentPrefix(title string, headings []string, part, parts int) string {
	context := documentContext(title, headings)
	if parts > 1 {
		context += documentContinuationPrefix + strconv.Itoa(part) + documentContinuationJoin + strconv.Itoa(parts)
	}
	return documentPromptTitlePrefix + context + documentPromptTextJoin
}

// splitSection reserves a continuation prefix large enough for the number of
// parts. The reserved digit width only grows; this makes the loop converge and
// may conservatively create one extra part at a power-of-ten boundary, never
// an over-cap submission.
func splitSection(title string, section render.PlainSection, tokenCap int) ([]string, bool) {
	digits := 1
	for {
		placeholder := strings.Repeat("9", digits)
		prefix := documentPromptTitlePrefix + documentContext(title, section.Headings) +
			documentContinuationPrefix + placeholder + documentContinuationJoin + placeholder + documentPromptTextJoin
		if ProxyTokens(prefix) >= tokenCap {
			return nil, false
		}
		parts, ok := splitBlocks(section.Blocks, prefix, tokenCap)
		if !ok || len(parts) == 0 {
			return nil, false
		}
		neededDigits := len(strconv.Itoa(len(parts)))
		if neededDigits <= digits {
			return parts, true
		}
		digits = neededDigits
	}
}

func splitBlocks(blocks []render.PlainBlock, prefix string, tokenCap int) ([]string, bool) {
	accumulator := blockAccumulator{prefix: prefix, tokenCap: tokenCap}
	for _, block := range blocks {
		if block.Code {
			// An oversized section never packs a fence together with prose. Its
			// own lines are the next boundary before a hard rune split.
			accumulator.flush()
			pieces, ok := splitLines(block.Text, prefix, tokenCap)
			if !ok {
				return nil, false
			}
			accumulator.out = append(accumulator.out, pieces...)
			continue
		}
		if !accumulator.add(block.Text) {
			return nil, false
		}
	}
	accumulator.flush()
	return accumulator.out, true
}

type blockAccumulator struct {
	prefix   string
	tokenCap int
	out      []string
	pending  string
}

func (a *blockAccumulator) flush() {
	if a.pending == "" {
		return
	}
	a.out = append(a.out, a.pending)
	a.pending = ""
}

func (a *blockAccumulator) add(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	if a.pending != "" {
		candidate := a.pending + "\n" + text
		if fits(a.prefix, candidate, a.tokenCap) {
			a.pending = candidate
			return true
		}
		a.flush()
	}
	if fits(a.prefix, text, a.tokenCap) {
		a.pending = text
		return true
	}
	pieces, ok := splitLines(text, a.prefix, a.tokenCap)
	if !ok {
		return false
	}
	a.out = append(a.out, pieces...)
	return true
}

func splitLines(text, prefix string, tokenCap int) ([]string, bool) {
	lines := strings.Split(text, "\n")
	var out []string
	var pending string
	flush := func() {
		if pending != "" {
			out = append(out, pending)
			pending = ""
		}
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if pending != "" {
			candidate := pending + "\n" + line
			if fits(prefix, candidate, tokenCap) {
				pending = candidate
				continue
			}
			flush()
		}
		if fits(prefix, line, tokenCap) {
			pending = line
			continue
		}
		pieces, ok := splitRunes(line, prefix, tokenCap)
		if !ok {
			return nil, false
		}
		out = append(out, pieces...)
	}
	flush()
	return out, true
}

func splitRunes(text, prefix string, tokenCap int) ([]string, bool) {
	var out []string
	var current strings.Builder
	for _, r := range text {
		candidate := current.String() + string(r)
		if fits(prefix, candidate, tokenCap) {
			current.WriteRune(r)
			continue
		}
		if current.Len() == 0 {
			return nil, false
		}
		out = append(out, current.String())
		current.Reset()
		if !fits(prefix, string(r), tokenCap) {
			return nil, false
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out, utf8.ValidString(text)
}

func fits(prefix, body string, tokenCap int) bool {
	return body != "" && ProxyTokens(prefix+body) <= tokenCap
}
