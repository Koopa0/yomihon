package note_test

import (
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestNotePageNamesTheFieldAtFault is the claim this surface was built for:
// the page tells a reader what the command already knew, and names the field
// the fault is actually in. A note whose type is not one the schema declares
// used to be answered with a sentence about its status, which was legal —
// pointing a reader at the one field that was fine.
func TestNotePageNamesTheFieldAtFault(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		rel      string
		body     string
		wantSaid []string
		wantNot  []string
	}{
		{
			name:     "a type outside the schema's list is reported against type",
			rel:      "Concepts/golang/Memo.md",
			body:     "---\ntype: memorandum\ncreated: 2026-06-01\n---\n\nbody\n",
			wantSaid: []string{"<code>type</code>", "memorandum"},
		},
		{
			name:     "a declared type with a level outside its list still says so",
			rel:      "Writing/lessons/golang/Off.md",
			body:     "---\ntype: lesson\nslug: off\nlevel: nonesuch\ncreated: 2026-06-01\n---\n\nbody\n",
			wantSaid: []string{"<code>level</code>", "nonesuch"},
		},
		{
			name:     "a slug the schema's shape rejects is reported against slug",
			rel:      "Writing/lessons/golang/L01.md",
			body:     "---\ntype: lesson\nslug: Not Kebab\ncreated: 2026-06-01\n---\n\nbody\n",
			wantSaid: []string{"<code>slug</code>", "Not Kebab"},
		},
		{
			name:     "a frontmatter key the schema does not know is named",
			rel:      "Concepts/golang/Extra.md",
			body:     "---\ntype: concept\ncreated: 2026-06-01\nbased_on: \"[[x]]\"\nnot_a_field: 1\n---\n\nbody\n",
			wantSaid: []string{"<code>not_a_field</code>"},
		},
		{
			name:     "a missing required field is named",
			rel:      "Concepts/golang/Bare.md",
			body:     "---\ntype: concept\nbased_on: \"[[x]]\"\n---\n\nbody\n",
			wantSaid: []string{"<code>created</code>"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			full := filepath.Join(root, filepath.FromSlash(tc.rel))
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(full, []byte(tc.body), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			srv := newServerWithContract(t, root, loadHomeContract(t))

			code, page := get(t, srv.Client(), srv.URL+"/notes/"+tc.rel)
			if code != http.StatusOK {
				t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
			}
			for _, want := range tc.wantSaid {
				if !strings.Contains(page, want) {
					t.Errorf("the page never says %q; a reader is left with the silence this replaced", want)
				}
			}
			for _, unwanted := range tc.wantNot {
				if strings.Contains(page, unwanted) {
					t.Errorf("the page says %q, which points at a field that is not the one at fault", unwanted)
				}
			}
		})
	}
}

// TestNotePageEscapesTheNotesOwnTextInANotice covers what a notice is made of:
// the note's own words, quoted back to a reader. A note is a file anyone can
// write, so a value that looks like markup has to arrive as the characters the
// author typed rather than as anything the page acts on.
func TestNotePageEscapesTheNotesOwnTextInANotice(t *testing.T) {
	t.Parallel()

	const rel = "Writing/lessons/golang/L02.md"
	const hostile = `<b onclick="x">y</b>`
	body := "---\ntype: lesson\nslug: '" + hostile + "'\ncreated: 2026-06-01\n---\n\nbody\n"

	root := t.TempDir()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "&lt;b onclick=") {
		t.Errorf("the notice does not carry the author's characters escaped; page did not contain the escaped form")
	}
	if strings.Contains(page, hostile) {
		t.Errorf("the page carries %q verbatim, so a note's own text reached the reader as markup", hostile)
	}
}

// TestNotePageNamesWhatANonTextValueWrote holds the schema notice for an enum
// value the YAML reader takes as a number: the judge quotes the characters the
// author typed, and the page passes them on.
func TestNotePageNamesWhatANonTextValueWrote(t *testing.T) {
	t.Parallel()

	const rel = "Writing/lessons/golang/Numeric.md"
	body := "---\ntype: lesson\nslug: numeric\nlevel: 123\ncreated: 2026-06-01\n---\n\nbody\n"

	root := t.TempDir()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := newServerWithContract(t, root, loadHomeContract(t))

	code, page := get(t, srv.Client(), srv.URL+"/notes/"+rel)
	if code != http.StatusOK {
		t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
	}
	if !strings.Contains(page, "<code>123</code>") {
		t.Error("the page does not name what was written")
	}
}

// TestFrontmatterSchemaNoticesQuoteTheCapturedParser keeps parser evidence on
// the note's schema face, beside the explanation rather than only in the rail.
func TestFrontmatterSchemaNoticesQuoteTheCapturedParser(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, diagnostic string
	}{
		{"invalid mapping", "---\ntitle: Bad: yaml\n---\nReadable body.\n", "frontmatter is not valid YAML: yaml: line 2: mapping values are not allowed in this context"},
		{"duplicate key", "---\ntype: writing\ndup: one\ndup: two\n---\nReadable body.\n", "frontmatter is not valid YAML: yaml: unmarshal errors:\n  line 4: mapping key \"dup\" already defined at line 3"},
		{"HTML-sensitive duplicate key", "---\ntype: writing\n'<script>&</script>': one\n'<script>&</script>': two\n---\nReadable body.\n", "frontmatter is not valid YAML: yaml: unmarshal errors:\n  line 4: mapping key \"<script>&</script>\" already defined at line 3"},
	} {
		for _, chrome := range []struct {
			lang wording.Lang
			want string
		}{
			{wording.ZhHant, "frontmatter 不是有效的 YAML。解析器指出："},
			{wording.En, "The frontmatter is not valid YAML. The parser reported: "},
		} {
			t.Run(tc.name+"/"+string(chrome.lang), func(t *testing.T) {
				t.Parallel()
				root := writeNotes(t, map[string]string{"Writing/Bad.md": tc.body})
				server := newServerWithContract(t, root, loadContract(t))
				page := frontmatterNoticePage(t, server, chrome.lang)
				notice, ok := schemaNoticeMarkup(page)
				if !ok {
					t.Fatal("invalid YAML has no schema notice block")
				}
				for _, want := range []string{chrome.want, "<code>" + html.EscapeString(tc.diagnostic) + "</code>"} {
					if !strings.Contains(notice, want) {
						t.Errorf("schema notice omitted %q: %s", want, notice)
					}
				}
				for _, unwanted := range []string{"schema.frontmatter", "這個頁面還沒有它的說法", "this page has no words for it yet", "<script>"} {
					if strings.Contains(notice, unwanted) {
						t.Errorf("schema notice contains %q: %s", unwanted, notice)
					}
				}
			})
		}
	}
}

func TestFrontmatterSchemaNoticeDistinguishesMissingAndUnclosedBlocks(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		contract   func(*testing.T) *schema.Contract
		hasNotice  bool
	}{
		{"required block missing", "Readable body.\n", loadContractRequiringFrontmatter, true},
		{"optional block missing", "Readable body.\n", loadContract, false},
		{"fence never closes", "---\ntitle: Unclosed\nReadable body.\n", loadContractRequiringFrontmatter, false},
	} {
		for _, chrome := range []struct {
			lang    wording.Lang
			missing string
		}{
			{wording.ZhHant, "這篇筆記需要 frontmatter，但沒有 frontmatter 區塊。"},
			{wording.En, "This note requires frontmatter, but no frontmatter block is present."},
		} {
			t.Run(tc.name+"/"+string(chrome.lang), func(t *testing.T) {
				t.Parallel()
				root := writeNotes(t, map[string]string{"Writing/Bad.md": tc.body})
				server := newServerWithContract(t, root, tc.contract(t))
				page := frontmatterNoticePage(t, server, chrome.lang)
				notice, hasNotice := schemaNoticeMarkup(page)
				if hasNotice != tc.hasNotice {
					t.Fatalf("schema notice block present=%v, want %v", hasNotice, tc.hasNotice)
				}
				if hasNotice {
					if !strings.Contains(notice, chrome.missing) {
						t.Errorf("missing block notice lacks truthful explanation %q: %s", chrome.missing, notice)
					}
					for _, unwanted := range []string{"valid YAML", "有效的 YAML", "parser", "解析器", "<code>"} {
						if strings.Contains(notice, unwanted) {
							t.Errorf("missing block notice incorrectly contains %q: %s", unwanted, notice)
						}
					}
				}
				if tc.name == "fence never closes" && !strings.Contains(page, wording.FrontmatterNeverCloses.In(chrome.lang)) {
					t.Error("unclosed fence lost its existing explanation")
				}
			})
		}
	}
}

// schemaNoticeMarkup cuts the block of schema notices out of a reading page:
// a block of notice paragraphs other than the one that says a frontmatter
// fence never closes. ok is false where the page draws none.
func schemaNoticeMarkup(page string) (notice string, ok bool) {
	const opening = `<p class="y-statuspanel__notice y-statusflag">`
	for rest := page; ; {
		at := strings.Index(rest, opening)
		if at < 0 {
			return "", false
		}
		block, _, closed := strings.Cut(rest[at:], "</div>")
		if !closed {
			return "", false
		}
		unclosed := strings.Contains(block, wording.FrontmatterNeverCloses.In(wording.ZhHant)) ||
			strings.Contains(block, wording.FrontmatterNeverCloses.In(wording.En))
		if !unclosed {
			return block, true
		}
		rest = rest[at+len(block):]
	}
}

func frontmatterNoticePage(t *testing.T, server *httptest.Server, lang wording.Lang) string {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/notes/Writing/Bad.md", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", wording.CookieName+"="+string(lang))
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("close response: %v", closeErr)
		}
	}()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("frontmatter note HTTP %d, want 200", response.StatusCode)
	}
	page := string(raw)
	if !strings.Contains(page, "Readable body.") {
		t.Error("frontmatter explanation lost the readable body")
	}
	return page
}
