package vault_test

import (
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

func TestContentIdentityDefinition(t *testing.T) {
	t.Parallel()
	// The identity of a note with one status line is the SHA-256 of its bytes
	// with that line's text removed — the newline that ended it stays. The
	// expectation is spelled out by hand so this test pins the definition
	// rather than echoing the implementation.
	const note = "---\ntitle: T\nstatus: draft\ntags: [a]\n---\nbody\n"
	const spliced = "---\ntitle: T\n\ntags: [a]\n---\nbody\n"
	if got, want := vault.ContentIdentity([]byte(note)), sha256.Sum256([]byte(spliced)); got != want {
		t.Errorf("ContentIdentity = %x, want the hash of the note without its status line %x", got, want)
	}
}

func TestContentIdentity(t *testing.T) {
	t.Parallel()
	const note = "---\ntitle: T\nstatus: draft\ntags: [a]\n---\nbody\n"

	tests := []struct {
		name string
		a, b string
		same bool
	}{
		{
			name: "a status value change alone leaves the identity unchanged",
			a:    note,
			b:    strings.Replace(note, "status: draft", "status: ready", 1),
			same: true,
		},
		{
			name: "a status value change alone leaves a crlf identity unchanged",
			a:    strings.ReplaceAll(note, "\n", "\r\n"),
			b:    strings.ReplaceAll(strings.Replace(note, "status: draft", "status: ready", 1), "\n", "\r\n"),
			same: true,
		},
		{
			name: "a body edit changes the identity",
			a:    note,
			b:    strings.Replace(note, "body", "edited body", 1),
			same: false,
		},
		{
			name: "a frontmatter edit outside the status line changes the identity",
			a:    note,
			b:    strings.Replace(note, "title: T", "title: U", 1),
			same: false,
		},
		{
			name: "identical bytes share one identity",
			a:    note,
			b:    note,
			same: true,
		},
		{
			name: "without a frontmatter block every byte counts",
			a:    "status: draft\nplain text\n",
			b:    "status: ready\nplain text\n",
			same: false,
		},
		{
			name: "with two status lines every byte counts",
			a:    "---\nstatus: draft\nstatus: draft\n---\nbody\n",
			b:    "---\nstatus: ready\nstatus: draft\n---\nbody\n",
			same: false,
		},
		{
			name: "without any status line every byte counts",
			a:    "---\ntitle: T\n---\nbody\n",
			b:    "---\ntitle: U\n---\nbody\n",
			same: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := vault.ContentIdentity([]byte(tt.a))
			b := vault.ContentIdentity([]byte(tt.b))
			if (a == b) != tt.same {
				t.Errorf("ContentIdentity(%q) == ContentIdentity(%q) is %t, want %t", tt.a, tt.b, a == b, tt.same)
			}
		})
	}
}
