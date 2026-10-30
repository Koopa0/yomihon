package note

import (
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/wording"
)

func TestThoughtNoteName(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.October, 30, 9, 5, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		title string
		lang  wording.Lang
		want  string
	}{
		{"chinese", "提早返回的管線", wording.ZhHant, "提早返回的管線的想法 2026-10-30 0905"},
		{"english", "Early returns", wording.En, "Thought on Early returns 2026-10-30 0905"},
		{"every refused character", `a\b/c:d*e"f<g>h?i|j[k]l#m^n`, wording.En, "Thought on a b c d e f g h i j k l m n 2026-10-30 0905"},
		{"a leading dot would hide the file", "..secret", wording.En, "Thought on secret 2026-10-30 0905"},
		{"control characters and runs of space", "a\tb\n  c", wording.En, "Thought on a b c 2026-10-30 0905"},
		{"nothing is left of the title", "???", wording.En, "Thought on 2026-10-30 0905"},
		{"title that is only a dot", ".", wording.ZhHant, "的想法 2026-10-30 0905"},
		{"percent and ampersand are ordinary", "50% & more", wording.En, "Thought on 50% & more 2026-10-30 0905"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := thoughtNoteName(tc.title, tc.lang, at); got != tc.want {
				t.Errorf("thoughtNoteName(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

func TestThoughtNoteNameBoundsALongTitle(t *testing.T) {
	t.Parallel()
	long := ""
	for range 200 {
		long += "長"
	}
	name := thoughtNoteName(long, wording.ZhHant, time.Date(2026, time.October, 30, 9, 5, 0, 0, time.UTC))
	// File systems cap a name at 255 bytes, and Obsidian adds ".md".
	if len(name)+len(".md") > 255 {
		t.Errorf("name is %d bytes, want it to fit a file system's limit", len(name))
	}
}
