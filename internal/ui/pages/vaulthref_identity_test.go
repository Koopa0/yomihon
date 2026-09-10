package pages_test

import (
	"html"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestVaultHrefIsByteIdenticalToARenderedWikilink holds the invariant
// VaultHref documents: a rail link and an in-body wikilink to the same note
// must be the same bytes. The two escapers are kept as two copies (archlock
// counts the literal, not the output), so a quoting tweak in one that the
// other does not take would pass the whole suite. This table puts each path
// in the index, renders [[…]] through the exported pipeline, reads the href
// the body link carries, and compares it to VaultHref.
func TestVaultHrefIsByteIdenticalToARenderedWikilink(t *testing.T) {
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
			target, aliases := wikilinkTarget(tt.path)
			idx := graph.BuildFromNotes([]graph.NoteInput{{RelPath: tt.path, Aliases: aliases}}, nil)
			pipe := render.New(idx, emptyBodies{}, noTitles{}, vaultHolds{})
			page := pipe.HTML("Notes/here.md", "", "[["+target+"]]", wording.ZhHant).HTML
			got := pages.VaultHref("/notes/", tt.path)
			want := renderedWikilinkHref(t, page)
			if got != want {
				t.Errorf("VaultHref(\"/notes/\", %q) = %q, rendered [[%s]] href = %q\n%s",
					tt.path, got, target, want, page)
			}
		})
	}
}

// wikilinkTarget is the inner of [[…]] that resolves to path. A "#" (or "^",
// "|") inside a name is a fragment marker to the parser, so those notes are
// reached through an alias; the href still comes from the stored RelPath.
func wikilinkTarget(path string) (target string, aliases []string) {
	stem := strings.TrimSuffix(path, ".md")
	if strings.ContainsAny(stem, "#^|") {
		return "hash-in-name", []string{"hash-in-name"}
	}
	return stem, nil
}

func renderedWikilinkHref(t *testing.T, page string) string {
	t.Helper()
	if strings.Contains(page, "wikilink-degraded") {
		t.Fatalf("wikilink did not resolve:\n%s", page)
	}
	const mark = ` class="wikilink"`
	at := strings.Index(page, mark)
	if at < 0 {
		t.Fatalf("no resolved wikilink in:\n%s", page)
	}
	open := strings.LastIndex(page[:at], `<a href="`)
	if open < 0 {
		t.Fatalf("wikilink has no href:\n%s", page)
	}
	rest := page[open+len(`<a href="`):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("href is unclosed:\n%s", page)
	}
	return html.UnescapeString(rest[:end])
}

type emptyBodies struct{}

func (emptyBodies) Transclusion(string) (string, bool) { return "", false }

type noTitles struct{}

func (noTitles) TitledBy(string) []string { return nil }

type vaultHolds struct{}

func (vaultHolds) MissingFile(string) bool { return false }
