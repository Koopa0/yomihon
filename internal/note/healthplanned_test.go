package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestHealthOmitsNameUnderDeclaredNonDefaultGapMark is the /health face of
// the harvest lock: a contract that writes planned_gap_marks = ["Gaps"]
// (not a loader default) must keep a name listed under that heading off the
// unwritten list. Nowhere stays on the page so the section is evaluating.
func TestHealthOmitsNameUnderDeclaredNonDefaultGapMark(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("Maps/ledger.md", "---\ntitle: Ledger\ntype: topic-map\ndomain: golang\n---\n\n## Gaps\n- Ghost\n")
	write("Concepts/citer.md", "---\ntitle: Citer\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nSee [[Ghost]] and [[Nowhere]].\n")

	srv := newServerWithContract(t, root, loadHomeContractWithRules(t,
		"planned_gap_marks = [\"Gaps\"]\nplanned_inline_marks = []"))
	code, page := get(t, srv.Client(), srv.URL+"/health")
	if code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200", code)
	}
	if !strings.Contains(page, "Nowhere") {
		t.Fatalf("Nowhere must appear as unwritten on /health so the page is evaluating; page = %q", page)
	}
	unwritten := healthSectionBody(t, page, "連到不存在的目標")
	if strings.Contains(unwritten, "Ghost") {
		t.Fatalf("Ghost under declared Gaps is listed as unwritten; the harvest ignored the contract; section = %q", unwritten)
	}
}

func loadHomeContractWithRules(t *testing.T, extraRules string) *schema.Contract {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema test contract: %v", err)
	}
	const needle = "forbid_tag_with_slash = true"
	text := strings.Replace(string(base), needle, needle+"\n"+extraRules, 1)
	if text == string(base) {
		t.Fatal("the contract fixture does not contain the rules needle")
	}
	path := filepath.Join(t.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(text), 0o600); writeErr != nil { // #nosec G703 -- path is a fixed basename under this test's TempDir
		t.Fatalf("write contract: %v", writeErr)
	}
	contract, err := schema.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%q) = %v", path, err)
	}
	return contract
}
