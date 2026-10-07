package lexical

import (
	"html"
	"strings"

	"github.com/koopa0/yomihon/internal/render"
)

// DocumentFromBriefing retains a registered report's searchable source and
// annotates the characters its excerpt must hide or decode. Registration is
// the caller's decision; an ordinary HTML file remains searchable as source.
func DocumentFromBriefing(relPath string, data []byte) Document {
	document := DocumentFromFile(relPath, data)
	document.DisplaySpans = briefingSpans(document.PlainText)
	return document
}

func briefingSpans(source string) []render.DisplaySpan {
	var spans []render.DisplaySpan
	for at := 0; at < len(source); {
		if strings.HasPrefix(source[at:], "<!--") {
			end := len(source)
			if stop := strings.Index(source[at+4:], "-->"); stop >= 0 {
				end = at + 4 + stop + 3
			}
			spans = append(spans, render.DisplaySpan{Start: at, End: end, Hidden: true})
			at = end
			continue
		}
		if source[at] == '<' {
			span, ok := briefingTagSpan(source, at)
			if !ok && span.End > at {
				break
			}
			if ok {
				spans = append(spans, span)
				at = span.End
				continue
			}
		}
		if source[at] == '&' {
			if end, replacement := briefingEntity(source, at); end > at {
				spans = append(spans, render.DisplaySpan{Start: at, End: end, Replacement: replacement})
				at = end
				continue
			}
		}
		at++
	}
	return spans
}

func briefingTagSpan(source string, at int) (render.DisplaySpan, bool) {
	end, name, closing, ok := briefingTag(source, at)
	if !ok {
		return render.DisplaySpan{End: end}, false
	}
	if !closing && (name == "script" || name == "style") {
		end = briefingRawEnd(source, end, name)
	}
	span := render.DisplaySpan{Start: at, End: end, Hidden: true}
	if briefingBlockTag(name) {
		span.Hidden, span.Replacement = false, " "
	}
	return span, true
}

// briefingTag bounds a tag before reading its name, so an attribute's quoted
// greater-than sign cannot turn the rest of the attribute into excerpt text.
func briefingTag(source string, at int) (end int, name string, closing, ok bool) {
	start := at + 1
	if start >= len(source) {
		return 0, "", false, false
	}
	if source[start] == '/' {
		closing = true
		start++
	}
	if start >= len(source) {
		return 0, "", false, false
	}
	declaration := source[start] == '!' || source[start] == '?'
	if !closing && !declaration && !briefingAlpha(source[start]) {
		return 0, "", false, false
	}
	stop := start
	for stop < len(source) && briefingNameByte(source[stop]) {
		stop++
	}
	if stop == start && !declaration {
		return 0, "", false, false
	}
	if !declaration && stop < len(source) && !strings.ContainsRune(" \t\r\n\f/>", rune(source[stop])) {
		return 0, "", false, false
	}
	name = strings.ToLower(source[start:stop])
	return briefingTagEnd(source, stop, name, closing)
}

func briefingTagEnd(source string, from int, name string, closing bool) (end int, tagName string, isClosing, ok bool) {
	var quote byte
	for i := from; i < len(source); i++ {
		switch {
		case quote != 0:
			if source[i] == quote {
				quote = 0
			}
		case source[i] == '\'' || source[i] == '"':
			quote = source[i]
		case source[i] == '>':
			return i + 1, name, closing, true
		}
	}
	// An unfinished construct is left literal rather than claiming that the
	// browser repaired malformed markup in a particular way.
	return len(source), "", false, false
}

func briefingAlpha(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func briefingNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-' || b == ':'
}

func briefingRawEnd(source string, from int, name string) int {
	for from < len(source) {
		next := strings.IndexByte(source[from:], '<')
		if next < 0 {
			break
		}
		at := from + next
		if !briefingOwnClose(source, at, name) {
			from = at + 1
			continue
		}
		end, _, _, ok := briefingTag(source, at)
		if ok {
			return end
		}
		if !ok && end > at {
			return len(source)
		}
		from = at + 1
	}
	return len(source)
}

// Raw text only leaves its element through its own closing name. Reject other
// markup before scanning attributes, so literal openers cannot rescan a suffix.
func briefingOwnClose(source string, at int, name string) bool {
	start := at + 2
	stop := start + len(name)
	if stop > len(source) || !strings.HasPrefix(source[at:], "</") || !strings.EqualFold(source[start:stop], name) {
		return false
	}
	return stop == len(source) || strings.ContainsRune(" \t\r\n\f/>", rune(source[stop]))
}

// Block boundaries separate the words the page places in different elements.
// Inline tags add no space: a formatted part of a word remains the same word.
func briefingBlockTag(name string) bool {
	switch name {
	case "address", "article", "aside", "blockquote", "br", "caption", "dd", "div", "dl", "dt", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav", "ol", "p", "pre", "section", "table", "td", "th", "tr", "ul":
		return true
	}
	return false
}

func briefingEntity(source string, at int) (end int, replacement string) {
	stop := at + 1
	numeric := stop < len(source) && source[stop] == '#'
	if numeric {
		return briefingNumericEntity(source, at, stop+1)
	}
	for stop < len(source) && briefingAlphanumeric(source[stop]) {
		stop++
	}
	if stop < len(source) && source[stop] == ';' {
		stop++
	}
	raw := source[at:stop]
	decoded := html.UnescapeString(raw)
	if decoded == raw {
		return 0, ""
	}
	// UnescapeString can consume only a prefix of a semicolon-less name.
	// Find that prefix through the decoder itself; the unchanged suffix is
	// still authored text and must keep its own query-hit provenance.
	for i := 2; i <= len(raw); i++ {
		prefix := html.UnescapeString(raw[:i])
		if prefix != raw[:i] && prefix+raw[i:] == decoded {
			return at + i, prefix
		}
	}
	return 0, ""
}

func briefingNumericEntity(source string, at, stop int) (end int, replacement string) {
	hex := false
	if stop < len(source) && (source[stop] == 'x' || source[stop] == 'X') {
		hex = true
		stop++
	}
	for stop < len(source) && briefingEntityDigit(source[stop], hex) {
		stop++
	}
	if stop < len(source) && source[stop] == ';' {
		stop++
	}
	// A short semicolon-less number needs its next character as decoder
	// context. Decode that character too, then leave it at its source site.
	contextEnd := min(len(source), stop+1)
	raw := source[at:contextEnd]
	decoded := html.UnescapeString(raw)
	if decoded != raw {
		return stop, strings.TrimSuffix(decoded, source[stop:contextEnd])
	}
	return 0, ""
}

func briefingEntityDigit(b byte, hex bool) bool {
	return b >= '0' && b <= '9' || hex && (b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F')
}

func briefingAlphanumeric(b byte) bool {
	return briefingAlpha(b) || b >= '0' && b <= '9'
}
