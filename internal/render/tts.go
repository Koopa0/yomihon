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
	// rubyReading matches a ruby reading annotation — an <rt>…</rt> or an
	// <rp>…</rp>, each closed by its own tag — so a caller that strips it keeps
	// the base characters and drops the furigana. Two alternations (not a single
	// [tp] class) so an <rt> can never pair with a stray </rp> on malformed
	// markup. <rp> does not appear in current content, but it is stripped too,
	// defensively — its fallback parentheses are reading apparatus, not part of
	// the base text.
	rubyReading = regexp.MustCompile(`(?s)<rt>.*?</rt>|<rp>.*?</rp>`)
	// ttsListItem matches one goldmark-emitted list item — a TIGHT one, whose
	// content is inline (a sentence), which is where the practice/example
	// sentences live. A LOOSE item wraps its content in <p> and is handled by
	// the paragraph pass instead; the list pass skips it (see nestedListOpen).
	ttsListItem = regexp.MustCompile(`(?s)<li>(.*?)</li>`)
	// nestedParaOpen detects a raw inline paragraph tag (<p> or <p …>) inside a
	// paragraph's own inner HTML. goldmark never nests block <p>, so this only
	// ever fires on a hand-authored raw inline <p> pasted mid-sentence — the one
	// input that would make ttsParagraph's non-greedy match stop at the wrong
	// </p>. The `[>\s]` guard keeps it from matching inline SVG's <path>.
	nestedParaOpen = regexp.MustCompile(`<p[>\s]`)
	// nestedListOpen detects, inside a list item's own inner HTML, either a
	// paragraph (a LOOSE item, already given a button by the paragraph pass) or
	// a nested list (<ul>/<ol>/<li>, where the non-greedy <li> match would have
	// stopped at the wrong </li>). Either way the list pass leaves the item
	// untouched — no double button, no mis-match.
	nestedListOpen = regexp.MustCompile(`<(?:p|ul|ol|li)[>\s]`)
	// ttsTag matches any remaining tag, reducing a segment's inner HTML to its
	// text: the <ruby> wrappers, emphasis, links and the rest fall away.
	ttsTag = regexp.MustCompile(`<[^>]+>`)
)

// ttsSpeaker is the speak button's inline speaker icon (stroke-only, matching
// the repo's other inline SVGs).
const ttsSpeaker = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M11 5 6 9H2v6h4l5 4z"></path><path d="M15.54 8.46a5 5 0 0 1 0 7.07"></path><path d="M19.07 4.93a10 10 0 0 1 0 14.14"></path></svg>`

// InjectTTS gives every readable Japanese segment — a paragraph or a tight
// list item containing <ruby> — a speak button whose data-tts attribute holds
// the segment's spoken text, computed HERE (server-side) with the furigana
// stripped: the front end reads data-tts, it never crawls the DOM to
// reconstruct reading-free text.
//
// It ADDS a control to such segments; it does not reshape the locked dialect
// constructs (wikilinks, callouts, embeds, ==highlight==, or the <ruby>
// itself), which the render fixtures pin as substrings and which survive
// untouched. This is read-path enrichment, not a content edit — the
// never-edit-a-note contract governs a note's own bytes, not derived HTML.
//
// It is deliberately generic: it keys only on <ruby>, never on note type. The
// note handler calls it for lesson notes alone, so a diary or concept note that
// happens to contain <ruby> never grows a speaker button. Keeping the type
// decision in the handler, not here, is what keeps render unaware that "lesson"
// exists at all.
func InjectTTS(htmlOut string) string {
	htmlOut = injectParagraphTTS(htmlOut)
	htmlOut = injectListItemTTS(htmlOut)
	return htmlOut
}

// injectParagraphTTS wraps each ruby-bearing paragraph in a reading line: the
// speak button sits in the left gutter as a sibling of the untouched <p>.
func injectParagraphTTS(htmlOut string) string {
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
		// reads (degrade, never corrupt).
		if nestedParaOpen.MatchString(inner) {
			return para
		}
		spoken := spokenText(inner)
		if spoken == "" {
			return para
		}
		return `<div class="k-reading">` + speakButton(spoken) + para + `</div>`
	})
}

// injectListItemTTS gives each ruby-bearing tight list item a speak button as
// its first child. A list item cannot be wrapped without breaking list
// semantics, so the button goes inside; CSS renders it inline before the
// sentence. Loose items (their content already in a <p>) and items wrapping a
// nested list are left untouched — see nestedListOpen.
func injectListItemTTS(htmlOut string) string {
	return ttsListItem.ReplaceAllStringFunc(htmlOut, func(item string) string {
		inner := ttsListItem.FindStringSubmatch(item)[1]
		if !strings.Contains(inner, "<ruby") || nestedListOpen.MatchString(inner) {
			return item
		}
		spoken := spokenText(inner)
		if spoken == "" {
			return item
		}
		return `<li>` + speakButton(spoken) + inner + `</li>`
	})
}

// spokenText reduces a segment's inner HTML to its spoken form: the ruby
// readings (<rt>/<rp>) removed so only the base characters remain, every other
// tag stripped, HTML entities decoded, and the result trimmed.
func spokenText(inner string) string {
	s := ttsTag.ReplaceAllString(rubyReading.ReplaceAllString(inner, ""), "")
	return strings.TrimSpace(html.UnescapeString(s))
}

// speakButton is the read-aloud control emitted for both paragraphs and list
// items, so the two passes render an identical button. text is already the
// reading-stripped spoken form; it is attribute-escaped into data-tts.
func speakButton(text string) string {
	return `<button class="k-tts" type="button" data-tts="` + html.EscapeString(text) +
		`" aria-label="Read this sentence aloud">` + ttsSpeaker + `</button>`
}
