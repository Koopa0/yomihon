package lexical

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

const testdataKnowledgeDirs = `knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`

// TestAKnowledgeLayerNoteRanksFirstInItsEvidenceGroup is the flatten-time
// knowledge rank the ruling named: the same word in a Notes/ note and a
// System/ file, plus a vault-root file because Includes cuts at the first
// slash and README.md is skip_basenames in the example vault. Notes/ sorts
// before System/ under ComparePaths, so the root file is what path order
// cannot already pass. Membership is Includes, not a copied directory list.
func TestAKnowledgeLayerNoteRanksFirstInItsEvidenceGroup(t *testing.T) {
	t.Parallel()

	root := "AAA.md"
	note := "Notes/late.md"
	system := "System/early.md"
	if vault.ComparePaths(root, note) >= 0 || vault.ComparePaths(note, system) >= 0 {
		t.Fatal("the root file must sort first by path, then Notes/, then System/, or this fixture cannot catch a missing demotion")
	}

	scope := knowledgeScopeDeclaring(t, `knowledge_dirs = ["Notes"]`)
	if !scope.Available() {
		t.Fatal("the Notes/ fixture must declare a knowledge layer")
	}
	if scope.Includes(root) {
		t.Fatal("a vault-root file must sit outside a declared layer; Includes cuts at the first slash")
	}
	if !scope.Includes(note) {
		t.Fatal("Notes/ must be inside the declared layer")
	}
	if scope.Includes(system) {
		t.Fatal("System/ must sit outside the declared layer")
	}

	idx := indexMarked(t, knowledgeRankDocs(root, note, system), scope)
	got := paths(searchResults(t, idx, Parse("needle")))
	want := []string{note, root, system}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(needle) order mismatch (-want +got):\n%s", diff)
	}
}

// TestAnUndeclaredKnowledgeLayerLeavesSearchOrderUnchanged is the empty-set
// polarity the fix inherits from knowledge.go: a contract that declares no
// knowledge_dirs marks nothing, so the same three documents keep the vault's
// reading order.
func TestAnUndeclaredKnowledgeLayerLeavesSearchOrderUnchanged(t *testing.T) {
	t.Parallel()

	root := "AAA.md"
	note := "Notes/late.md"
	system := "System/early.md"
	if vault.ComparePaths(root, note) >= 0 || vault.ComparePaths(note, system) >= 0 {
		t.Fatal("the root file must sort first by path, then Notes/, then System/, or this fixture cannot catch a silent reorder")
	}

	scope := knowledgeScopeDeclaring(t, "")
	if scope.Available() {
		t.Fatal("the fixture without knowledge_dirs still declares a scope, so a test built on it would prove nothing")
	}
	for _, p := range []string{root, note, system} {
		if !scope.Includes(p) {
			t.Fatalf("Includes(%q) = false, want true when no layer is declared", p)
		}
	}

	idx := indexMarked(t, knowledgeRankDocs(root, note, system), scope)
	got := paths(searchResults(t, idx, Parse("needle")))
	want := []string{root, note, system}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search(needle) order mismatch (-want +got):\n%s", diff)
	}
}

func knowledgeRankDocs(root, note, system string) []Document {
	return []Document{
		{RelPath: root, Title: "Unrelated", PlainText: "needle at the root"},
		{RelPath: note, Title: "Unrelated", PlainText: "needle in a note"},
		{RelPath: system, Title: "Unrelated", PlainText: "needle in a template"},
	}
}

func indexMarked(tb testing.TB, docs []Document, scope schema.KnowledgeScope) *Index {
	tb.Helper()
	marked := append([]Document(nil), docs...)
	for i := range marked {
		marked[i].OutsideKnowledge = !scope.Includes(marked[i].RelPath)
	}
	return NewIndex(marked, validArtifactPolicy(tb))
}

func knowledgeScopeDeclaring(tb testing.TB, knowledgeDirsLine string) schema.KnowledgeScope {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		tb.Fatalf("read contract fixture: %v", err)
	}
	text := string(data)
	if knowledgeDirsLine == "" {
		text = strings.Replace(text, testdataKnowledgeDirs+"\n", "", 1)
	} else {
		text = strings.Replace(text, testdataKnowledgeDirs, knowledgeDirsLine, 1)
	}
	if text == string(data) {
		tb.Fatal("fixture drift: the schema testdata contract no longer declares the knowledge_dirs needle")
	}
	path := filepath.Join(tb.TempDir(), "vault-schema.toml")
	if writeErr := os.WriteFile(path, []byte(text), 0o600); writeErr != nil {
		tb.Fatalf("os.WriteFile: %v", writeErr)
	}
	loaded, err := schema.LoadFile(path)
	if err != nil {
		tb.Fatalf("schema.LoadFile: %v", err)
	}
	return loaded.KnowledgeScope()
}
