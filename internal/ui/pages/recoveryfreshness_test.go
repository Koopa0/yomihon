package pages

import (
	"testing"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestRecoveryFreshnessAttrsCarryTheHoldSentences pins the whole set of
// attributes the recovery column hands the client, in both languages. The
// client reads the hold sentences off the column exactly as the reading page's
// banner reads its own, so a column that names the note and the version but
// carries no words leaves the held link saying nothing a reader can use. The
// attribute names are the client's contract and the values are compared
// against the reading page's own column, so the two surfaces cannot come to
// say different things about the same wait.
func TestRecoveryFreshnessAttrsCarryTheHoldSentences(t *testing.T) {
	t.Parallel()
	view := StatusRecoveryView{NotePath: "Writing/note.md", NoteIdentity: "3f2a"}
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run(string(lang), func(t *testing.T) {
			t.Parallel()
			got := recoveryFreshnessAttrs(view, lang)
			want := templ.Attributes{
				"data-freshness-path":       "Writing/note.md",
				"data-freshness-identity":   "3f2a",
				"data-freshness-holdtitle":  wording.FreshnessHoldPreparingTitle.In(lang),
				"data-freshness-holddetail": wording.FreshnessHoldPreparingDetail.In(lang),
				"data-freshness-gonetitle":  wording.FreshnessHoldGoneTitle.In(lang),
				"data-freshness-gone":       wording.FreshnessGone.In(lang),
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("recoveryFreshnessAttrs() mismatch (-want +got):\n%s", diff)
			}
			for _, key := range []string{
				"data-freshness-holdtitle",
				"data-freshness-holddetail",
				"data-freshness-gonetitle",
				"data-freshness-gone",
			} {
				if got[key] == nil || got[key] == "" {
					t.Errorf("recoveryFreshnessAttrs()[%q] = %v, want a sentence the client can show", key, got[key])
					continue
				}
				noteColumn := freshnessAttrs(NoteView{RelPath: "Writing/note.md", ContentIdentity: "3f2a"}, lang)
				if got[key] != noteColumn[key] {
					t.Errorf("recoveryFreshnessAttrs()[%q] = %v, but the reading page's column says %v", key, got[key], noteColumn[key])
				}
			}
		})
	}
}

func TestRecoveryFreshnessAttrsNeedBothFacts(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		view  StatusRecoveryView
		watch bool
	}{
		{
			name:  "a refusal that knows the note and the version",
			view:  StatusRecoveryView{NotePath: "Writing/note.md", NoteIdentity: "3f2a"},
			watch: true,
		},
		{
			name:  "a refusal with no version to wait for",
			view:  StatusRecoveryView{NotePath: "Writing/note.md"},
			watch: false,
		},
		{
			name:  "a refusal that names no note",
			view:  StatusRecoveryView{NoteIdentity: "3f2a"},
			watch: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := len(recoveryFreshnessAttrs(tt.view, wording.ZhHant)) > 0; got != tt.watch {
				t.Errorf("recoveryFreshnessAttrs() watches = %t, want %t", got, tt.watch)
			}
		})
	}
}
