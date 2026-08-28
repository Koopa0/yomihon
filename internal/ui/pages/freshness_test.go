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
