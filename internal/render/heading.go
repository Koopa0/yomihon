package render

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/vault"
)

var (
	headingTag = regexp.MustCompile(`(?s)<h([1-6])>(.*?)</h[1-6]>`)
	slugDrop   = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	tagStrip   = regexp.MustCompile(`<[^>]+>`)

	// offscreenNote matches the explanation this renderer attaches, out of
	// sight, to a link whose target is unwritten. It is dropped with its
	// contents rather than unwrapped: those words say why the link goes
	// nowhere, which is an answer to the reader who pauses on the link and
	// never part of what the section holding it is called. Left in, they
	// became the heading's anchor, its table-of-contents entry, and the
	// fragment of every address written at that section. The element is
	// renderer-owned and never nests another, so the shortest close is its
	// own.
	offscreenNote = regexp.MustCompile(`(?s)<span class="` + offscreenNoteClass + `">.*?</span>`)

	// nestedHeadingOpen detects a raw inline <h1-6> inside a heading's own inner
	// HTML. goldmark never nests block headings, so this only fires on a raw
	// inline <hN> pasted mid-heading — the one input that makes headingTag's
	// non-greedy match stop at the wrong </hN>, truncating the heading. The
	// `[>\s]` guard keeps <h1-6> from matching <hr>/<header>/<hgroup>.
	nestedHeadingOpen = regexp.MustCompile(`<h[1-6][>\s]`)
)

// foldFragment is the fold the two kinds of "#fragment" share: Unicode form
// and letter case, and nothing else. A name written on one side of the vault
// and read back on the other differs in exactly these two ways for reasons the
// author never chose — the filesystem hands over decomposed Japanese while the
// editor composes it, and a name typed in a link rarely repeats the heading's
// capitals — while every other difference is something they did choose.
func foldFragment(s string) string {
	return strings.ToLower(vault.NormalizeNFC(s))
}

// slugify makes a CJK-safe fragment/DOM id from heading text: normalize to
// NFC, lowercase, keep only Unicode letters and digits (\p{L}, \p{N}),
// collapse every run of anything else to a single hyphen, trim
// leading/trailing hyphens, and fall back to "section" when nothing is left.
// Keeping every Unicode letter, not just ASCII, is what lets a CJK heading
// produce a usable id instead of an empty string.
//
// The NFC step comes first because two spellings of the same Japanese word are
// the ordinary case here, not an exotic one: this vault's filenames arrive
// decomposed from the filesystem and its prose arrives composed from an
// editor. Left as bytes, a combining mark counts as "not a letter" and becomes
// a hyphen, so か+◌゙ん slugs to "か-ん" while がん slugs to "がん" — two anchors
// for one heading, and a link that misses with nothing on screen to show why.
// It uses vault.NormalizeNFC so the repository keeps one definition of the
// fold rather than a second one that agrees only by maintenance.
func slugify(s string) string {
	s = slugDrop.ReplaceAllString(foldFragment(s), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "section"
	}
	return s
}

// headingInnerText reduces a heading's inner markup to the words the reader
// sees in it: a ruby heading carries its reading inside <rt>, and a link to an
// unwritten note carries the sentence saying so out of sight, and both are
// dropped with their contents before the remaining tags are — so the text keeps
// the base characters once rather than the kana echoed after them, and names a
// section by the words on screen rather than by an explanation of one of them.
// Character references then resolve to the characters they name. The scan that
// answers an embed's "#section" fragment reduces heading source through this
// same step, so the two surfaces agree on what a section is called.
//
// Both removals run before the tags do, and therefore before any character
// reference is resolved: authored markup arrives escaped, so a note writing
// either element by hand keeps it as the text it is.
func headingInnerText(inner string) string {
	inner = offscreenNote.ReplaceAllString(inner, "")
	inner = rubyReading.ReplaceAllString(inner, "")
	return strings.TrimSpace(html.UnescapeString(tagStrip.ReplaceAllString(inner, "")))
}

// assignHeadingSlugs walks the final rendered HTML, assigns each h1-h6 a
// slugify'd id, and collects the TOC in document order. Two headings
// producing the same base slug are disambiguated by bumping a numeric
// suffix until it lands on an id not already assigned anywhere earlier
// in the document — not merely counted per base slug — so a heading whose
// own natural text happens to collide with a generated "-2" form can
// never silently duplicate an id.
//
// reserved is an id already spoken for elsewhere on the page — the anchor a
// removed opening heading passed to the visible title — and is taken as
// assigned before the walk begins. The title is above this HTML and outside
// it, so nothing in the walk could otherwise see it, and a section further
// down reducing to the same name would quietly issue the id a second time.
func assignHeadingSlugs(htmlOut, reserved string) (string, []TOCEntry) {
	var toc []TOCEntry
	seen := map[string]bool{}
	if reserved != "" {
		seen[reserved] = true
	}
	out := headingTag.ReplaceAllStringFunc(htmlOut, func(tag string) string {
		m := headingTag.FindStringSubmatch(tag)
		level := int(m[1][0] - '0') // headingTag guarantees a single 1-6 digit
		inner := m[2]

		// A raw inline <hN> inside this heading would have made the non-greedy
		// match stop at the inner </hN> — truncating the heading and unbalancing
		// tags. Leave it byte-identical (no id, no TOC entry): degrade, never
		// corrupt (mirrors tts.go's nested-<p> guard).
		if nestedHeadingOpen.MatchString(inner) {
			return tag
		}
		text := headingInnerText(inner)

		id := slugify(text)
		if seen[id] {
			for n := 2; ; n++ {
				cand := fmt.Sprintf("%s-%d", id, n)
				if !seen[cand] {
					id = cand
					break
				}
			}
		}
		seen[id] = true

		toc = append(toc, TOCEntry{Level: level, Text: text, ID: id})
		//nolint:gocritic // sprintfQuotedString false positive: the quotes are HTML attribute syntax, not Go string quoting (id is already a slugify() output — [a-z0-9-] only, no HTML-special chars)
		return fmt.Sprintf(`<h%d id="%s">%s</h%d>`, level, id, inner, level)
	})
	return out, toc
}
