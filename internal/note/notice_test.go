package note

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
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
		got := folderNotices(pair, nil, nil, lang)
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
	if none := folderNotices(nil, nil, nil, wording.En); len(none) != 0 {
		t.Errorf("folderNotices(nil) = %+v, want nothing to draw", none)
	}
}

// TestFolderNoticesSaysWhatDiffersAndWhichLineToChange holds the notice for a
// rules file written for another version to the two facts a reader needs: the
// version it declares against the one this reads, and the line to change.
func TestFolderNoticesSaysWhatDiffersAndWhichLineToChange(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		rules schema.VersionError
		lang  wording.Lang
		want  []string
	}{
		{
			name:  "a line to change, in English",
			rules: schema.VersionError{Declared: "2", Line: 3, Supported: "1"},
			lang:  wording.En,
			want:  []string{`schema_version = "2"`, `reads "1"`, "line 3", `schema_version = "1"`},
		},
		{
			name:  "a line to change, in Chinese",
			rules: schema.VersionError{Declared: "2", Line: 3, Supported: "1"},
			lang:  wording.ZhHant,
			want:  []string{`schema_version = "2"`, `讀 "1"`, "第 3 行", `schema_version = "1"`},
		},
		{
			name:  "no version declared",
			rules: schema.VersionError{Missing: true, Supported: "1"},
			lang:  wording.En,
			want:  []string{"no schema_version", `reads "1"`, `Add the line schema_version = "1"`},
		},
		{
			name:  "a declared value too long to quote whole",
			rules: schema.VersionError{Declared: strings.Repeat("x", 200), Line: 1, Supported: "1"},
			lang:  wording.En,
			want:  []string{strings.Repeat("x", 24) + "…"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := folderNotices(nil, nil, &tt.rules, tt.lang)
			if len(got) != 1 {
				t.Fatalf("folderNotices returned %d notices, want 1", len(got))
			}
			text := got[0].Title + "\n" + got[0].Summary + "\n" + got[0].Detail
			for _, want := range tt.want {
				if !strings.Contains(text, want) {
					t.Errorf("the notice reads %q, want it to carry %q", text, want)
				}
			}
			if strings.Contains(text, strings.Repeat("x", 25)) {
				t.Errorf("the notice quotes more of the declared value than it should: %q", text)
			}
		})
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
		for _, notice := range folderNotices([]string{"Notes/が.md", "Notes/が.md"}, &snapshot.RootNotice{SelectedPath: "/selected", OpenedName: "/opened"}, nil, tt.lang) {
			titles = append(titles, notice.Title)
		}
		if diff := cmp.Diff(tt.want, titles); diff != "" {
			t.Fatalf("independent folder notices (-want +got):\n%s", diff)
		}
	}
}
