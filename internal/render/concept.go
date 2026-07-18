package render

import (
	"net/url"
	"regexp"
	"strings"
)

// conceptLink matches one resolved wikilink as renderWikilink emits it —
// `<a href="…" class="wikilink">…</a>`, the Unique case only. An ambiguous or
// broken wikilink is a <span>, never this shape, so it is never turned into a
// trigger (a term the reader cannot reliably follow should not open a sheet).
var conceptLink = regexp.MustCompile(`<a href="([^"]*)" class="wikilink">`)

// InjectConceptTriggers upgrades the wikilinks that point at concept notes into
// concept-sheet triggers, and reports which concepts a body references.
//
// A trigger stays a real navigable <a> (no-JS opens the concept note's reading
// page — the approved degradation); it only gains data-concept + the
// concept-link class, so lesson.js can intercept the click and open the sheet
// instead. Like InjectTTS this is a lesson-gated post-pass over already-rendered
// HTML — render.HTML stays a generic note renderer, and the locked wikilink
// output is not reshaped, only annotated.
//
// lookup reports whether a href's decoded vault-relative path is a concept and
// its sheet ID. The returned refs are the concept paths referenced, deduped in
// first-seen order, for the caller to load into the sheet's <template>s.
func InjectConceptTriggers(htmlOut string, lookup func(relPath string) (id string, ok bool)) (out string, refs []string) {
	seen := map[string]bool{}
	out = conceptLink.ReplaceAllStringFunc(htmlOut, func(tag string) string {
		href := conceptLink.FindStringSubmatch(tag)[1]
		rel := decodeNotesHref(href)
		if rel == "" {
			return tag
		}
		id, ok := lookup(rel)
		if !ok {
			return tag
		}
		if !seen[rel] {
			seen[rel] = true
			refs = append(refs, rel)
		}
		return `<a href="` + href + `" class="wikilink concept-link" data-concept="` + id + `">`
	})
	return out, refs
}

// decodeNotesHref reverses notesHref: it strips the /notes/ prefix and
// path-unescapes each segment back to a vault-relative path. A href that is not
// a /notes/ link (or fails to decode) yields "" — not a concept, skip it.
func decodeNotesHref(href string) string {
	rest, ok := strings.CutPrefix(href, "/notes/")
	if !ok {
		return ""
	}
	segments := strings.Split(rest, "/")
	for i, s := range segments {
		dec, err := url.PathUnescape(s)
		if err != nil {
			return ""
		}
		segments[i] = dec
	}
	return strings.Join(segments, "/")
}
