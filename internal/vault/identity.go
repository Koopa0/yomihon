package vault

import "crypto/sha256"

// ContentIdentity returns the SHA-256 of every byte of a note.
func ContentIdentity(data []byte) [sha256.Size]byte {
	return sha256.Sum256(data)
}
