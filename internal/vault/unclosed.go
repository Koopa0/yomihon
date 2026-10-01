package vault

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// OpensUnclosedFrontmatter reports whether data opens a frontmatter fence that
// nothing closes: the first line is "---", the line after it reads as a "key:"
// line, and SplitFrontmatter found no closing fence, so every reader took the
// whole file for body text.
//
// It answers beside SplitFrontmatter and never inside it. The split's contract
// is shared with the status write path, which must keep reading an unterminated
// opening fence as no block at all; this is only the question a reader asks
// afterwards, of the bytes that came back as no block, whether the author
// plainly meant one. It takes the closing verdict from SplitFrontmatter itself,
// so what it calls unclosed is exactly what the readers did not close.
//
// The "key:" line is what keeps a note that opens with a thematic break out of
// it. A rule followed by a blank line, a heading or a sentence is a rule; a rule
// followed by a line shaped like a field is a block whose closing line was lost.
func OpensUnclosedFrontmatter(data []byte) bool {
	// The opening fence is read the way SplitFrontmatter reads it, a byte-order
	// mark stepped over and the fence an exact "---" line in either line
	// ending, so a file is never called unclosed for a fence the split would
	// not have opened.
	opening, _ := bytes.CutPrefix(data, []byte("\xef\xbb\xbf"))
	rest, found := bytes.CutPrefix(opening, []byte("---\n"))
	if !found {
		if rest, found = bytes.CutPrefix(opening, []byte("---\r\n")); !found {
			return false
		}
	}
	line, _, _ := bytes.Cut(rest, []byte("\n"))
	if !looksLikeKeyLine(bytes.TrimSuffix(line, []byte("\r"))) {
		return false
	}
	_, closed := SplitFrontmatter(data)
	return !closed
}

// looksLikeKeyLine reports whether line begins a YAML mapping entry the way a
// frontmatter field does: a name of letters, digits, "_" and "-" that starts
// with a letter or "_", then a colon, then a space, a tab or the end of the
// line. A colon run straight into text is not an entry in YAML either, which is
// what keeps a bare address such as "https://example.com" from reading as one.
func looksLikeKeyLine(line []byte) bool {
	rest := line
	for i := 0; len(rest) > 0; i++ {
		r, size := utf8.DecodeRune(rest)
		switch {
		case r == ':':
			if i == 0 {
				return false
			}
			next := rest[size:]
			return len(next) == 0 || next[0] == ' ' || next[0] == '\t'
		case unicode.IsLetter(r) || r == '_':
		case i > 0 && (unicode.IsDigit(r) || r == '-'):
		default:
			return false
		}
		rest = rest[size:]
	}
	return false
}
