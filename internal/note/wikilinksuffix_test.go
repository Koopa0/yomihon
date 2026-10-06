package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestRegisteredReadingPathSuffix(t *testing.T) {
	t.Parallel()
	root := writeNotes(t, map[string]string{
		"Concepts/Atlas/Page.md":           "---\ntitle: Atlas page\ntype: concept\n---\n## Part\n\nSection payload.\n\nBlock payload. ^piece\n",
		"Concepts/A/Shared/Twin.md":        "A twin.\n",
		"Concepts/B/Shared/Twin.md":        "B twin.\n",
		"Concepts/Thought/Source.md":       "---\ntitle: Thought source\n---\n## Part\nbody\n",
		"Concepts/Source.md":               "---\ntitle: Source\ntype: concept\nbased_on: '[[Atlas/Page#Part]]'\n---\n[[Atlas/Page|Chosen]]\n\n[[Shared/Twin]]\n\n[[Nope/Page]]\n\n![[Atlas/Page#^piece]]\n",
		"Extra/Concepts/Thought/Source.md": "---\ntitle: Deeper candidate\n---\nDo not choose me.\n",
	})
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), "[navigation]\n", "[navigation]\nanswer_type = \"writing\"\n", 1)
	file := filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))
	if mkdirErr := os.MkdirAll(filepath.Dir(file), 0o750); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(file, []byte(text), 0o600); writeErr != nil { // #nosec G703 -- writeNotes returns a testing.TempDir root, and the schema path is constant
		t.Fatal(writeErr)
	}
	contract, err := schema.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	srv := newServerWithContract(t, root, contract)
	code, body := get(t, srv.Client(), srv.URL+"/notes/Concepts/Source.md")
	t.Log("invoked: actual suffix resolution contract")
	if code != http.StatusOK {
		t.Fatalf("GET source status = %d", code)
	}
	for _, want := range []string{`href="/notes/Concepts/Atlas/Page.md" class="wikilink">Chosen</a>`, `class="wikilink-ambiguous"`, `class="wikilink-broken"`, `href="/notes/Concepts/Atlas/Page.md#part"`, "Block payload."} {
		if !strings.Contains(body, want) {
			t.Errorf("caught: registered suffix reading missing: %q", want)
		}
	}
	code, body = get(t, srv.Client(), srv.URL+"/notes/Concepts/Atlas/Page.md")
	if code != http.StatusOK || !strings.Contains(body, "Section payload.") {
		t.Errorf("caught: canonical suffix href failed: (%d,%q)", code, body)
	}
	// The thought door emits the full captured identity even when a deeper
	// suffix candidate exists, so its generated citation still names this note.
	code, body = get(t, srv.Client(), srv.URL+"/thought/Concepts/Thought/Source.md?section=part")
	if code != http.StatusOK || !strings.Contains(thoughtTextarea(t, body), `based_on: "[[Concepts/Thought/Source#Part]]"`) {
		t.Errorf("caught: exact thought source changed: (%d,%q)", code, body)
	}
	code, _ = get(t, srv.Client(), srv.URL+"/notes/Nope/Page.md")
	if code != http.StatusNotFound {
		t.Errorf("missing GET status = %d", code)
	}
}
