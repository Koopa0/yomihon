package note_test

import (
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestOpenThoughtsListsTheThoughtsStillDrafts pins what puts a reader's thought
// on the shelf of what they left open: the thought declares draft: true. One
// that writes the flag false, and one that leaves it out, are finished and stay
// off the shelf, and the lede says what the shelf holds.
func TestOpenThoughtsListsTheThoughtsStillDrafts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for name, flag := range map[string]string{
		"Unfinished": "draft: true\n",
		"Settled":    "draft: false\n",
		"Unflagged":  "",
	} {
		body := "---\ntype: writing\n" + flag + "---\n# " + name + "\n\nbody\n"
		if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := writingHoldsTheThoughtRole(t, string(data))
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(contractPath, []byte(text), 0o600); err != nil { // #nosec G703 -- a temporary file this test just named
		t.Fatal(err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	srv := newServerWithContract(t, root, contract)

	code, body := get(t, srv.Client(), srv.URL+"/open-thoughts")
	if code != http.StatusOK {
		t.Fatalf("GET /open-thoughts status = %d, want 200", code)
	}
	page := html.UnescapeString(body)
	if !strings.Contains(page, ">Unfinished<") {
		t.Errorf("the shelf does not list the thought that declares draft: true; page = %q", page)
	}
	for _, finished := range []string{">Settled<", ">Unflagged<"} {
		if strings.Contains(page, finished) {
			t.Errorf("the shelf lists a finished thought %s; page = %q", finished, page)
		}
	}
	if !strings.Contains(page, wording.OpenThoughtsLede.In(wording.ZhHant)) {
		t.Errorf("the shelf does not say what it holds; page = %q", page)
	}
}

// writingHoldsTheThoughtRole moves the shared contract's thought role from the
// thought type to writing, so a fixture's writing notes are the thoughts.
func writingHoldsTheThoughtRole(t *testing.T, contract string) string {
	t.Helper()
	const held = "[types.thought]\nrole = \"thought\"\n"
	const writing = "[types.writing]\n"
	if strings.Count(contract, held) != 1 || strings.Count(contract, writing) != 1 {
		t.Fatal("the contract fixture's thought role or writing type is not declared once")
	}
	contract = strings.Replace(contract, held, "[types.thought]\n", 1)
	return strings.Replace(contract, writing, writing+"role = \"thought\"\n", 1)
}

// TestOpenThoughtsAreOrderedAndDatedByCreation pins the date a thought is shelved
// by: the creation date it declares, newest first, shown beside it. A thought
// that declares none sorts after every dated one and shows no date, and an
// update key is no date at all.
func TestOpenThoughtsAreOrderedAndDatedByCreation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// The file names sort against the dates, so path order cannot pass.
	for file, note := range map[string]struct{ name, dates string }{
		"c.md": {"Newer", "created: 2026-03-07\n"},
		"b.md": {"Older", "created: 2026-01-05\n"},
		"a.md": {"Undated", "updated: 2026-12-31\n"},
	} {
		body := "---\ntype: writing\ndraft: true\n" + note.dates + "---\n# " + note.name + "\n\nbody\n"
		if err := os.WriteFile(filepath.Join(root, file), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(contractPath, []byte(writingHoldsTheThoughtRole(t, string(data))), 0o600); err != nil { // #nosec G703 -- a temporary file this test just named
		t.Fatal(err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	srv := newServerWithContract(t, root, contract)

	code, body := get(t, srv.Client(), srv.URL+"/open-thoughts")
	if code != http.StatusOK {
		t.Fatalf("GET /open-thoughts status = %d, want 200", code)
	}
	page := html.UnescapeString(body)
	last := -1
	for _, title := range []string{">Newer<", ">Older<", ">Undated<"} {
		at := strings.Index(page, title)
		if at < 0 || at < last {
			t.Fatalf("the shelf does not list %s after the thoughts created later (at %d, after %d); page = %q", title, at, last, page)
		}
		last = at
	}
	for _, date := range []string{"2026-03-07", "2026-01-05"} {
		if !strings.Contains(page, ">"+date+"<") {
			t.Errorf("the shelf does not show the creation date %s", date)
		}
	}
	if strings.Contains(page, "2026-12-31") {
		t.Error("the shelf dates a thought by an update key")
	}
}
