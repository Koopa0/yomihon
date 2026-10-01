package note

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEveryNoticeReasonIsWordedInBothLanguages holds the set of reasons the
// snapshot can give to the words the pages have for them. The set is the
// snapshot's own list, so a reason added there and left unworded here fails
// this test rather than drawing a notice with no title; and each language has
// to say it in its own words, since a title identical in both is the one that
// was copied across.
func TestEveryNoticeReasonIsWordedInBothLanguages(t *testing.T) {
	t.Parallel()

	reasons := snapshot.NoticeReasons()
	if len(reasons) == 0 {
		t.Fatal("the snapshot lists no notice reason, so nothing below was checked")
	}
	for _, reason := range reasons {
		var titles []string
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			title, summary := noticeWords(reason, lang)
			if strings.TrimSpace(title) == "" || strings.TrimSpace(summary) == "" {
				t.Errorf("reason %d has title %q and summary %q in %s, so its notice would draw an empty heading or sentence", reason, title, summary, lang)
			}
			titles = append(titles, title)
		}
		if titles[0] == titles[1] {
			t.Errorf("reason %d is titled %q in both languages", reason, titles[0])
		}
	}
}

// TestFolderNoticesDropsNothingAndNamesEveryFile pins the carry from the
// snapshot's notices to the page's. A surface that was handed a notice and drew
// fewer than it was given, or drew one without its files, is the silence the
// notice exists to end.
func TestFolderNoticesDropsNothingAndNamesEveryFile(t *testing.T) {
	t.Parallel()

	paths := []string{"Notes/\u304b\u3099.md", "Notes/\u304c.md"}
	given := []snapshot.Notice{
		{Reason: snapshot.NoticeNamesCollide, Paths: paths},
		{Reason: snapshot.NoticeNamesCollide, Paths: []string{"Elsewhere/a.md"}},
	}
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		got := folderNotices(given, lang)
		if len(got) != len(given) {
			t.Fatalf("folderNotices returned %d notices from %d in %s", len(got), len(given), lang)
		}
		for _, raw := range paths {
			if !strings.Contains(got[0].Detail, vault.Spelled(raw)) {
				t.Errorf("the notice's detail %q does not name %q in %s", got[0].Detail, raw, lang)
			}
		}
		if got[0].Title == "" || got[0].Summary == "" {
			t.Errorf("the notice is %+v in %s, want a title and a sentence", got[0], lang)
		}
	}
	if none := folderNotices(nil, wording.En); len(none) != 0 {
		t.Errorf("folderNotices(nil) = %+v, want nothing to draw", none)
	}
}
