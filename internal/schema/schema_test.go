package schema_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/kurodo/internal/schema"
)

// The testdata contract is a loader fixture, not a second schema: runtime
// code only ever reads the real vault contract.
func loadFixture(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.LoadFile(filepath.Join("testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("LoadFile(testdata/contract.toml) = %v", err)
	}
	return s
}

// TestStatusValuesAreNeverHardcodedOutsideSchema guards the single-source rule
// for the status state machine. The legal status values are defined once, in
// the vault contract that this package alone reads; no other package under
// internal/ may pin one of those values as a string literal in its logic.
// Naming the one distinguished seal value as a const is allowed — that is a
// single named reference, not a second copy of the value set — so only literals
// used directly in expressions are flagged. The judge package is exempt: it
// reproduces, byte for byte, a frozen external contract whose rule constants
// happen to be status values, pinned by its golden files rather than derived
// from this contract. The forbidden set is loaded from the fixture contract, so
// the test behaves the same on every machine, with or without the real vault.
func TestStatusValuesAreNeverHardcodedOutsideSchema(t *testing.T) {
	t.Parallel()
	s := loadFixture(t)
	forbidden := map[string]bool{}
	for _, group := range s.Enums.Status {
		for _, value := range group {
			forbidden[value] = true
		}
	}

	const root = ".." // this test runs in internal/schema, so .. is internal/
	fset := token.NewFileSet()
	var violations []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// schema owns the contract; judge reproduces a frozen external one
			// whose rule constants happen to be status values.
			if path == filepath.Join(root, "schema") || path == filepath.Join(root, "judge") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", path, perr)
		}
		if ast.IsGenerated(f) {
			return nil
		}
		// A status value on the right of a const declaration names the value in
		// one place; that is the allowed form, so collect those positions and
		// flag every other literal.
		named := map[token.Pos]bool{}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, value := range vs.Values {
					if lit, ok := value.(*ast.BasicLit); ok {
						named[lit.ValuePos] = true
					}
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || named[lit.ValuePos] {
				return true
			}
			value, uerr := strconv.Unquote(lit.Value)
			if uerr != nil || !forbidden[value] {
				return true
			}
			violations = append(violations, fmt.Sprintf("%s: %q", fset.Position(lit.ValuePos), value))
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk internal/: %v", walkErr)
	}
	if len(violations) > 0 {
		t.Errorf("a status value is defined only in the vault contract, but these string literals hardcode one outside internal/schema — name it as a const or move the decision behind this package:\n%s",
			strings.Join(violations, "\n"))
	}
}

func TestStatusGroup(t *testing.T) {
	t.Parallel()
	s := loadFixture(t)

	tests := []struct {
		noteType string
		want     string
	}{
		{"lesson", "lesson"},
		{"system", "system"},
		{"guide", "system"},
		{"concept", "note"},
		{"writing", "note"},
	}
	for _, tt := range tests {
		t.Run(tt.noteType, func(t *testing.T) {
			t.Parallel()
			if got := s.StatusGroup(tt.noteType); got != tt.want {
				t.Errorf("StatusGroup(%q) = %q, want %q", tt.noteType, got, tt.want)
			}
		})
	}
}

func TestStatuses(t *testing.T) {
	t.Parallel()
	s := loadFixture(t)

	got := s.Statuses("lesson")
	want := []string{"imported", "draft", "ready", "archived"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Statuses(\"lesson\") mismatch (-want +got):\n%s", diff)
	}
}

func TestTransition(t *testing.T) {
	t.Parallel()
	s := loadFixture(t)

	tests := []struct {
		name    string
		typ     string
		from    string
		to      string
		actor   string
		wantErr error
	}{
		{"lesson imported→draft by claude", "lesson", "imported", "draft", "claude", nil},
		{"lesson draft→ready by koopa", "lesson", "draft", "ready", "koopa", nil},
		{"ready is koopa-only", "lesson", "draft", "ready", "claude", schema.ErrOwnerForbidden},
		{"skip a stage", "lesson", "imported", "ready", "koopa", schema.ErrIllegalTransition},
		{"archived from anywhere", "concept", "seedling", "archived", "koopa", nil},
		{"initial captured", "transcript", "", "captured", "hermes", nil},
		{"non-initial needs predecessor", "source-note", "", "cleaned", "claude", schema.ErrIllegalTransition},
		{"undefined status for type", "concept", "seedling", "ready", "koopa", schema.ErrUnknownStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := s.Transition(tt.typ, tt.from, tt.to, tt.actor)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Transition(%q, %q, %q, %q) = %v, want %v",
					tt.typ, tt.from, tt.to, tt.actor, err, tt.wantErr)
			}
		})
	}
}

// TestLoadRealContract locks the assumptions this repo makes about the real
// vault contract. Skips when the vault is absent (fresh clone, CI).
func TestLoadRealContract(t *testing.T) {
	t.Parallel()

	root := os.Getenv("KURODO_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home dir: %v", err)
		}
		root = filepath.Join(home, "obsidian")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))); err != nil { // #nosec G703 -- probing the operator's own vault to decide whether to skip
		t.Skipf("real vault contract not available: %v", err)
	}

	s, err := schema.Load(root)
	if err != nil {
		t.Fatalf("Load(%q) = %v", root, err)
	}
	if s.Version != "1" {
		t.Errorf("Version = %q, want %q", s.Version, "1")
	}
	st, ok := s.Stage("lesson", "ready")
	if !ok {
		t.Fatal("Stage(\"lesson\", \"ready\") not found in real contract")
	}
	want := []string{"koopa"}
	if diff := cmp.Diff(want, st.Owner); diff != "" {
		t.Errorf("ready owner mismatch (-want +got):\n%s", diff)
	}
}
