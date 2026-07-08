package render

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	headingTag = regexp.MustCompile(`(?s)<h([1-6])>(.*?)</h[1-6]>`)
	slugDrop   = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	tagStrip   = regexp.MustCompile(`<[^>]+>`)

	// nestedHeadingOpen detects a raw inline <h1-6> inside a heading's own inner
	// HTML. goldmark never nests block headings, so this only fires on a raw
	// inline <hN> pasted mid-heading — the one input that makes headingTag's
	// non-greedy match stop at the wrong </hN>, truncating the heading. The
	// `[>\s]` guard keeps <h1-6> from matching <hr>/<header>/<hgroup>.
	nestedHeadingOpen = regexp.MustCompile(`<h[1-6][>\s]`)
)

// slugify makes a CJK-safe fragment/DOM id from heading text: lowercase,
// keep only Unicode letters and digits (\p{L}, \p{N}), collapse every run
// of anything else to a single hyphen, trim leading/trailing hyphens, and
// fall back to "section" when nothing is left. Keeping every Unicode
// letter, not just ASCII, is what lets a CJK heading produce a usable id
// instead of an empty string.
func slugify(s string) string {
	s = slugDrop.ReplaceAllString(strings.ToLower(s), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "section"
	}
	return s
}

// assignHeadingSlugs walks the final rendered HTML, assigns each h1-h6 a
// slugify'd id, and collects the TOC in document order. Two headings
// producing the same base slug are disambiguated by bumping a numeric
// suffix until it lands on an id not already assigned anywhere earlier
// in the document — not merely counted per base slug — so a heading whose
// own natural text happens to collide with a generated "-2" form can
// never silently duplicate an id.
func assignHeadingSlugs(htmlOut string) (string, []TOCEntry) {
	var toc []TOCEntry
	seen := map[string]bool{}
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
		// A ruby heading carries its reading inside <rt>; strip that before
		// reducing the remaining tags so the entry and its anchor keep the base
		// characters once, not the kana echoed after them.
		text := strings.TrimSpace(html.UnescapeString(tagStrip.ReplaceAllString(rubyReading.ReplaceAllString(inner, ""), "")))

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
