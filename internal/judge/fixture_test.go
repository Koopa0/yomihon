package judge

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func judgeFixtureRoot(tb testing.TB, root string) string {
	tb.Helper()
	return judgeFixtureRootWithPrivacy(tb, root)
}

func judgeFixtureRootWithPrivacy(tb testing.TB, root string, privateDirs ...string) string {
	tb.Helper()
	if len(privateDirs) == 0 {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))); err == nil {
			return root
		}
	}
	copyRoot := tb.TempDir()
	if err := os.CopyFS(copyRoot, os.DirFS(root)); err != nil {
		tb.Fatalf("CopyFS(%q) error = %v", root, err)
	}
	if _, err := os.Stat(filepath.Join(copyRoot, filepath.FromSlash(schema.ContractRelPath))); err == nil {
		if len(privateDirs) == 0 {
			return copyRoot
		}
		tb.Fatalf("fixture %q already has a contract; custom privacy replacement is ambiguous", root)
	}
	writeTestContract(tb, copyRoot, privateDirs)
	return copyRoot
}

func testScanAuthority(tb testing.TB, privateDirs ...string) scanAuthority {
	tb.Helper()
	root := tb.TempDir()
	writeTestContract(tb, root, privateDirs)
	return loadTestAuthority(tb, root)
}

func loadTestAuthority(tb testing.TB, root string) scanAuthority {
	tb.Helper()
	reader, err := vault.Open(root)
	if err != nil {
		tb.Fatalf("vault.Open() error = %v", err)
	}
	tb.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			tb.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	authority, err := loadScanAuthority(tb.Context(), reader)
	if err != nil {
		tb.Fatalf("loadScanAuthority() error = %v", err)
	}
	return authority
}

func collectNotes(root string) ([]note, error) {
	a, err := openAction(root, actionHooks{})
	if err != nil {
		return nil, err
	}
	if err := a.finish(); err != nil {
		return nil, err
	}
	return a.notes, nil
}

func writeTestContract(tb testing.TB, root string, privateDirs []string) {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		tb.Fatalf("ReadFile(schema fixture) error = %v", err)
	}
	const knowledgeDirs = `knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`
	text := strings.Replace(string(data), knowledgeDirs, "knowledge_dirs = []", 1)
	if text == string(data) {
		tb.Fatalf("schema fixture does not contain %q", knowledgeDirs)
	}
	quoted := make([]string, len(privateDirs))
	for i, dir := range privateDirs {
		quoted[i] = strconv.Quote(dir)
	}
	text += "\n[privacy]\nnever_egress_dirs = [" + strings.Join(quoted, ", ") + "]\n"
	write(tb, root, schema.ContractRelPath, text)
}
