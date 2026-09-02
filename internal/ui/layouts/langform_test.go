package layouts

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestHeaderLanguageControlIsAPlainForm holds the language switch to the
// write-path shape every mutation in this interface uses: a plain form the
// server answers with a redirect, working before a line of script arrives. The
// old control was a button only script could hear, which locked a reader
// without JavaScript into whichever language the default happens to be — the
// one reader most in need of the other language is the one least able to ask
// for it. The form carries the language it switches to and the address to come
// back to, and its button keeps the mark naming the way out.
func TestHeaderLanguageControlIsAPlainForm(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		chrome   Chrome
		wantLang string
		wantMark string
	}{
		{
			name:     "default chrome offers English",
			chrome:   Chrome{ReturnTo: "/notes/Concepts/A.md"},
			wantLang: "en",
			wantMark: ">EN<",
		},
		{
			name:     "English chrome offers the way back",
			chrome:   Chrome{Lang: wording.En, ReturnTo: "/search?q=x"},
			wantLang: "zh-Hant",
			wantMark: ">中<",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := header(tt.chrome).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render header: %v", err)
			}
			html := buf.String()
			formAt := strings.Index(html, `action="/lang"`)
			if formAt < 0 {
				t.Fatalf("header has no form posting to /lang; html = %q", html)
			}
			start := strings.LastIndex(html[:formAt], "<form")
			end := strings.Index(html[formAt:], "</form>")
			if start < 0 || end < 0 {
				t.Fatalf("the /lang control is not a complete form; html = %q", html)
			}
			form := html[start : formAt+end]
			for _, want := range []string{
				`method="post"`,
				`name="lang" value="` + tt.wantLang + `"`,
				`name="next" value="` + tt.chrome.ReturnTo + `"`,
				`type="submit"`,
				tt.wantMark,
			} {
				if !strings.Contains(form, want) {
					t.Errorf("language form is missing %q; form = %q", want, form)
				}
			}
		})
	}
}
