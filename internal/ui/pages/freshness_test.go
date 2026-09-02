package pages

import (
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestFreshnessAttrsWithholdTheWatchWhereItWouldDouble pins the two cases where
// a reading page must not be watched. The page that already says its own words
// may be behind the file is the one that matters: a second notice, worded
// differently, about the same doubt is how a reader learns to read neither.
func TestFreshnessAttrsWithholdTheWatchWhereItWouldDouble(t *testing.T) {
	t.Parallel()
	const identity = "3f2a" // any non-empty value; the client never parses it

	for _, tt := range []struct {
		name  string
		view  NoteView
		watch bool
	}{
		{
			name:  "an ordinary reading page",
			view:  NoteView{RelPath: "Writing/note.md", ContentIdentity: identity},
			watch: true,
		},
		{
			name:  "a page already saying its words may be behind the file",
			view:  NoteView{RelPath: "Writing/note.md", ContentIdentity: identity, Stale: true},
			watch: false,
		},
		{
			name:  "a page carrying no identity to compare against",
			view:  NoteView{RelPath: "Writing/note.md"},
			watch: false,
		},
		{
			name:  "a page with no path to ask about",
			view:  NoteView{ContentIdentity: identity},
			watch: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			attrs := freshnessAttrs(tt.view, wording.ZhHant)
			if got := len(attrs) > 0; got != tt.watch {
				t.Fatalf("freshnessAttrs() watches = %t, want %t (attrs = %v)", got, tt.watch, attrs)
			}
			if !tt.watch {
				return
			}
			if attrs["data-freshness-path"] != tt.view.RelPath {
				t.Errorf("data-freshness-path = %v, want %q", attrs["data-freshness-path"], tt.view.RelPath)
			}
			if attrs["data-freshness-identity"] != tt.view.ContentIdentity {
				t.Errorf("data-freshness-identity = %v, want %q", attrs["data-freshness-identity"], tt.view.ContentIdentity)
			}
		})
	}
}

// TestFreshnessAttrsStampTranscludedContentOnlyWhereItExists pins the third
// stamp's shape: a page that pulled words in from other notes stamps their
// identity beside its own, and a page that pulled in none carries no such
// attribute at all — absent, not empty, so its polling ask stays exactly as
// narrow as it was before the stamp existed.
func TestFreshnessAttrsStampTranscludedContentOnlyWhereItExists(t *testing.T) {
	t.Parallel()
	with := freshnessAttrs(NoteView{
		RelPath:             "Writing/note.md",
		ContentIdentity:     "3f2a",
		TranscludedIdentity: "0a1b",
	}, wording.ZhHant)
	if with["data-freshness-embeds"] != "0a1b" {
		t.Errorf("data-freshness-embeds = %v, want %q", with["data-freshness-embeds"], "0a1b")
	}
	without := freshnessAttrs(NoteView{RelPath: "Writing/note.md", ContentIdentity: "3f2a"}, wording.ZhHant)
	if got, ok := without["data-freshness-embeds"]; ok {
		t.Errorf("a page that transcluded nothing stamps data-freshness-embeds = %v; the attribute must be absent", got)
	}
}

// TestFreshnessAttrsCarryTheWriteHoldSentence pins the sentence the status
// faces show beside their disabled controls once the watch has learned the
// page is behind the file. The client reads it off the column like every other
// sentence of the watch, so a column without it would disable the controls
// and explain nothing.
func TestFreshnessAttrsCarryTheWriteHoldSentence(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			attrs := freshnessAttrs(NoteView{RelPath: "Writing/note.md", ContentIdentity: "3f2a"}, lang)
			want := wording.FreshnessWriteHold.In(lang)
			if want == "" {
				t.Fatal("the write-hold sentence is empty in this language, so the assertion below cannot mean anything")
			}
			if attrs["data-freshness-writehold"] != want {
				t.Errorf("data-freshness-writehold = %v, want %q", attrs["data-freshness-writehold"], want)
			}
		})
	}
}
