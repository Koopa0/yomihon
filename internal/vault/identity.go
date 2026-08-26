package vault

import "crypto/sha256"

// ContentIdentity returns the SHA-256 identity of a note's content: every
// byte of data except its frontmatter status value. That value is the note's
// adjudication state — pages that offer transitions read it live and bind it
// separately — so a rewrite of the value, and nothing else, leaves the
// identity unchanged. The excised bytes are the span StatusValueSpan reports;
// when it reports none, the identity covers all of data, which is the
// fail-closed direction: a line no write can touch has nothing to keep out of
// the compare.
//
// The rest of the status line stays inside. A reason its author wrote in a
// trailing comment is content, and a ruling read against one version of it
// must not install over another; the write preserves those bytes, so nothing
// yomihon does can disturb the identity it just bound itself to.
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
