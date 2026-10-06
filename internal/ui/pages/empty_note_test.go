package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestEmptyArticleSpeaksTheInterfaceLanguage(t *testing.T) {
	t.Parallel()
	for _, language := range []struct {
		lang wording.Lang
		want string
	}{
		{lang: wording.ZhHant, want: "這篇筆記還沒有內容。"},
		{lang: wording.En, want: "This note has no content yet."},
	} {
		for _, body := range []string{"", " \t\n"} {
			t.Run(string(language.lang)+"/"+body, func(t *testing.T) {
				t.Parallel()
				var output bytes.Buffer
				view := NoteView{Title: "Empty", Language: "ja", BodyHTML: body}
				if err := Note(view, layouts.Chrome{Lang: language.lang}).Render(t.Context(), &output); err != nil {
					t.Fatalf("render note: %v", err)
				}
				want := `<p lang="` + language.lang.Tag() + `">` + language.want + `</p>`
				if !strings.Contains(output.String(), want) || strings.Count(output.String(), `data-nothing="note"`) != 1 {
					t.Errorf("caught: empty article lacks one native notice with interface-language paragraph %q", want)
				}
			})
		}
		t.Run("compare/"+string(language.lang), func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			view := CompareView{
				A: NoteView{Title: "Empty", Language: "ja"},
				B: NoteView{Title: "Body", BodyHTML: "<p>A body to read.</p>"},
			}
			if err := Compare(view, layouts.Chrome{Lang: language.lang}).Render(t.Context(), &output); err != nil {
				t.Fatalf("render compare: %v", err)
			}
			if strings.Count(output.String(), language.want) != 1 || !strings.Contains(output.String(), "<p>A body to read.</p>") {
				t.Errorf("caught: compare must name only its empty column and retain the other body")
			}
		})
	}
}
