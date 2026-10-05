package note_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

func TestReadingNamesAnEmptyRenderedBody(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		content string
		empty   bool
	}{
		{name: "zero bytes", empty: true},
		{name: "whitespace", content: " \t\n\r\n", empty: true},
		{name: "frontmatter only", content: "---\ntitle: Empty\nlanguage: ja\n---\n", empty: true},
		{name: "frontmatter and whitespace", content: "---\ntitle: Empty\n---\n \t\n", empty: true},
		{name: "title consumed by article head", content: "# Empty\n", empty: true},
		{name: "body", content: "A body to read.\n"},
		{name: "image body", content: "![An illustration](asset.png)\n"},
	} {
		for _, language := range []struct {
			lang wording.Lang
			want string
		}{
			{lang: wording.ZhHant, want: "這篇筆記還沒有內容。"},
			{lang: wording.En, want: "This note has no content yet."},
		} {
			t.Run(test.name+"/"+string(language.lang), func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				if err := os.WriteFile(filepath.Join(root, "Empty.md"), []byte(test.content), 0o600); err != nil {
					t.Fatalf("write note: %v", err)
				}
				if err := os.WriteFile(filepath.Join(root, "asset.png"), []byte("\x89PNG\r\n\x1a\n"), 0o600); err != nil {
					t.Fatalf("write asset: %v", err)
				}
				server := newServer(t, root)
				request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/notes/Empty.md", http.NoBody)
				if err != nil {
					t.Fatalf("new request: %v", err)
				}
				request.Header.Set("Cookie", wording.CookieName+"="+string(language.lang))
				response, err := server.Client().Do(request)
				if err != nil {
					t.Fatalf("GET note: %v", err)
				}
				defer func() {
					if closeErr := response.Body.Close(); closeErr != nil {
						t.Errorf("close response: %v", closeErr)
					}
				}()
				body, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatalf("read response: %v", err)
				}
				if response.StatusCode != http.StatusOK {
					t.Fatalf("status = %d, want 200", response.StatusCode)
				}
				got := strings.Count(string(body), language.want)
				want := 0
				if test.empty {
					want = 1
				}
				if got != want {
					t.Errorf("caught: empty-body sentence count = %d, want %d for %q", got, want, test.content)
				}
			})
		}
	}
}
