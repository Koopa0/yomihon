package render

import (
	"html"
	"regexp"
	"strings"
)

var (
	// ttsMarkedParagraph is the explicit authoring contract for a speakable
	// paragraph. A lesson author places <!-- read-aloud: ja --> immediately before
	// the Markdown paragraph; goldmark preserves that trusted raw-HTML comment,
	// and this pass consumes it into one read-aloud line. This is what lets pure
	// kana practice opt in without pretending ruby is a language declaration.
	ttsMarkedParagraph = regexp.MustCompile(`(?s)<!--\s*read-aloud:\s*ja\s*-->\s*<p>(.*?)</p>`)
	// rubyReading matches a ruby reading annotation — an <rt>…</rt> or an
	// <rp>…</rp>, each closed by its own tag — so a caller that strips it keeps
	// the base characters and drops the furigana. The open tag may carry
	// attributes; only its name is anchored, so an annotation is removed whole
	// rather than having its wrapper stripped and the reading left behind. Two
	// alternations (not a single [tp] class) so an <rt> can never pair with a
	// stray </rp> on malformed markup. <rp> does not appear in current content,
	// but it is stripped too, defensively — its fallback parentheses are
	// reading apparatus, not part of the base text.
	rubyReading = regexp.MustCompile(`(?s)<rt[^>]*>.*?</rt>|<rp[^>]*>.*?</rp>`)
	// nestedParaOpen detects a raw inline paragraph tag (<p> or <p …>) inside a
	// paragraph's own inner HTML. goldmark never nests block <p>, so this only
	// ever fires on a hand-authored raw inline <p> pasted mid-sentence — the one
	// input that would make ttsParagraph's non-greedy match stop at the wrong
	// </p>. The `[>\s]` guard keeps it from matching inline SVG's <path>.
	nestedParaOpen = regexp.MustCompile(`<p[>\s]`)
	// ttsTag matches any remaining tag, reducing a segment's inner HTML to its
	// text: the <ruby> wrappers, emphasis, links and the rest fall away.
	ttsTag = regexp.MustCompile(`<[^>]+>`)
)

// ttsSpeaker is the speak button's inline speaker icon (stroke-only, matching
// the repo's other inline SVGs).
const ttsSpeaker = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M11 5 6 9H2v6h4l5 4z"></path><path d="M15.54 8.46a5 5 0 0 1 0 7.07"></path><path d="M19.07 4.93a10 10 0 0 1 0 14.14"></path></svg>`

// InjectTTS gives each explicitly marked Japanese paragraph a speak button
// whose data-tts attribute holds the segment's spoken text, computed here
// (server-side) with the furigana
// stripped: the front end reads data-tts, it never crawls the DOM to
// reconstruct reading-free text.
//
// It ADDS a control to such segments; it does not reshape the locked dialect
// constructs (wikilinks, callouts, embeds, ==highlight==, or the <ruby>
// itself), which the render fixtures pin as substrings and which survive
// untouched. This is read-path enrichment, not a content edit — the
// never-edit-a-note contract governs a note's own bytes, not derived HTML.
//
// Ruby is reading apparatus, not a language or interaction declaration. A
// paragraph, recall item, or answer that merely contains ruby stays untouched;
// the author must opt a paragraph in with <!-- read-aloud: ja -->. Keeping that
// semantic decision in source content prevents presentation from guessing.
func InjectTTS(htmlOut string) string {
	return injectMarkedParagraphTTS(htmlOut)
}

// injectMarkedParagraphTTS consumes the explicit author marker and wraps the
// following paragraph whether or not it contains ruby. The paragraph gains
// lang=ja for assistive technology.
func injectMarkedParagraphTTS(htmlOut string) string {
	return ttsMarkedParagraph.ReplaceAllStringFunc(htmlOut, func(marked string) string {
		inner := ttsMarkedParagraph.FindStringSubmatch(marked)[1]
		if nestedParaOpen.MatchString(inner) {
			return marked
		}
		spoken := spokenText(inner)
		if spoken == "" {
			return `<p lang="ja">` + inner + `</p>`
		}
		return `<div class="y-reading" lang="ja">` + speakButton(spoken) +
			`<p lang="ja">` + inner + `</p></div>`
	})
}

// spokenText reduces a segment's inner HTML to its spoken form: the ruby
// readings (<rt>/<rp>) removed so only the base characters remain, every other
// tag stripped, HTML entities decoded, and the result trimmed.
func spokenText(inner string) string {
	s := ttsTag.ReplaceAllString(rubyReading.ReplaceAllString(inner, ""), "")
	return strings.TrimSpace(html.UnescapeString(s))
}

// speakButton emits the read-aloud control for an opted-in paragraph. text is
// already the reading-stripped spoken form; it is attribute-escaped into
// data-tts.
func speakButton(text string) string {
	return `<button class="y-tts" type="button" data-tts="` + html.EscapeString(text) +
		`" aria-label="Read this sentence aloud">` + ttsSpeaker + `</button>`
}
