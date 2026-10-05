package graph

import (
	"net/url"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/vault"
)

// MarkdownResolution is the reading of a local Markdown destination. Relative
// is its note-relative spelling even when no captured file answers to it.
// Suffix preserves the authored query and fragment without decoding either.
type MarkdownResolution struct {
	Resolution

	Local    bool
	Invalid  bool
	Outside  bool
	Withheld bool
	Relative string
	Suffix   string
}

// ResolveMarkdown collects every captured file a Markdown path can name. Its
// interpretations are note-relative, vault-root and whole path suffixes; two
// different files remain ambiguous even when one interpretation came first.
// Authorization precedes each membership observation, including absent paths.
func (idx *Index) ResolveMarkdown(source, destination string, allowed, contains func(string) bool) MarkdownResolution {
	if allowed == nil || contains == nil {
		panic("graph: ResolveMarkdown requires authorization and membership")
	}
	result, interpretations, shortest := markdownReadings(source, destination)
	if !result.Local || result.Invalid || result.Outside {
		return result
	}
	candidates, authorized := idx.markdownCandidates(interpretations, shortest, allowed)
	if !authorized {
		result.Withheld = true
		return result
	}
	// Every authorization check finishes before membership is observed. A
	// private interpretation cannot turn a public one into a guessed answer.
	candidates = slices.DeleteFunc(candidates, func(candidate string) bool { return !contains(candidate) })
	slices.Sort(candidates)
	candidates = slices.Compact(candidates)
	switch len(candidates) {
	case 0:
	case 1:
		result.Kind, result.RelPath = KindUnique, candidates[0]
	default:
		result.Kind, result.Candidates = KindAmbiguous, candidates
	}
	return result
}

func markdownReadings(source, destination string) (result MarkdownResolution, interpretations []string, shortest string) {
	raw, suffix := markdownPathAndSuffix(destination)
	if !markdownLocalPath(raw) {
		return result, nil, ""
	}
	result.Local, result.Suffix = true, suffix
	decoded, err := url.PathUnescape(raw)
	if err != nil || !markdownLocalPath(decoded) || !utf8.ValidString(decoded) || strings.ContainsFunc(decoded, func(r rune) bool {
		return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == '\\'
	}) {
		result.Invalid = true
		return result, nil, ""
	}
	noteRelative := path.Clean(path.Join(path.Dir(source), decoded))
	if rooted, found := strings.CutPrefix(decoded, "/"); found {
		noteRelative = path.Clean(rooted)
	}
	result.Relative = vault.NormalizeNFC(noteRelative)
	if !markdownWithinRoot(noteRelative) {
		result.Outside = true
		return result, nil, ""
	}
	shortest = path.Clean(strings.TrimPrefix(decoded, "/"))
	interpretations = []string{result.Relative}
	if markdownWithinRoot(shortest) {
		interpretations = append(interpretations, vault.NormalizeNFC(shortest))
	}
	return result, interpretations, shortest
}

func (idx *Index) markdownCandidates(interpretations []string, shortest string, allowed func(string) bool) ([]string, bool) {
	for _, candidate := range interpretations {
		if vault.OutsideScan(candidate) || !allowed(candidate) {
			return nil, false
		}
	}
	var candidates []string
	for _, candidate := range idx.paths {
		if !markdownCandidate(candidate, interpretations, shortest) {
			continue
		}
		if vault.OutsideScan(candidate) || !allowed(candidate) {
			return nil, false
		}
		candidates = append(candidates, candidate)
	}
	return candidates, true
}

func markdownCandidate(candidate string, interpretations []string, shortest string) bool {
	key := NormalizeKey(candidate)
	for _, reading := range interpretations {
		if key == NormalizeKey(reading) {
			return true
		}
	}
	return markdownWithinRoot(shortest) && (key == NormalizeKey(shortest) || strings.HasSuffix(key, "/"+NormalizeKey(shortest)))
}

func markdownWithinRoot(candidate string) bool {
	return candidate != "." && candidate != ".." && !strings.HasPrefix(candidate, "../") && !strings.HasPrefix(candidate, "/")
}

func markdownLocalPath(value string) bool {
	if value == "" || strings.HasPrefix(value, "//") || markdownHasScheme(value) || strings.HasPrefix(value, "~") {
		return false
	}
	for _, route := range []string{"/notes/", "/raw/", "/static/"} {
		if strings.HasPrefix(value, route) {
			return false
		}
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
