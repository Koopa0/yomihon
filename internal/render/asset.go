package render

import (
	"net/url"
	pathpkg "path"
	"regexp"
	"strings"
)

// localImageSrc matches the src of an already-rendered local image. The
// renderer emits exactly this shape, so the pattern reads what this package
// wrote rather than trying to parse arbitrary HTML.
var localImageSrc = regexp.MustCompile(`(<img src=")([^"]*)(")`)

// resolveAssetHrefs rewrites the local image sources in htmlOut so a browser
// asks for the bytes instead of for a reading page.
//
// Markdown says where an image is relative to the note that mentions it, which
// is the ordinary way anyone writes one. The reading page lives under /notes/,
// so a browser resolves `../images/x.png` against that route and lands on
// another reading page — served, HTML, and not an image. The bytes are on
// /raw/.
//
// noteRelPath is the vault-relative path of the note the sources were written
// in, which is not always the note being displayed: a transcluded body carries
// its own directory with it, so it is resolved where it is spliced rather than
// against its host. A destination that leaves the vault, or that is not
// vault-local in the first place, is left exactly as it was — this resolves
// paths, and refusing them belongs to the routes that serve them.
func resolveAssetHrefs(htmlOut, noteRelPath string) string {
	noteDir := pathpkg.Dir(noteRelPath)
	if noteDir == "." {
		noteDir = ""
	}
	return localImageSrc.ReplaceAllStringFunc(htmlOut, func(tag string) string {
		parts := localImageSrc.FindStringSubmatch(tag)
		resolved, ok := rawAssetHref(parts[2], noteDir)
		if !ok {
			return tag
		}
		return parts[1] + resolved + parts[3]
	})
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

// vaultPath resolves one source against the note's own directory and reports
// the vault-relative path it names, or false when it names none — a source that
// climbs past the root is not a vault asset, and answering it here would invent
// a path no route serves.
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
	var escaped strings.Builder
	for i, segment := range strings.Split(joined, "/") {
		if i > 0 {
			escaped.WriteByte('/')
		}
		escaped.WriteString(url.PathEscape(segment))
	}
	return "/raw/" + escaped.String() + suffix, true
}
