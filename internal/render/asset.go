package render

import (
	"fmt"
	"html"
	"net/url"
	pathpkg "path"
	"regexp"
	"strings"

	"github.com/koopa0/yomihon/internal/wording"
)

// localImageSrc matches an already-rendered local image, from the opening angle
// bracket to the closing one. The renderer emits exactly this shape — src first,
// no ">" inside an attribute value — so the pattern reads what this package
// wrote rather than trying to parse arbitrary HTML.
//
// It holds the whole tag rather than stopping at the src, because a caller that
// puts something around the match must be putting it around an element. Stopping
// at the source's closing quote made the mark below open inside the tag: the
// browser then read the marker as an attribute of the image, dropped the alt
// text, and printed the rest of the tag as words on the page.
var localImageSrc = regexp.MustCompile(`(<img src=")([^"]*)("[^>]*>)`)

// resolveAssetHrefs rewrites the local image sources in htmlOut so a browser asks
// for the bytes instead of for a reading page. noteRelPath is the note the
// sources were written in, which is not always the note being displayed, since a
// transcluded body carries its own directory; a destination leaving the vault is
// left as it was. It reads an emitted <img> rather than an image node because
// each body is parsed on its own and could not answer which note a source is from.
// What it reads is attribute text rather than a URL, so a source crosses out of
// the attribute to be resolved and is written back into one afterwards. A source
// it leaves alone is returned as the tag it arrived in, so an address it has no
// business rewriting keeps the spelling its author gave it.
func resolveAssetHrefs(htmlOut, noteRelPath string, files Files, lang wording.Lang, diags *[]Diagnostic) string {
	noteDir := pathpkg.Dir(noteRelPath)
	if noteDir == "." {
		noteDir = ""
	}
	return localImageSrc.ReplaceAllStringFunc(htmlOut, func(tag string) string {
		parts := localImageSrc.FindStringSubmatch(tag)
		src := attributeUnescaper.Replace(parts[2])
		resolved, ok := rawAssetHref(src, noteDir)
		if !ok {
			return tag
		}
		rewritten := parts[1] + attributeEscaper.Replace(resolved) + parts[3]
		// The address resolved, so it names a path inside the vault: the same
		// resolution the rewrite above just used. What is asked now is whether
		// anything is there.
		//
		// A path the scan does not walk — one with a hidden segment — answers
		// no, and stays unmarked. Its bytes are not served either, so the
		// reader sees a broken image with nothing said about it; naming that
		// would mean reporting where this reader looks as a fault in the note,
		// and where the picture actually is, is the author's own business.
		target, _ := splitSuffix(src)
		joined, _ := vaultPath(target, noteDir)
		if !files.MissingFile(joined) {
			return rewritten
		}
		*diags = append(*diags, Diagnostic{
			Kind:    DiagImageMissing,
			Target:  joined,
			Message: fmt.Sprintf("the vault holds no file at %q, so this picture cannot be shown", joined),
		})
		return missingImage(rewritten, joined, lang)
	})
}

// missingImage marks a picture whose file the vault does not hold. The tag is
// kept exactly as it was resolved — the address is what the author wrote and
// the browser's own broken-image mark is part of what the reader sees — and the
// explanation is added around it in the shape every citation the page could not
// follow already takes: a title for whoever can point at it, the same sentence
// offscreen for whoever is listening.
func missingImage(tag, target string, lang wording.Lang) string {
	reason := fmt.Sprintf(wording.UnwrittenFileFmt.In(lang), target)
	escaped := html.EscapeString(reason)
	return `<span class="image-missing" title="` + escaped + `">` + tag +
		`<span class="` + offscreenNoteClass + `">` +
		html.EscapeString(wording.ParenOpen.In(lang)) + escaped +
		html.EscapeString(wording.ParenClose.In(lang)) + `</span></span>`
}

// servedElsewhere reports that a source already names something the reader can
// fetch, or something that is not a vault path at all. Each of these would be
// broken by a rewrite rather than fixed by one.
func servedElsewhere(src string) bool {
	if src == "" || strings.HasPrefix(strings.ToLower(src), "data:") {
		return true
	}
	if strings.Contains(src, "://") || strings.HasPrefix(src, "//") {
		return true
	}
	for _, route := range []string{"/raw/", "/notes/", "/static/"} {
		if strings.HasPrefix(src, route) {
			return true
		}
	}
	return false
}

// splitSuffix separates a query and fragment from the path they decorate, so
// the path can be resolved and they can be reattached unchanged.
func splitSuffix(src string) (target, suffix string) {
	target = src
	if base, rest, found := strings.Cut(target, "?"); found {
		target, suffix = base, "?"+rest
	}
	if base, rest, found := strings.Cut(target, "#"); found {
		target, suffix = base, "#"+rest+suffix
	}
	return target, suffix
}

// vaultPath resolves one source against the note's own directory and reports the
// vault-relative path it names, or false when it names none: a source climbing
// past the root is not a vault asset.
func vaultPath(target, noteDir string) (string, bool) {
	decoded, err := url.PathUnescape(target)
	if err != nil {
		return "", false
	}
	// A root-relative source names the vault root, which is the reading
	// surface's own idea of "/" — not the filesystem's.
	var joined string
	if rooted, found := strings.CutPrefix(decoded, "/"); found {
		joined = pathpkg.Clean(rooted)
	} else {
		joined = pathpkg.Clean(pathpkg.Join(noteDir, decoded))
	}
	if joined == "." || joined == ".." || strings.HasPrefix(joined, "../") || strings.HasPrefix(joined, "/") {
		return "", false
	}
	return joined, true
}

// rawAssetHref maps one rendered image source onto the raw-bytes route, or
// reports that it should be left exactly as it is.
func rawAssetHref(src, noteDir string) (string, bool) {
	if servedElsewhere(src) {
		return "", false
	}
	target, suffix := splitSuffix(src)
	joined, ok := vaultPath(target, noteDir)
	if !ok {
		return "", false
	}
	// The escaping is rawHref's, not a second copy of it: the route that serves
	// these bytes decodes one spelling, and a rule written twice in one package
	// is one a later change fixes in one of its two places.
	return rawHref(joined) + suffix, true
}
