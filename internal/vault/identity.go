package vault

import "crypto/sha256"

// ContentIdentity returns the SHA-256 of a note's content: every byte of data
// except the frontmatter status value StatusValueSpan reports, so rewriting
// that value alone leaves the identity unchanged. The rest of the status line
// stays inside, a reason written in a trailing comment included. With no such
// span the identity covers all of data, which is the fail-closed direction.
func ContentIdentity(data []byte) [sha256.Size]byte {
	start, end, ok := StatusValueSpan(data)
	if !ok {
		return sha256.Sum256(data)
	}
	spliced := make([]byte, 0, len(data)-(end-start))
	spliced = append(spliced, data[:start]...)
	spliced = append(spliced, data[end:]...)
	return sha256.Sum256(spliced)
}
