package render

import (
	"net/url"
	"regexp"
	"strings"
)

// conceptLink matches one resolved wikilink as renderWikilink emits it, the
// unique case only. An ambiguous or broken one is a span, never this shape, so a
// term the reader cannot reliably follow never opens a sheet.
var conceptLink = regexp.MustCompile(`<a href="([^"]*)" class="wikilink">`)

// InjectConceptTriggers upgrades the wikilinks pointing at concept notes into
// concept-sheet triggers, and reports which concepts a body references. A trigger
// stays a real navigable anchor and only gains an attribute and a class, so
// without scripting it still opens the concept note's own page. The returned refs
// are the concept paths, deduped in first-seen order.
func InjectConceptTriggers(htmlOut string, lookup func(relPath string) (id string, ok bool)) (out string, refs []string) {
	seen := map[string]bool{}
	out = conceptLink.ReplaceAllStringFunc(htmlOut, func(tag string) string {
		href := conceptLink.FindStringSubmatch(tag)[1]
		rel := decodeNotesHref(attributeUnescaper.Replace(href))
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
		// The href goes back exactly as it was read. It is already attribute
		// text, so escaping it here would put a second layer on a value that
		// carries one; only the copy handed to decodeNotesHref crosses out.
		return `<a href="` + href + `" class="wikilink concept-link" data-concept="` + id + `">`
	})
	return out, refs
}

// decodeNotesHref reverses notesHref. It takes the URL, not the attribute text
// that URL was read out of: it strips the /notes/ prefix and path-unescapes each
// segment back to a vault-relative path. A href that is not a /notes/ link (or
// fails to decode) yields "" — not a concept, skip it.
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
