package vault

import "crypto/sha256"

// ContentIdentity returns the SHA-256 identity of a note's content: every
// byte of data except the text of its frontmatter status line. The status
// line is the note's adjudication state — pages that offer transitions read
// it live and bind it separately — so a rewrite of that one line, and
// nothing else, leaves the identity unchanged. The excised bytes are the
// span StatusLineSpan reports; when it reports none, the identity covers
// all of data.
func ContentIdentity(data []byte) [sha256.Size]byte {
	start, end, ok := StatusLineSpan(data)
	if !ok {
		return sha256.Sum256(data)
	}
	spliced := make([]byte, 0, len(data)-(end-start))
	spliced = append(spliced, data[:start]...)
	spliced = append(spliced, data[end:]...)
	return sha256.Sum256(spliced)
}
