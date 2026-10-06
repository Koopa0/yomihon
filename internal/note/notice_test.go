package note

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestFolderNoticesNamesBothFilesInEachLanguage pins the carry from the
// snapshot's colliding pair to what the pages draw. A surface that was handed a
// pair and drew nothing, or drew a notice without its files or without words in
// the reader's language, is the silence the notice exists to end.
func TestFolderNoticesNamesBothFilesInEachLanguage(t *testing.T) {
	t.Parallel()

	pair := []string{"Notes/が.md", "Notes/が.md"}
	var titles []string
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		got := folderNotices(pair, nil, lang)
		if len(got) != 1 {
			t.Fatalf("folderNotices returned %d notices for one pair in %s, want 1", len(got), lang)
		}
		notice := got[0]
		if strings.TrimSpace(notice.Title) == "" || strings.TrimSpace(notice.Summary) == "" {
			t.Errorf("the notice is %+v in %s, want a title and a sentence", notice, lang)
		}
		for _, raw := range pair {
			if !strings.Contains(notice.Detail, vault.Spelled(raw)) {
				t.Errorf("the notice's detail %q does not name %q in %s", notice.Detail, raw, lang)
			}
		}
		titles = append(titles, notice.Title)
	}
	if titles[0] == titles[1] {
		t.Errorf("the notice is titled %q in both languages, so one was copied across", titles[0])
	}
	if none := folderNotices(nil, nil, wording.En); len(none) != 0 {
		t.Errorf("folderNotices(nil) = %+v, want nothing to draw", none)
	}
}

func TestIndependentRootAndCollisionNoticesCoexist(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		lang wording.Lang
		want []string
	}{
		{lang: wording.En, want: []string{"Pages have stopped updating", "Reading the startup folder"}},
		{lang: wording.ZhHant, want: []string{"頁面已停止更新", "讀取的是啟動時的資料夾"}},
	} {
		var titles []string
		for _, notice := range folderNotices([]string{"Notes/が.md", "Notes/が.md"}, &snapshot.RootNotice{SelectedPath: "/selected", OpenedName: "/opened"}, tt.lang) {
			titles = append(titles, notice.Title)
		}
		if diff := cmp.Diff(tt.want, titles); diff != "" {
			t.Fatalf("independent folder notices (-want +got):\n%s", diff)
		}
	}
}
