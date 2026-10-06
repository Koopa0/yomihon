package render

import (
	"html"
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// readAloudWord is the word that opens the comment an author places before a
// paragraph to have it spoken. It is spelled once: the pattern below and the
// cheap question MayMarkReadAloud asks both read it.
const readAloudWord = "read-aloud"

var (
	// ttsMarkedParagraph is the authoring contract for a speakable paragraph: an
	// author places a language marker immediately before it, and this pass
	// consumes the comment into one read-aloud line.
	ttsMarkedParagraph       = regexp.MustCompile(`(?s)(<!--\s*` + readAloudWord + `:[^<>]*?-->)\s*<p>(.*?)</p>`)
	readAloudLanguagePattern = regexp.MustCompile(`^<!--[ \t\r\n]*` + readAloudWord + `:[ \t\r\n]*([^<>]*?)[ \t\r\n]*-->$`)
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
	// trailingBlockAddressSpan is a trailing span whose only content is text —
	// the shape markBlockAnchor plants for a classified address. Nested markup
	// is someone else's span and is left for the flatten.
	trailingBlockAddressSpan = regexp.MustCompile(`<span(?:\s+[^>]*)?>[^<]*</span>\s*$`)
	// footnoteReference is one citation mark as goldmark writes it: the mark's
	// own id and a link down to the note's footnote list.
	footnoteReference = regexp.MustCompile(`<sup id="[^"]*"><a href="#[^"]*" class="footnote-ref"[^>]*>([^<]*)</a></sup>`)
)

// ttsSpeaker is the speak button's inline speaker icon (stroke-only, matching
// the repo's other inline SVGs).
const ttsSpeaker = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M11 5 6 9H2v6h4l5 4z"></path><path d="M15.54 8.46a5 5 0 0 1 0 7.07"></path><path d="M19.07 4.93a10 10 0 0 1 0 14.14"></path></svg>`

// InjectTTS gives each explicitly marked paragraph a speak button whose
// data-tts attribute holds the segment's spoken text, computed here with the
// furigana stripped, so the front end never crawls the DOM for it. It only adds a
// control. Ruby is reading apparatus rather than a language declaration, so a
// paragraph merely containing it stays untouched and the author opts in.
func InjectTTS(htmlOut string, lang wording.Lang) string {
	return eachMarkedParagraph(htmlOut, func(inner, spoken, tag string) string {
		return readAloudBlock(inner, spoken, tag, lang)
	})
}

// MayMarkReadAloud reports whether a note's rendered page could carry a
// read-aloud marker at all: the source says the marker's word, or embeds
// something that may be another note, whose words arrive on this page and may
// say it. A picture or a PDF brings no words, so an embed of one does not
// count; any other embed does, since a note's own name can hold a dot. It is a
// necessary condition and not a sufficient one: the word can sit in prose or in
// a comment the grammar does not claim, so a caller that needs the answer
// renders the note and asks MarkedParagraphs. Its use is to spare a page that
// only wants to know whether anything in a whole path is marked from rendering
// every note in it when none of them could be. The one shape it misses is a
// note whose own name ends in a picture's extension, embedded by that name.
func MayMarkReadAloud(body string) bool {
	return strings.Contains(body, readAloudWord) || embedsSomethingThatMayBeANote(body)
}

// embedsSomethingThatMayBeANote reports whether body writes an embed whose
// target is not named as a picture or a PDF. The target is what stands before a
// display width or a section in the embed's brackets.
func embedsSomethingThatMayBeANote(body string) bool {
	const open = "![["
	for rest := body; ; {
		at := strings.Index(rest, open)
		if at < 0 {
			return false
		}
		rest = rest[at+len(open):]
		inner, _, _ := strings.Cut(rest, "]]")
		target, _, _ := strings.Cut(inner, "|")
		target, _, _ = strings.Cut(target, "#")
		target = strings.TrimSpace(target)
		if !IsPicture(target) && !IsPDF(target) {
			return true
		}
	}
}

// MarkedParagraphs returns every paragraph an author marked to be read aloud,
// in document order, as the read-aloud elements a page renders for them. It is
// for a page that gathers paragraphs from notes it does not otherwise show: the
// bytes are the note's own page's, so one runtime drives both, except that a
// footnote citation keeps its mark and loses its link — the list it pointed at
// belongs to the note, which is not on this page.
func MarkedParagraphs(htmlOut string, lang wording.Lang) []string {
	var found []string
	eachMarkedParagraph(htmlOut, func(inner, spoken, tag string) string {
		found = append(found, readAloudBlock(footnoteReference.ReplaceAllString(inner, "<sup>$1</sup>"), spoken, tag, lang))
		return ""
	})
	return found
}

// eachMarkedParagraph is the one reader of the read-aloud marker's grammar: it
// consumes the explicit author marker and hands the following paragraph to
// replace, whether or not the paragraph contains ruby. A paragraph the grammar
// cannot claim is left as the author's own and never reaches replace — nothing
// asks twice and gets two answers.
func eachMarkedParagraph(htmlOut string, replace func(inner, spoken, tag string) string) string {
	return ttsMarkedParagraph.ReplaceAllStringFunc(htmlOut, func(marked string) string {
		parts := ttsMarkedParagraph.FindStringSubmatch(marked)
		tag, valid := readAloudLanguage(parts[1])
		if !valid {
			return marked
		}
		inner := parts[2]
		if nestedParaOpen.MatchString(inner) {
			return marked
		}
		spoken := spokenText(inner)
		if spoken == "" {
			return `<p lang="` + tag + `">` + inner + `</p>`
		}
		return replace(inner, spoken, tag)
	})
}

// readAloudLanguage shares the schema's canonical tag authority between safe
// markup admission and the paragraph traversal. A marker owns no HTML beyond
// its comment; only a validated tag may become a language attribute.
func readAloudLanguage(marker string) (string, bool) {
	parts := readAloudLanguagePattern.FindStringSubmatch(marker)
	if parts == nil {
		return "", false
	}
	tag, err := schema.ParseLanguageTag(strings.Trim(parts[1], " \t\r\n"))
	return tag, err == nil
}

// readAloudBlock is the element a marked paragraph becomes: the wrapper, the
// speaker, and the paragraph, both declaring their authored language.
func readAloudBlock(inner, spoken, tag string, lang wording.Lang) string {
	return `<div class="y-reading" lang="` + tag + `">` + speakButton(spoken, lang) +
		`<p lang="` + tag + `">` + inner + `</p></div>`
}

// spokenText reduces a segment's inner HTML to its spoken form: a trailing
// block address dropped first, then the ruby readings (<rt>/<rp>) removed so
// only the base characters remain, every other tag stripped, HTML entities
// decoded, and the result trimmed. The address is taken off the HTML, not
// off the flattened text: a caret inside a code span, an escaped caret, or
// an entity-spelled one can flatten to the same characters and must stay.
func spokenText(inner string) string {
	s := ttsTag.ReplaceAllString(rubyReading.ReplaceAllString(stripTrailingBlockAddress(inner), ""), "")
	return strings.TrimSpace(html.UnescapeString(s))
}

// stripTrailingBlockAddress removes a trailing span that blockMarkerTail
// classifies as an address. It does not invent a second caret grammar, and it
// does not look at a flattened tail: the span is the signal the preprocess
// pass left for a classified marker, claimed or not.
func stripTrailingBlockAddress(inner string) string {
	loc := trailingBlockAddressSpan.FindStringIndex(inner)
	if loc == nil {
		return inner
	}
	span := strings.TrimSpace(inner[loc[0]:loc[1]])
	openAt := strings.Index(span, ">")
	endAt := strings.LastIndex(span, "</span>")
	if openAt < 0 || endAt <= openAt {
		return inner
	}
	text := html.UnescapeString(span[openAt+1 : endAt])
	if !blockMarkerTail.MatchString(text) {
		return inner
	}
	return inner[:loc[0]]
}

// speakButton emits the read-aloud control for an opted-in paragraph. text is
// already the reading-stripped spoken form; it is attribute-escaped into
// data-tts.
func speakButton(text string, lang wording.Lang) string {
	return `<button class="y-tts" type="button" data-tts="` + html.EscapeString(text) +
		`" lang="` + lang.Tag() + `" aria-label="` + html.EscapeString(wording.ReadAloud.In(lang)) + `">` + ttsSpeaker + `</button>`
}
