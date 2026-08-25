package note_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fragmentSplitVault seeds the shape a reader cannot make sense of on their
// own: a file whose own name carries "#", and a note linking to that name.
// The link addresses a note called 井號 at a section called 筆記, so the file
// sitting in plain sight in the sidebar is not what it names, and the base
// note it does name was never written.
func fragmentSplitVault(t *testing.T) string {
	t.Helper()
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
	write("Concepts/井號#筆記.md", "---\ntitle: 井號#筆記\ntype: concept\ndomain: golang\nstatus: draft\n---\n\nbody\n")
	write("Concepts/入口.md", "---\ntitle: 入口\ntype: concept\ndomain: golang\nstatus: draft\n---\n\n見 [[井號#筆記]]，以及 [[根本沒寫的筆記]]。\n")
	return root
}

// noteConditions cuts the rail's 筆記狀況 block out of a rendered note page.
// The page renders the same block twice — once in the rail, once folded above
// the prose — so an assertion is made against one of them rather than against
// a count that two copies would satisfy on their own.
func noteConditions(t *testing.T, body string) string {
	t.Helper()
	const opens = `<div class="y-diaglist">`
	start := strings.Index(body, opens)
	if start < 0 {
		t.Fatalf("the note page carries no 筆記狀況 list; body = %q", body)
	}
	block, _, closed := strings.Cut(body[start:], `<p class="y-diag__note">`)
	if !closed {
		t.Fatalf("the 筆記狀況 list is not closed; body = %q", body)
	}
	return block
}

// diagRow cuts the one diagnostic naming target out of a 筆記狀況 list.
func diagRow(t *testing.T, conditions, target string) string {
	t.Helper()
	for _, chunk := range strings.Split(conditions, `<div class="y-diag">`)[1:] {
		row, _, terminated := strings.Cut(chunk, "</div></div>")
		if !terminated {
			row = chunk
		}
		if strings.Contains(row, "<code>"+target+"</code>") {
			return row
		}
	}
	t.Fatalf("no diagnostic names %q; conditions = %q", target, conditions)
	return ""
}

// The explanation of how the link was read lived only in the prose mark's
// title attribute and in text carried out of sight, so a reader who opened
// 筆記狀況 — the one place on the page that exists to say what is wrong with
// the file — was told a shorter name than the one they typed had failed, with
// nothing saying where the rest of it went, while the file carrying that exact
// name stood in the sidebar beside it.
func TestNoteConditionsStateHowAFragmentLinkWasRead(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, fragmentSplitVault(t), loadHomeContract(t))
	code, body := get(t, srv.URL+"/notes/Concepts/%E5%85%A5%E5%8F%A3.md")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	conditions := noteConditions(t, body)
	split := diagRow(t, conditions, "井號")

	// Both halves of the reading are named, each as the thing it is.
	for _, want := range []string{"筆記目標", "章節", "<code>井號</code>", "<code>筆記</code>"} {
		if !strings.Contains(split, want) {
			t.Errorf("the diagnostic does not show the link's reading (%q missing); row = %q", want, split)
		}
	}
	// And it says which half failed, so the reader is not left to conclude the
	// section was the problem.
	if !strings.Contains(split, "「#」") {
		t.Errorf("the diagnostic never names the mark that split the link; row = %q", split)
	}
	// Visible text, not a hover promise and not text carried out of sight.
	for _, forbidden := range []string{"title=", "y-offscreen"} {
		if strings.Contains(split, forbidden) {
			t.Errorf("the diagnostic hides its explanation behind %q; row = %q", forbidden, split)
		}
	}

	// The control: an ordinary unwritten link addressed no section, so it
	// gains none of this — otherwise the assertions above would pass on a
	// sentence the page prints for every broken link alike.
	plain := diagRow(t, conditions, "根本沒寫的筆記")
	for _, forbidden := range []string{"章節", "筆記目標"} {
		if strings.Contains(plain, forbidden) {
			t.Errorf("a link that addressed no section is explained as if it had; row = %q", plain)
		}
	}
}
