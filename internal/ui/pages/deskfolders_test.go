package pages

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestDeskFolderBlockSkipsFoldersWithoutNotes(t *testing.T) {
	t.Parallel()
	root, _ := buildVault(t)
	for _, name := range []string{"C01.md", "C02.md"} {
		if err := os.Remove(filepath.Join(root, "Concepts", "go", name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "Concepts", "go", "scan.pdf"), []byte("attachment"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := modelOf(t, root)
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		page := NewFolderIndex(model, ContractGoverning, lang, nil)
		if page.Shelf.Rows[1].Text != "Concepts" {
			t.Fatalf("full folder page lost its zero-note folder: %+v", page.Shelf.Rows)
		}
		folderBlocks := 0
		for _, block := range NewDeskBlocks(model, schema.NavigationRoles{}, ContractGoverning, lang, nil) {
			if block.Mode != folderMode {
				continue
			}
			folderBlocks++
			shown := shelfRows(&block.Shelf, deskBlockItems)
			var names []string
			for _, row := range shown {
				names = append(names, row.Text)
			}
			if diff := cmp.Diff([]string{"Sources", "Maps", "Writing"}, names); diff != "" {
				t.Errorf("caught: desk folder choices in %s (-want +got):\n%s", lang, diff)
			}
			if block.Shelf.Count != page.Count {
				t.Errorf("desk count = %q, full shelf count = %q", block.Shelf.Count, page.Count)
			}
		}
		if folderBlocks != 1 {
			t.Errorf("caught: desk in %s offers %d folder blocks, want exactly one", lang, folderBlocks)
		}
	}
}

// The strip has room for three folders, so which three matters: the biggest,
// whatever order the vault holds them in. The full page keeps the vault's order
// and every folder.
func TestDeskFolderBlockLeadsWithTheBiggestFolders(t *testing.T) {
	t.Parallel()
	root, _ := buildVault(t)
	// Writing is the last folder in the vault's order and now holds the most.
	for _, name := range []string{"a", "b", "c", "d"} {
		path := filepath.Join(root, "Writing", name+".md")
		if err := os.WriteFile(path, []byte("body\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	model := modelOf(t, root)
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		page := NewFolderIndex(model, ContractGoverning, lang, nil)
		var inOrder []string
		for _, row := range page.Shelf.Rows {
			if row.Href != "" && row.Mark != "" {
				inOrder = append(inOrder, row.Text)
			}
		}
		if len(inOrder) < 4 || inOrder[0] == "Writing" {
			t.Fatalf("the full page lists %v in %s, want every folder in the vault's order, Writing not first", inOrder, lang)
		}
		for _, block := range NewDeskBlocks(model, schema.NavigationRoles{}, ContractGoverning, lang, nil) {
			if block.Mode != folderMode {
				continue
			}
			var names []string
			for _, row := range shelfRows(&block.Shelf, deskBlockItems) {
				names = append(names, row.Text)
			}
			if len(names) != deskBlockItems || names[0] != "Writing" {
				t.Errorf("caught: the desk strip in %s lists %v, want the biggest folder, Writing, first", lang, names)
			}
		}
	}
}

func TestFolderCountLeavesOutATermThatIsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		notes, folders int
		lang           wording.Lang
		want           string
	}{
		{3, 2, wording.ZhHant, "2 個資料夾 · 3 篇"},
		{0, 2, wording.ZhHant, "2 個資料夾"},
		{3, 0, wording.ZhHant, "3 篇"},
		{0, 0, wording.ZhHant, ""},
		{1, 1, wording.En, "1 folder · 1 note"},
		{0, 2, wording.En, "2 folders"},
	}
	for _, tt := range tests {
		if got := folderCount(tt.notes, tt.folders, tt.lang); got != tt.want {
			t.Errorf("folderCount(%d, %d) = %q, want %q", tt.notes, tt.folders, got, tt.want)
		}
	}
}
