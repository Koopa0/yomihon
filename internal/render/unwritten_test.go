package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
)

// A link to a note nobody has written yet is styled with a help cursor, which
// tells the reader that pausing on it will explain something. Readers took that
// offer, got nothing, clicked, still got nothing, and concluded the page was
// broken. The explanation has to travel with the mark that promises it.
func TestUnwrittenLinkCarriesItsExplanation(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "運弓筆記.md"}}, nil, nil)

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "the name that failed is the one named, not the words shown",
			body: "見 [[節拍器練習法|那份練習法]]\n",
			want: `title="還沒有「節拍器練習法」這篇筆記"`,
		},
		{
			name: "an embed of a note nobody wrote says the same thing",
			body: "![[節拍器練習法]]\n",
			want: `title="還沒有「節拍器練習法」這篇筆記"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("練習日誌/2026-07-30.md", "2026-07-30", tt.body)
			if !strings.Contains(got.HTML, tt.want) {
				t.Errorf("HTML(%q) = %s\nwant it to contain %s", tt.body, got.HTML, tt.want)
			}
			// The help cursor is what makes the explanation owed, so a mark
			// carrying one and nothing else is the state this guards against.
			if strings.Contains(got.HTML, `<span class="wikilink-broken">`) {
				t.Errorf("HTML(%q) still renders a help-cursor mark with no explanation:\n%s", tt.body, got.HTML)
			}
		})
	}
}

// A target that does resolve can still fail to reach the page, and saying it
// was never written would be a different and false claim.
func TestResolvedButUnavailableEmbedSaysWhatActuallyHappened(t *testing.T) {
	t.Parallel()
	// The note is in the resolver but its body was not captured, which is what
	// a generation that read the name but not the bytes looks like.
	r := newRenderer(t, []graph.NoteInput{{Path: "運弓筆記.md"}}, nil, nil)

	got := r.HTML("a.md", "A", "![[運弓筆記]]\n")
	if strings.Contains(got.HTML, "還沒有") {
		t.Errorf("HTML() told the reader the note was never written, when it exists:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "拿不到它的內容") {
		t.Errorf("HTML() did not say what actually failed:\n%s", got.HTML)
	}
}
