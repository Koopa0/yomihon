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
		body := "---\ntitle: " + name + "\ntype: writing\n" + flag + "---\n\nbody\n"
		if err := os.WriteFile(filepath.Join(root, name+".md"), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), "known = [", "known = [\"draft\", ", 1)
	text = strings.Replace(text, "[navigation]\n", "[navigation]\nanswer_type = \"writing\"\n", 1)
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
