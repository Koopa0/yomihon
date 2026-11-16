package vault_test

import (
	"crypto/sha256"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

// TestContentIdentityCoversEveryByte pins that no part of a note is left out
// of its identity: a change to any frontmatter value, the draft flag included,
// is a change to the note.
func TestContentIdentityCoversEveryByte(t *testing.T) {
	t.Parallel()
	const note = "---\ntitle: T\ndraft: true\n---\nbody\n"
	if got, want := vault.ContentIdentity([]byte(note)), sha256.Sum256([]byte(note)); got != want {
		t.Errorf("ContentIdentity = %x, want the hash of every byte %x", got, want)
	}
	for _, other := range []string{
		"---\ntitle: T\ndraft: false\n---\nbody\n",
		"---\ntitle: T\n---\nbody\n",
		"---\ntitle: T\ndraft: true\n---\nbody.\n",
	} {
		if vault.ContentIdentity([]byte(note)) == vault.ContentIdentity([]byte(other)) {
			t.Errorf("ContentIdentity(%q) equals ContentIdentity(%q)", note, other)
		}
	}
}
