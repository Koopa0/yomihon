package note_test

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// scopeVault holds a template carrying a placeholder citation and one ordinary
// note. Whether the template's placeholder is anyone's problem depends
// entirely on knowing which folders hold templates, which is what these tests
// vary.
func scopeVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, n := range []struct{ rel, body string }{
		{"System/templates/T1.md", "---\ntitle: T1\ntype: template\n---\n\n見 [[PLACEHOLDER-SLOT]]。\n"},
		{"Concepts/golang/Real.md", "---\ntitle: Real\ntype: concept\ndomain: golang\nstatus: draft\ncreated: 2026-06-01\nupdated: 2026-06-01\nbased_on: \"[[Real]]\"\n---\n\nbody\n"},
	} {
		full := filepath.Join(root, filepath.FromSlash(n.rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", n.rel, err)
		}
		if err := os.WriteFile(full, []byte(n.body), 0o600); err != nil {
			t.Fatalf("write %s: %v", n.rel, err)
		}
	}
	return root
}

// TestHealthSaysWhenItCouldNotEvaluate covers the three states the page can be
// in. The one that matters is the third: a contract that was written and could
// not be read leaves which files are instances unknown, and the lists built on
// that knowledge used to be produced anyway — reporting a template's
// placeholder as a citation the reader owes, which is a repair nobody can make.
func TestHealthSaysWhenItCouldNotEvaluate(t *testing.T) {
	t.Parallel()

	t.Run("a contract that reads, and a folder with findings", func(t *testing.T) {
		t.Parallel()
		srv := newServerWithContract(t, scopeVault(t), loadHomeContract(t))
		_, page := get(t, srv.URL+"/health")
		if strings.Contains(page, "PLACEHOLDER-SLOT") {
			t.Error("a template's placeholder is reported as a citation someone owes")
		}
		if strings.Contains(page, "無法評估") {
			t.Error("the page says it could not evaluate while holding a contract it read")
		}
	})

	t.Run("a contract that could not be read", func(t *testing.T) {
		t.Parallel()
		srv := newServerWithGovernance(t, scopeVault(t), nil, schema.Unreadable(errors.New("bad toml")))
		code, page := get(t, srv.URL+"/health")
		if code != http.StatusOK {
			t.Fatalf("health status = %d, want 200", code)
		}
		if strings.Contains(page, "PLACEHOLDER-SLOT") {
			t.Error("the placeholder is still reported, so the page is answering from a scope it does not know")
		}
		if !strings.Contains(page, "引用與孤島無法評估") {
			t.Error("the page does not say the citation lists could not be evaluated")
		}
		if strings.Contains(page, "yomihon check") {
			t.Error("the page says the folder has nothing to answer for while unable to evaluate it")
		}
	})

	// The doctrine this product holds elsewhere: an undeclared exclusion
	// excludes nothing. A folder that never wrote a contract is not a folder
	// in trouble, and its lists are answers rather than guesses — inventing a
	// rule its owner never wrote would be the larger error.
	t.Run("a folder that never declared anything", func(t *testing.T) {
		t.Parallel()
		srv := newServer(t, scopeVault(t))
		_, page := get(t, srv.URL+"/health")
		if !strings.Contains(page, "PLACEHOLDER-SLOT") {
			t.Error("the placeholder is withheld, so an undeclared exclusion was treated as one that failed")
		}
		if strings.Contains(page, "無法評估") {
			t.Error("a folder that declared nothing is told its lists could not be evaluated")
		}
	})
}

// TestHealthNamesTheClaimThatActuallyFailed holds the reason to the truth. A
// contract can read perfectly and still close governance — an artifacts
// section naming a folder outside the vault does exactly that — and a page
// that answered "the contract could not be read" would be false about a
// contract that was.
func TestHealthNamesTheClaimThatActuallyFailed(t *testing.T) {
	t.Parallel()

	contract := loadHomeContractWithArtifactSection(t, "[artifacts]\nnon_instance_dirs = [\"../outside\"]\n")
	srv := newServerWithContract(t, scopeVault(t), contract)

	_, page := get(t, srv.URL+"/health")
	if !strings.Contains(page, "non_instance_dirs") {
		t.Error("the page does not name what actually failed")
	}
	if strings.Contains(page, "無法讀取") {
		t.Error("the page says the contract could not be read, about a contract it read")
	}
}
