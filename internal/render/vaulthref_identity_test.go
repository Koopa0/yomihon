package render_test

import (
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// TestVaultHrefIsByteIdenticalToEscapedVaultPath holds the cross-package
// invariant VaultHref documents: a rail link and an in-body wikilink to the
// same note must be the same bytes. The two escapers are kept as two copies
// (archlock counts the literal, not the output), so a quoting tweak in one
// that the other does not take would pass the whole suite. This table feeds
// both the same names and compares the bytes.
func TestVaultHrefIsByteIdenticalToEscapedVaultPath(t *testing.T) {
	t.Parallel()

	composed := norm.NFC.String("caf\u00e9")
	decomposed := norm.NFD.String("caf\u00e9")
	if composed == decomposed {
		t.Fatal("caf\u00e9 has one Unicode form, so a case for each would say nothing about the other")
	}

	tests := []struct {
		name string
		path string
	}{
		{name: "an ampersand stays a path byte", path: "Notes/A&B.md"},
		{name: "a space is percent-escaped", path: "Notes/a note.md"},
		{name: "a hash cannot start a fragment", path: "Notes/C#basics.md"},
		{name: "a percent cannot start a stray escape", path: "Notes/50% done.md"},
		{name: "a question mark cannot start a query", path: "Notes/what is this?.md"},
		{name: "CJK is UTF-8 percent-escaped", path: "Notes/中文/讀懂 yomihon.md"},
		{name: "a slash inside a name stays a separator", path: "Notes/a/b.md"},
		{name: "NFC composed form", path: "Notes/" + composed + ".md"},
		{name: "decomposed form", path: "Notes/" + decomposed + ".md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := pages.VaultHref("/notes/", tt.path)
			want := "/notes/" + render.EscapedVaultPath(tt.path)
			if got != want {
				t.Errorf("VaultHref(\"/notes/\", %q) = %q, \"/notes/\" + escapedVaultPath(%q) = %q",
					tt.path, got, tt.path, want)
			}
		})
	}
}
