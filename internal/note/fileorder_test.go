package note_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
)

func TestFolderRepliesUseReadingOrder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	paths := []string{"Letters/Untitled 2.md", "Letters/Zed.md", "Letters/Untitled.md", "Letters/index.md", "Letters/TODO.md", "Letters/第10課.md", "Letters/第2課.md", "Branches/Untitled 2/note.md", "Branches/Untitled/note.md", "Branches/TODO/note.md", "Branches/index/note.md"}
	for _, rel := range paths {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("MkdirAll(%q): %v", rel, err)
		}
		// A title deliberately unrelated to the filename keeps file order
		// distinct from sorting the reader's authored title.
		if err := os.WriteFile(full, []byte("---\ntitle: The same authored title\n---\n\nA reading.\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q): %v", rel, err)
		}
	}
	srv := newServer(t, root)
	cases := []struct {
		name, folder string
		want         []string
	}{
		{"notes", "Letters", []string{"/notes/Letters/index.md", "/notes/Letters/TODO.md", "/notes/Letters/Untitled.md", "/notes/Letters/Untitled 2.md", "/notes/Letters/Zed.md", "/notes/Letters/第2課.md", "/notes/Letters/第10課.md"}},
		{"folders", "Branches", []string{"/folders/Branches/index", "/folders/Branches/TODO", "/folders/Branches/Untitled", "/folders/Branches/Untitled 2"}},
	}
	for _, lang := range []string{"zh-Hant", "en"} {
		for _, tc := range cases {
			for replay := range 3 {
				t.Run(lang+"/"+tc.name+"/"+strconv.Itoa(replay), func(t *testing.T) {
					t.Parallel()
					t.Log("invoked: committed folder row order")
					req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/folders/"+tc.folder, http.NoBody)
					if err != nil {
						t.Fatal(err)
					}
					req.Header.Set("Cookie", "yomihon_lang="+lang)
					res, err := srv.Client().Do(req)
					if err != nil {
						t.Fatal(err)
					}
					body, readErr := io.ReadAll(res.Body)
					closeErr := res.Body.Close()
					if readErr != nil || closeErr != nil {
						t.Fatalf("read/close: %v / %v", readErr, closeErr)
					}
					if res.StatusCode != http.StatusOK {
						t.Fatalf("status=%d, want200: %s", res.StatusCode, body)
					}
					if !strings.Contains(res.Header.Get("Content-Type"), "text/html") {
						t.Fatalf("Content-Type=%q", res.Header.Get("Content-Type"))
					}
					var got []string
					rootLang := ""
					z := html.NewTokenizer(strings.NewReader(string(body)))
					for {
						kind := z.Next()
						if kind == html.ErrorToken {
							if !errors.Is(z.Err(), io.EOF) {
								t.Fatal(z.Err())
							}
							break
						}
						if kind != html.StartTagToken {
							continue
						}
						tok := z.Token()
						if tok.Data == "html" {
							for _, attr := range tok.Attr {
								if attr.Key == "lang" {
									rootLang = attr.Val
								}
							}
						}
						if tok.Data != "a" {
							continue
						}
						row, href := false, ""
						for _, attr := range tok.Attr {
							if attr.Key == "data-index-row" {
								row = true
							}
							if attr.Key == "href" {
								href = attr.Val
							}
						}
						if !row {
							continue
						}
						decoded, decodeErr := url.PathUnescape(href)
						if decodeErr != nil {
							t.Fatal(decodeErr)
						}
						got = append(got, decoded)
					}
					if rootLang != lang {
						t.Errorf("caught: committed html lang=%q, want %q", rootLang, lang)
					}
					if diff := cmp.Diff(tc.want, got); diff != "" {
						t.Errorf("caught: committed folder order (-want +got):\n%s", diff)
					}
				})
			}
		}
	}
	t.Run("cancelled folder rendering", func(t *testing.T) {
		t.Log("invoked: cancelled folder rendering")
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/folders/Letters", http.NoBody)
		rec := httptest.NewRecorder()
		srv.Config.Handler.ServeHTTP(rec, req)
		response := rec.Result()
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read/close cancelled reply: %v / %v", readErr, closeErr)
		}
		if len(body) != 0 {
			t.Errorf("caught: cancelled folder rendering wrote %d bytes, want none", len(body))
		}
	})
}
