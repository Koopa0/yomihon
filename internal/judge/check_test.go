package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"os/exec"
	"testing"
)

// TestCheckGolden drives the whole check engine — extraction, resolution, the
// graph rules, the disk-reference rule, and the frontmatter checks — over
// fixture vaults and asserts the emitted bytes equal each golden. Each golden
// is the reference tool's sorted output over that same fixture, so a byte
// difference is a divergence from the frozen wire format. The one deliberate
// departure from the reference lives in its own test below, keeping every
// golden here a faithful copy of the reference's bytes.
func TestCheckGolden(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fixture string
		golden  string
	}{
		{name: "graph rules", fixture: "testdata/vault", golden: "testdata/golden/check.jsonl"},
		{name: "map vs disk", fixture: "testdata/vault-maps", golden: "testdata/golden/maps.jsonl"},
		{name: "escape surface", fixture: "testdata/vault-escapes", golden: "testdata/golden/escapes.jsonl"},
		{name: "line arithmetic", fixture: "testdata/vault-lines", golden: "testdata/golden/lines.jsonl"},
		{name: "planned vs broken", fixture: "testdata/vault-planned", golden: "testdata/golden/planned.jsonl"},
		{name: "disk references", fixture: "testdata/vault-diskref", golden: "testdata/golden/diskref.jsonl"},
		{name: "system scope", fixture: "testdata/vault-scope", golden: "testdata/golden/scope.jsonl"},
		{name: "map mismatch branches", fixture: "testdata/vault-mapmismatch", golden: "testdata/golden/mapmismatch.jsonl"},
		{name: "extraction edges", fixture: "testdata/vault-edges", golden: "testdata/golden/edges.jsonl"},
		{name: "report surface", fixture: "testdata/vault-report", golden: "testdata/golden/report.jsonl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want, err := os.ReadFile(tt.golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			got := runCheck(t, tt.fixture)
			if !bytes.Equal(got, want) {
				t.Errorf("check findings differ from golden %s\ngot:\n%s\nwant:\n%s\ngot hex:\n%s\nwant hex:\n%s",
					tt.golden, got, want, hex.Dump(got), hex.Dump(want))
			}
		})
	}
}

// TestCheckSkipsFileReferencesInComments pins the one place this engine
// deliberately departs from the reference. The reference checks a wikilink
// inside an Obsidian %%...%% comment as commented-out (it is not reported) but
// still checks a markdown or backticked file reference in the same comment — an
// inconsistency. Commented-out content is not a live reference, so this engine
// skips file references in comments too, matching how it already treats
// wikilinks. The golden is written by hand rather than copied from the
// reference, because it is exactly the output that should differ; regenerating
// it from the reference would reintroduce the finding this rule drops. The real
// vault carries no file reference inside a comment, so the whole-vault
// comparison stays byte-identical; this divergence surfaces only on the fixture.
func TestCheckSkipsFileReferencesInComments(t *testing.T) {
	t.Parallel()
	got := runCheck(t, "testdata/vault-comment-scope")
	want, err := os.ReadFile("testdata/golden/comment-scope.jsonl")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("comment-scope findings differ from golden\ngot:\n%s\nwant:\n%s", got, want)
	}
	// The intent, asserted directly so a revert to the reference's behavior
	// fails loudly: the reference in a comment is dropped, the one outside is
	// kept.
	if bytes.Contains(got, []byte("in-comment.md")) {
		t.Error("a file reference inside a comment must not be reported")
	}
	if !bytes.Contains(got, []byte("out-of-comment.md")) {
		t.Error("a file reference outside a comment must still be reported")
	}
}

// TestCheckMatchesReferenceOnRealVault runs the reference tool and this engine
// over the same live vault and asserts the whole check output is byte-identical
// — every rule, not just the frontmatter subset. It is skipped when the
// reference tool or the vault is absent, so it never blocks a build elsewhere.
// One rule departs from the reference (see TestCheckSkipsFileReferencesInComments);
// the real vault holds no input that triggers it, so the whole-vault bytes stay
// identical.
func TestCheckMatchesReferenceOnRealVault(t *testing.T) {
	t.Parallel()
	tool := referenceTool()
	if tool == "" {
		t.Skip("set KURODO_REFERENCE_BIN to the conformance reference binary to run this check")
	}
	vaultRoot := referenceVault()
	if vaultRoot == "" {
		t.Skip("vault not found; set KURODO_VAULT to a root holding the contract")
	}

	// #nosec G204 -- tool is resolved from a trusted environment variable set by the operator, not from user input
	out, err := exec.CommandContext(t.Context(), tool, "check", "--root", vaultRoot, "--format", "json").Output()
	if err != nil {
		t.Fatalf("run reference tool: %v", err)
	}
	want := out

	got := runCheck(t, vaultRoot)
	if !bytes.Equal(got, want) {
		t.Errorf("check findings differ from the reference tool on %s\ngot:\n%s\nwant:\n%s",
			vaultRoot, got, want)
	}
}

// runCheck runs the whole check engine over root and returns the wire bytes.
func runCheck(t *testing.T, root string) []byte {
	t.Helper()
	findings, err := Check(root)
	if err != nil {
		t.Fatalf("Check(%q): %v", root, err)
	}
	var buf bytes.Buffer
	if err := WriteJSONL(&buf, findings); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	return buf.Bytes()
}
