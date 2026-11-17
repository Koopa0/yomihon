package note_test

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// A folder says what is not for reading in its .gitignore, and some folders are
// never for reading at all: installed packages, a Python environment. Nothing
// from them is listed or served, and a note that links into them keeps the
// words of the link without an anchor that could only lead to a not-found page.
// The folder is read as any folder is, with no contract.
func TestReaderLeavesOutWhatTheVaultIgnoresAndDoesNotLinkToIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for rel, body := range map[string]string{
		".gitignore":                 "drafts/\n",
		"Host.md":                    "# Host\n\n[plan](drafts/plan.md) [pkg](node_modules/pkg/README.md) [env](env/lib/README.md) [kept](Kept.md)\n\n![figure](drafts/figure.png)\n",
		"Kept.md":                    "# Kept\n",
		"drafts/plan.md":             "# Plan\n",
		"drafts/figure.png":          "png",
		"node_modules/pkg/README.md": "# Installed\n",
		"env/pyvenv.cfg":             "home = /usr/bin\n",
		"env/lib/README.md":          "# A virtual environment's own document\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("MkdirAll(%q): %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("WriteFile(%q): %v", rel, err)
		}
	}
	srv := newServer(t, root)

	code, page := get(t, srv.Client(), srv.URL+"/notes/Host.md")
	if code != http.StatusOK {
		t.Fatalf("GET /notes/Host.md status = %d, want 200", code)
	}
	hrefs := pageHrefs(t, page)
	for _, href := range hrefs {
		for _, left := range []string{"drafts", "node_modules", "env"} {
			if strings.Contains(href, left) {
				t.Errorf("the page links to %q, which the scan left out", href)
			}
		}
	}
	if want := `<p>plan pkg env <a href="/notes/Kept.md">kept</a></p>`; !strings.Contains(page, want) {
		t.Errorf("the words of the links into left-out ground did not stay as plain text; want %q in:\n%s", want, page)
	}

	for _, path := range []string{
		"/notes/drafts/plan.md", "/raw/drafts/figure.png", "/raw/node_modules/pkg/README.md",
		"/notes/node_modules/pkg/README.md", "/notes/env/lib/README.md", "/folders/drafts", "/folders/env/lib",
	} {
		if code, _ := get(t, srv.Client(), srv.URL+path); code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, code)
		}
	}
	if code, _ := get(t, srv.Client(), srv.URL+"/notes/Kept.md"); code != http.StatusOK {
		t.Errorf("GET /notes/Kept.md status = %d, want 200", code)
	}
}

func pageHrefs(t *testing.T, page string) []string {
	t.Helper()
	var hrefs []string
	z := html.NewTokenizer(strings.NewReader(page))
	for {
		kind := z.Next()
		if kind == html.ErrorToken {
			if !errors.Is(z.Err(), io.EOF) {
				t.Fatalf("tokenize page: %v", z.Err())
			}
			return hrefs
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		tok := z.Token()
		if tok.Data != "a" {
			continue
		}
		for _, attr := range tok.Attr {
			if attr.Key == "href" {
				hrefs = append(hrefs, attr.Val)
			}
		}
	}
}
