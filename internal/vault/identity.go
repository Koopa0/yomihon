package vault

import (
	"bytes"
	"crypto/sha256"
)

// ContentIdentity returns the SHA-256 identity of a note's content: every
// byte of data except the text of its frontmatter status line. The status
// line is the note's adjudication state — pages that offer transitions read
// it live and bind it separately — so a rewrite of that one line, and
// nothing else, leaves the identity unchanged. When data carries no
// frontmatter block or does not hold exactly one status line, the identity
// covers all of data.
func ContentIdentity(data []byte) [sha256.Size]byte {
	start, end, found := statusLineSpan(data)
	if !found {
		return sha256.Sum256(data)
	}
	spliced := make([]byte, 0, len(data)-(end-start))
	spliced = append(spliced, data[:start]...)
	spliced = append(spliced, data[end:]...)
	return sha256.Sum256(spliced)
}

// statusLineSpan locates the single line beginning with "status:" inside
// data's frontmatter block and returns the byte range of the line's text —
// its trailing newline excluded, a carriage return before that newline
// included. It reports false when data has no frontmatter block or when the
// block holds any number of such lines other than one, the same line shape
// the surgical status rewrite replaces.
func statusLineSpan(data []byte) (start, end int, found bool) {
	block, ok := SplitFrontmatter(data)
	if !ok {
		return 0, 0, false
	}
	offset := 0
	for line := range bytes.SplitSeq(block.Content, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("status:")) {
			if found {
				return 0, 0, false
			}
			start = block.ContentStart + offset
			end = start + len(line)
			found = true
		}
		offset += len(line) + 1
	}
	return start, end, found
}
