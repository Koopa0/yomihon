package note_test

import (
	"bytes"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// The two sentences are written out rather than read from the catalog, so a
// change of wording is a visible change to this test and not a silent one.
const (
	skippedNoteZh = "這份文件不在書庫的索引與診斷範圍內。"
	skippedNoteEn = "This document is outside the library's index and diagnostics."
)

// skippedReadme is the file the contract leaves out. Its frontmatter carries a
// sentinel the reading must not show, and it has a heading, a list, a relative
// link, a link leaving the vault, and a picture.
const skippedReadme = "---\nreviewer: frontmatter-sentinel\n---\n" +
	"# Maintainer notes\n\n" +
	"- first item\n" +
	"- second item\n\n" +
	"See [the guide](guide.md), or leave through [the parent](../../out.md).\n\n" +
	"![diagram](diagram.png)\n"

// skippedDocumentVault holds one README the contract leaves out of the library
// beside the note it links to. The README lives below the root so that a link
// resolved against the vault root, rather than against the file, lands
// somewhere else and the test notices.
func skippedDocumentVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "Writing"))
	mkdir(t, filepath.Join(root, "Elsewhere"))
	write(t, filepath.Join(root, "Writing", "README.md"), []byte(skippedReadme))
	write(t, filepath.Join(root, "Writing", "diagram.png"), []byte("\x89PNG\r\n\x1a\n fake pixels"))
	write(t, filepath.Join(root, "Writing", "guide.md"), []byte(
		"---\ntitle: The Guide\ntype: writing\nstatus: draft\n---\n# Guide heading\n\nA note that links [the parent](../../out.md).\n"))
	write(t, filepath.Join(root, "Writing", "plain.txt"), []byte("plain text\n"))
	write(t, filepath.Join(root, "Elsewhere", "README.md"), bytes.Repeat([]byte("x"), bigTextBytes))
	return root
}

func pageIn(t *testing.T, srv *httptest.Server, target string, lang wording.Lang) (code int, body string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+target, http.NoBody)
	if err != nil {
		t.Fatalf("new request %s: %v", target, err)
	}
	req.Header.Set("Cookie", wording.CookieName+"="+string(lang))
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body %s: %v", target, err)
	}
	return resp.StatusCode, buf.String()
}

// TestSkippedMarkdownOpensAsADocument covers a Markdown file the contract
// leaves out of the library. It is not a note, and it is not unreadable either:
// opening it reads as a document, with none of the note's furniture, one quiet
// line saying where it stands, and the way to its bytes.
func TestSkippedMarkdownOpensAsADocument(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, skippedDocumentVault(t), loadContract(t))
	const target = "/notes/Writing/README.md"

	for _, tt := range []struct {
		lang wording.Lang
		want string
	}{
		{lang: wording.ZhHant, want: skippedNoteZh},
		{lang: wording.En, want: skippedNoteEn},
	} {
		t.Run(string(tt.lang), func(t *testing.T) {
			t.Parallel()
			code, body := pageIn(t, srv, target, tt.lang)
			if code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", target, code)
			}

			if !strings.Contains(body, html.EscapeString(tt.want)) {
				t.Errorf("the page does not carry the quiet line %q", tt.want)
			}
			if !strings.Contains(body, `href="/raw/Writing/README.md"`) {
				t.Error("the page carries no link to the raw source")
			}

			// Read as a document: the heading and the list are elements, and the
			// page is not highlighted source.
			if !strings.Contains(body, `data-level="1">Maintainer notes</h2>`) {
				t.Errorf("the heading is not rendered as a heading:\n%s", body)
			}
			for _, item := range []string{"<li>first item</li>", "<li>second item</li>"} {
				if !strings.Contains(body, item) {
					t.Errorf("the list is not rendered: missing %s", item)
				}
			}
			if strings.Contains(body, `class="chroma"`) {
				t.Error("the file is still drawn as highlighted source")
			}
			if strings.Contains(body, "frontmatter-sentinel") {
				t.Error("the frontmatter block is read as part of the document")
			}

			// None of a note's furniture: no status face, no aids rail, no
			// contents.
			for _, absent := range []string{`action="/status"`, "y-sealbar", "y-statusform", "y-rail-right", "y-toc"} {
				if strings.Contains(body, absent) {
					t.Errorf("the document carries note furniture: found %q", absent)
				}
			}
		})
	}

	// The link is resolved from the file's own folder, the way a browser
	// resolves it from the page's address. A README at the root could not tell
	// that from a link resolved against the vault root; this one cannot.
	_, body := pageIn(t, srv, target, wording.En)
	if !strings.Contains(body, `<a href="guide.md">the guide</a>`) {
		t.Fatalf("the relative link is not written as authored:\n%s", body)
	}
	page, err := url.Parse(srv.URL + target)
	if err != nil {
		t.Fatalf("parse page address: %v", err)
	}
	dest, err := page.Parse("guide.md")
	if err != nil {
		t.Fatalf("resolve the link against the page: %v", err)
	}
	if dest.Path != "/notes/Writing/guide.md" {
		t.Fatalf("the link resolves to %s, want /notes/Writing/guide.md", dest.Path)
	}
	code, guide := pageIn(t, srv, dest.Path, wording.En)
	if code != http.StatusOK || !strings.Contains(guide, "The Guide") {
		t.Errorf("GET %s = %d; the link does not reach the note it names", dest.Path, code)
	}

	// A picture is resolved on the server, against the file's own folder.
	if !strings.Contains(body, `src="/raw/Writing/diagram.png"`) || strings.Contains(body, "image-missing") {
		t.Errorf("the picture is not resolved from the file's own folder:\n%s", body)
	}

	// A link that leaves the vault is written exactly as a note writes it: the
	// renderer marks nothing for an ordinary Markdown link, so the document
	// adds nothing a note would not.
	const leaving = `<a href="../../out.md">the parent</a>`
	if !strings.Contains(body, leaving) {
		t.Errorf("the document does not write a link leaving the vault as a note does; want %s", leaving)
	}
	_, note := pageIn(t, srv, "/notes/Writing/guide.md", wording.En)
	if !strings.Contains(note, leaving) {
		t.Errorf("control: the note does not write that link as %s", leaving)
	}
}

// TestSkippedMarkdownChangesNothingElse holds the boundary of the page above:
// a note keeps its status face, a file that is not Markdown keeps being shown as
// source, a skipped file too large to read comfortably keeps its information
// page, and the raw bytes are untouched.
func TestSkippedMarkdownChangesNothingElse(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, skippedDocumentVault(t), loadContract(t))

	_, noteBody := pageIn(t, srv, "/notes/Writing/guide.md", wording.En)
	for _, want := range []string{"y-sealbar", "y-statusform", `action="/status"`, "y-rail-right", "y-toc"} {
		if !strings.Contains(noteBody, want) {
			t.Errorf("the note page lost %q", want)
		}
	}
	if strings.Contains(noteBody, skippedNoteEn) {
		t.Error("a note is told it is outside the library")
	}

	_, source := pageIn(t, srv, "/notes/Writing/plain.txt", wording.En)
	if !strings.Contains(source, `class="chroma"`) || strings.Contains(source, skippedNoteEn) {
		t.Error("a file that is not Markdown is no longer shown as source")
	}

	_, big := pageIn(t, srv, "/notes/Elsewhere/README.md", wording.En)
	if !strings.Contains(big, "y-fileinfo") || strings.Contains(big, skippedNoteEn) {
		t.Error("a skipped file over the size bound no longer gets its information page")
	}

	code, header, raw := fetch(t, srv.Client(), srv.URL+"/raw/Writing/README.md")
	if code != http.StatusOK || raw != skippedReadme {
		t.Errorf("GET /raw/Writing/README.md = %d with %d bytes, want 200 with the file's %d bytes", code, len(raw), len(skippedReadme))
	}
	if got := header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("raw response X-Content-Type-Options = %q, want nosniff", got)
	}
}
