package render

import (
	"html"
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/wording"
)

var (
	// ttsMarkedParagraph is the authoring contract for a speakable paragraph: an
	// author places <!-- read-aloud: ja --> immediately before it, and this pass
	// consumes the comment into one read-aloud line.
	ttsMarkedParagraph = regexp.MustCompile(`(?s)<!--\s*read-aloud:\s*ja\s*-->\s*<p>(.*?)</p>`)
	// rubyReading matches a ruby reading annotation, each closed by its own tag,
	// so a caller stripping it keeps the base characters and drops the furigana.
	// Only the tag name is anchored, so an annotation carrying attributes is
	// removed whole. Two alternations rather than one character class, so an <rt>
	// can never pair with a stray </rp> on malformed markup.
	rubyReading = regexp.MustCompile(`(?s)<rt[^>]*>.*?</rt>|<rp[^>]*>.*?</rp>`)
	// nestedParaOpen detects a raw inline paragraph tag inside a paragraph's own
	// inner HTML — the one input that would make ttsParagraph's non-greedy match
	// stop at the wrong close. The `[>\s]` guard keeps it off inline SVG's <path>.
	nestedParaOpen = regexp.MustCompile(`<p[>\s]`)
	// ttsTag matches any remaining tag, reducing a segment's inner HTML to its
	// text: the <ruby> wrappers, emphasis, links and the rest fall away.
	ttsTag = regexp.MustCompile(`<[^>]+>`)
)

// ttsSpeaker is the speak button's inline speaker icon (stroke-only, matching
// the repo's other inline SVGs).
const ttsSpeaker = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M11 5 6 9H2v6h4l5 4z"></path><path d="M15.54 8.46a5 5 0 0 1 0 7.07"></path><path d="M19.07 4.93a10 10 0 0 1 0 14.14"></path></svg>`

// InjectTTS gives each explicitly marked Japanese paragraph a speak button whose
// data-tts attribute holds the segment's spoken text, computed here with the
// furigana stripped, so the front end never crawls the DOM for it. It only adds a
// control. Ruby is reading apparatus rather than a language declaration, so a
// paragraph merely containing it stays untouched and the author opts in.
func InjectTTS(htmlOut string, lang wording.Lang) string {
	return injectMarkedParagraphTTS(htmlOut, lang)
}

// injectMarkedParagraphTTS consumes the explicit author marker and wraps the
// following paragraph whether or not it contains ruby. The paragraph gains
// lang=ja for assistive technology.
func injectMarkedParagraphTTS(htmlOut string, lang wording.Lang) string {
	return ttsMarkedParagraph.ReplaceAllStringFunc(htmlOut, func(marked string) string {
		inner := ttsMarkedParagraph.FindStringSubmatch(marked)[1]
		if nestedParaOpen.MatchString(inner) {
			return marked
		}
		spoken := spokenText(inner)
		if spoken == "" {
			return `<p lang="ja">` + inner + `</p>`
		}
		return `<div class="y-reading" lang="ja">` + speakButton(spoken, lang) +
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
func speakButton(text string, lang wording.Lang) string {
	return `<button class="y-tts" type="button" data-tts="` + html.EscapeString(text) +
		`" lang="` + lang.Tag() + `" aria-label="` + html.EscapeString(wording.ReadAloud.In(lang)) + `">` + ttsSpeaker + `</button>`
}
