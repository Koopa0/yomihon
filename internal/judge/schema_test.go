package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/schema"
)

// TestCheckSchemaGolden drives the frontmatter engine over fixture vaults and
// asserts the emitted bytes equal each golden. The rules fixture triggers
// every schema rule at least once and exercises the four scope behaviors that
// must produce nothing: a skipped basename, a note outside the knowledge
// directories, a note with no frontmatter, and a note whose block does not
// parse (which yields exactly one fault). The coercion fixture pins how an
// unquoted scalar's value reaches a finding: booleans lowercased, integers in
// decimal, nulls dropped, and everything else — quoted text, the 1.1 boolean
// words, out-of-range numbers, reals, aliases — left as written. The strictness
// fixture pins the parse boundary: a repeated key (nested, in a list, flow or
// block), a tab indent, an invalid escape, and an unterminated quote each
// become a single parse fault, while a merge key is read as an ordinary key.
// Each golden
// holds the schema subset of the reference tool's sorted output over that same
// fixture.
func TestCheckSchemaGolden(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fixture string
		golden  string
	}{
		{name: "rules", fixture: "testdata/vault-schema", golden: "testdata/golden/schema.jsonl"},
		{name: "scalar coercion", fixture: "testdata/vault-coercion", golden: "testdata/golden/coercion.jsonl"},
		{name: "parser strictness", fixture: "testdata/vault-strictness", golden: "testdata/golden/strictness.jsonl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want, err := os.ReadFile(tt.golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			got := runSchema(t, tt.fixture)
			if !bytes.Equal(got, want) {
				t.Errorf("schema findings differ from golden %s\ngot:\n%s\nwant:\n%s\ngot hex:\n%s\nwant hex:\n%s",
					tt.golden, got, want, hex.Dump(got), hex.Dump(want))
			}
		})
	}
}

// TestCheckSchemaMatchesReferenceOnRealVault is the strongest guard: it runs
// the reference tool and this engine over the same live vault and asserts the
// schema lines are byte-identical. Because the live vault is currently free of
// schema violations, this proves the engine raises no false positive on real,
// messy data; the day a violation appears, it becomes a true byte-for-byte
// conformance check. It is skipped when the reference tool or the vault is
// absent, so it never blocks a build elsewhere.
func TestCheckSchemaMatchesReferenceOnRealVault(t *testing.T) {
	t.Parallel()
	tool := referenceTool()
	if tool == "" {
		t.Skip("set KURODO_REFERENCE_BIN to the conformance reference binary to run this check")
	}
	vaultRoot := referenceVault()
	if vaultRoot == "" {
		t.Skip("vault not found; set KURODO_VAULT to a root holding " + schema.ContractRelPath)
	}

	// #nosec G204 -- tool is resolved from a trusted environment variable set by the operator, not from user input
	out, err := exec.CommandContext(t.Context(), tool, "check", "--root", vaultRoot, "--format", "json").Output()
	if err != nil {
		// With no deny flag the tool always exits zero, so a non-nil error
		// here is a real tool failure worth failing on.
		t.Fatalf("run reference tool: %v", err)
	}
	want := schemaLines(string(out))

	got := runSchema(t, vaultRoot)
	if !bytes.Equal(got, want) {
		t.Errorf("schema findings differ from the reference tool on %s\ngot:\n%s\nwant:\n%s",
			vaultRoot, got, want)
	}
}

// runSchema collects the notes under root, runs the frontmatter engine, sorts,
// and returns the wire bytes.
func runSchema(t *testing.T, root string) []byte {
	t.Helper()
	notes, err := collectNotes(root)
	if err != nil {
		t.Fatalf("collectNotes(%q): %v", root, err)
	}
	s, err := schema.Load(root)
	if err != nil {
		t.Fatalf("schema.Load(%q): %v", root, err)
	}
	findings, err := checkSchema(notes, s)
	if err != nil {
		t.Fatalf("checkSchema: %v", err)
	}
	sortFindings(findings)
	var buf bytes.Buffer
	if err := WriteJSONL(&buf, findings); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	return buf.Bytes()
}

// schemaLines keeps only the schema.* lines of a JSONL stream, preserving
// order and the trailing newline of each kept line.
func schemaLines(jsonl string) []byte {
	var buf bytes.Buffer
	for line := range strings.SplitSeq(jsonl, "\n") {
		if strings.Contains(line, `"rule_id":"schema.`) {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

// referenceTool returns the path to the conformance reference binary named by
// the KURODO_REFERENCE_BIN environment variable, or "" when it is unset or not
// executable. The binary is named only by that variable, so this scaffold
// carries no path or name of its own; the conformance run is opt-in and the
// test skips when the variable is not set.
func referenceTool() string {
	if bin := os.Getenv("KURODO_REFERENCE_BIN"); bin != "" && isExecutable(bin) {
		return bin
	}
	return ""
}

// referenceVault locates a vault root holding the contract file: KURODO_VAULT,
// else ~/obsidian. Returns "" when none holds the contract.
func referenceVault() string {
	if root := os.Getenv("KURODO_VAULT"); root != "" {
		if hasContract(root) {
			return root
		}
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil {
		if root := filepath.Join(home, "obsidian"); hasContract(root) {
			return root
		}
	}
	return ""
}

func isExecutable(path string) bool {
	info, err := os.Stat(path) // #nosec G703 -- path is a trusted env var or fixed install location probed for the conformance run
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func hasContract(root string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(schema.ContractRelPath))) // #nosec G703 -- root is a trusted env var or the default vault location probed for the conformance run
	return err == nil
}
