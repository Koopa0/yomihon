package note

import (
	"slices"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

// TestRecentHomeNotesMarkOnlyDrafts pins what a recent row says about its
// note: the author's draft flag and nothing else, so a finished note carries
// no label at all.
func TestRecentHomeNotesMarkOnlyDrafts(t *testing.T) {
	t.Parallel()
	notes := []nav.NoteSummary{
		{Title: "Open", RelPath: "Concepts/open.md", Type: "concept", Draft: true, Modified: time.Unix(2, 0)},
		{Title: "Done", RelPath: "Concepts/done.md", Type: "concept", Modified: time.Unix(1, 0)},
	}
	recent, _ := recentShelfNotes(notes, nil)
	if len(recent) != len(notes) {
		t.Fatalf("recentShelfNotes returned %d rows, want %d", len(recent), len(notes))
	}
	for _, n := range recent {
		if want := n.RelPath == "Concepts/open.md"; n.Draft != want {
			t.Errorf("row %q draft = %t, want %t", n.RelPath, n.Draft, want)
		}
	}
}

func TestRecentShelfNotesCarryDeclaredLanguage(t *testing.T) {
	t.Parallel()
	lookup := pages.ArticleLanguageFor(func(relPath string) string {
		if relPath == "Writing/lessons/japanese/L01.md" {
			return "ja"
		}
		return ""
	})
	notes := []nav.NoteSummary{{
		Title:   "L01 わたしは学生です",
		RelPath: "Writing/lessons/japanese/L01.md",
		Type:    "lesson",
	}}
	recent, _ := recentShelfNotes(notes, lookup)
	if len(recent) != 1 {
		t.Fatalf("recentShelfNotes returned %d rows, want 1", len(recent))
	}
	if recent[0].Language != "ja" {
		t.Errorf("recent row language = %q, want ja", recent[0].Language)
	}
}

// A fresh clone stamps every file with one moment, so the tie-break is what
// the recent shelf shows. It must be the vault's one reading order, the same
// a folder lists, not a byte order that puts Untitled 2 before Untitled and
// TODO before index.
func TestRecentShelfNotesBreakTiesInReadingOrder(t *testing.T) {
	t.Parallel()
	stamp := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var notes []nav.NoteSummary
	for _, rel := range []string{"Letters/Zed.md", "Letters/Untitled 2.md", "Letters/TODO.md", "Letters/Untitled.md", "Letters/index.md"} {
		notes = append(notes, nav.NoteSummary{Title: "Same", RelPath: rel, Modified: stamp})
	}
	recent, _ := recentShelfNotes(notes, nil)
	got := make([]string, 0, len(recent))
	for _, n := range recent {
		got = append(got, n.RelPath)
	}
	want := []string{"Letters/index.md", "Letters/TODO.md", "Letters/Untitled.md", "Letters/Untitled 2.md", "Letters/Zed.md"}
	if !slices.Equal(got, want) {
		t.Errorf("caught: recent shelf tie order = %q, want %q", got, want)
	}
}
