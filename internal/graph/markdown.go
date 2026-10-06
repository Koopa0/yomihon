package graph

import (
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/vault"
)

// MarkdownResolution is a once-decoded path relative to the authoring note.
// Checkable names the Markdown-note domain shared by the page and judge.
// Suffix keeps the authored query and fragment without decoding either.
type MarkdownResolution struct {
	Resolution

	Local     bool
	Invalid   bool
	Checkable bool
	Outside   bool
	Withheld  bool
	Relative  string
	Suffix    string
}

// ResolveMarkdown resolves only the note-relative path using exact captured
// membership. Target authorization finishes before membership is observed.
func ResolveMarkdown(source, destination string, allowed, contains func(string) bool) MarkdownResolution {
	if allowed == nil {
		panic("graph: ResolveMarkdown requires a non-nil allowed")
	}
	if contains == nil {
		panic("graph: ResolveMarkdown requires a non-nil contains")
	}
	result := ParseMarkdownPath(source, destination)
	if !result.Local || result.Invalid || result.Outside {
		return result
	}
	if vault.OutsideScan(result.Relative) || !allowed(result.Relative) {
		result.Withheld = true
		return result
	}
	if contains(result.Relative) {
		result.Kind, result.RelPath = KindUnique, result.Relative
	}
	return result
}

// ParseMarkdownPath separates the raw suffix, decodes the pathname once and
// normalizes its note-relative spelling to NFC. It observes no membership.
func ParseMarkdownPath(source, destination string) (result MarkdownResolution) {
	raw, suffix := markdownPathAndSuffix(destination)
	if !markdownLocalPath(raw) {
		return result
	}
	result.Local, result.Suffix = true, suffix
	decoded, err := url.PathUnescape(raw)
	if err != nil || !markdownLocalPath(decoded) || !utf8.ValidString(decoded) || strings.ContainsFunc(decoded, func(r rune) bool {
		return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == '\\'
	}) {
		result.Invalid = true
		return result
	}
	result.Checkable = vault.IsMarkdown(decoded) && !strings.ContainsAny(decoded, "*<>")
	result.Relative = vault.NormalizeNFC(path.Clean(path.Join(path.Dir(source), decoded)))
	result.Outside = !markdownWithinRoot(result.Relative)
	return result
}

func markdownWithinRoot(candidate string) bool {
	return candidate != "." && candidate != ".." && !strings.HasPrefix(candidate, "../") && !strings.HasPrefix(candidate, "/")
}

func markdownLocalPath(value string) bool {
	if value == "" || strings.HasPrefix(value, "/") || markdownHasScheme(value) || strings.HasPrefix(value, "~") {
		return false
	}
	return true
}

// A colon names a URI scheme only after an ASCII scheme token. A colon after
// a directory separator instead belongs to the filename being resolved.
func markdownHasScheme(value string) bool {
	at := strings.IndexByte(value, ':')
	if at <= 0 {
		return false
	}
	for i, r := range value[:at] {
		letter := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
		continuation := (r >= '0' && r <= '9') || r == '+' || r == '-' || r == '.'
		if !letter && (i == 0 || !continuation) {
			return false
		}
	}
	return true
}

func markdownPathAndSuffix(destination string) (pathname, suffix string) {
	if at := strings.IndexAny(destination, "?#"); at >= 0 {
		return destination[:at], destination[at:]
	}
	return destination, ""
}
