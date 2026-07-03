package render

import (
	"html"
	"regexp"
	"strings"
)

var (
	// ttsParagraph matches one goldmark-emitted paragraph. goldmark emits a
	// bare <p> (no attributes); a paragraph that somehow carried attributes
	// simply would not match and is left untouched — safe by construction. A
	// <p> cannot nest, so the non-greedy body never over-reaches, and literal
	// "<p>" inside code is escaped by goldmark, so only real paragraph tags
	// match (the same assumption heading.go's <h1-6> pass relies on).
	ttsParagraph = regexp.MustCompile(`(?s)<p>(.*?)</p>`)
	// ttsReading matches a ruby reading annotation — an <rt>…</rt> or an
	// <rp>…</rp>, each closed by its OWN tag — so the spoken text keeps the base
	// characters and never the furigana. Two alternations (not a single [tp]
	// class) so an <rt> can never pair with a stray </rp> on malformed markup.
	// <rp> does not appear in current content, but the spec requires stripping
	// it too, defensively.
	ttsReading = regexp.MustCompile(`(?s)<rt>.*?</rt>|<rp>.*?</rp>`)
	// nestedParaOpen detects a raw inline paragraph tag (<p> or <p …>) inside a
	// paragraph's own inner HTML. goldmark never nests block <p>, so this only
	// ever fires on a hand-authored raw inline <p> pasted mid-sentence — the one
	// input that would make ttsParagraph's non-greedy match stop at the wrong
	// </p>. The `[>\s]` guard keeps it from matching inline SVG's <path>.
	nestedParaOpen = regexp.MustCompile(`<p[>\s]`)
	// ttsTag matches any remaining tag, reducing a paragraph's inner HTML to
	// its text: the <ruby> wrappers, emphasis, links and the rest fall away.
	ttsTag = regexp.MustCompile(`<[^>]+>`)
)

// ttsSpeaker is the speak button's inline speaker icon (stroke-only, matching
// the repo's other inline SVGs).
const ttsSpeaker = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M11 5 6 9H2v6h4l5 4z"></path><path d="M15.54 8.46a5 5 0 0 1 0 7.07"></path><path d="M19.07 4.93a10 10 0 0 1 0 14.14"></path></svg>`

// InjectTTS wraps every readable Japanese segment — a paragraph containing
// <ruby> — with a speak button whose data-tts attribute holds the paragraph's
// spoken text, computed HERE (server-side) with the furigana stripped: the
// front end reads data-tts, it never crawls the DOM to reconstruct
// reading-free text.
//
// It ADDS a sibling wrapper around such paragraphs; it does not reshape the
// locked dialect constructs (wikilinks, callouts, embeds, ==highlight==, or the
// <ruby> itself), which the render fixtures pin as substrings and which survive
// untouched inside the wrapped <p>. This is read-path enrichment, not a content
// edit — wall 4 governs edits to a note's own bytes, not derived HTML.
//
// It is deliberately GENERIC: it keys only on <ruby>, never on note type. The
// note handler calls it for lesson notes ONLY, so a diary or concept note that
// happens to contain <ruby> never grows a speaker button. Keeping the type
// decision in the handler, not here, is what keeps render unaware that "lesson"
// exists at all.
func InjectTTS(htmlOut string) string {
	return ttsParagraph.ReplaceAllStringFunc(htmlOut, func(para string) string {
		inner := ttsParagraph.FindStringSubmatch(para)[1]
		if !strings.Contains(inner, "<ruby") {
			return para
		}
		// A raw inline <p> inside this paragraph would have made the non-greedy
		// match stop at the inner </p> — losing the tail text and unbalancing
		// tags. Leave such a paragraph unwrapped (no speak button): the match
		// consumed only up to the inner </p>, so returning it unchanged passes
		// the whole paragraph through byte-for-byte, and the sentence still
		// reads (wall 4: degrade, never corrupt).
		if nestedParaOpen.MatchString(inner) {
			return para
		}
		spoken := ttsTag.ReplaceAllString(ttsReading.ReplaceAllString(inner, ""), "")
		spoken = strings.TrimSpace(html.UnescapeString(spoken))
		if spoken == "" {
			return para
		}
		return `<div class="k-reading">` +
			`<button class="k-tts" type="button" data-tts="` + html.EscapeString(spoken) + `" aria-label="Read this sentence aloud">` +
			ttsSpeaker +
			`</button>` + para + `</div>`
	})
}
