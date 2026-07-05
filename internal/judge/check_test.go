package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
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

// TestCheckDropsDiaryFindings pins the drop that keeps the private daily
// journal out of check output, across every channel a journal path could reach
// a report through. A finding is removed when a path it touches — its citing
// path, a collision member, or the note a link resolved to — is under Diary/;
// and a link that resolves to a journal note's title is dropped at its source,
// so the journal's path never lands in the finding's evidence text. The journal
// is still scanned (see TestCollectNotesScanBoundary), so its links resolve for
// other notes; only the reporting is suppressed. The golden is written from
// this engine, not the reference, because the drop is exactly where the two
// differ. The fixture pairs a broken link inside the journal and a public note
// whose title-link resolves into the journal (both dropped) with a broken link
// outside it (kept), so a blanket drop would fail the kept-link assertion while
// every "no Diary/" assertion also covers the evidence-text channel.
func TestCheckDropsDiaryFindings(t *testing.T) {
	t.Parallel()
	got := runCheck(t, "testdata/vault-diary")
	want, err := os.ReadFile("testdata/golden/diary.jsonl")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("diary findings differ from golden\ngot:\n%s\nwant:\n%s", got, want)
	}
	// The intent, asserted directly so reverting the drop fails loudly: the
	// journal entry's finding is gone, the note's is kept.
	if bytes.Contains(got, []byte("Diary/")) {
		t.Error("a finding touching the private daily journal must not be reported")
	}
	if !bytes.Contains(got, []byte("Notes/keep.md")) {
		t.Error("a broken link outside the journal must still be reported")
	}
	// The drop holds even when the full, unfiltered set is requested, checked
	// against the raw paths rather than the engine's own helper.
	all, err := check("testdata/vault-diary", nil, true)
	if err != nil {
		t.Fatalf("check(--all): %v", err)
	}
	for i := range all {
		if strings.HasPrefix(all[i].Path, "Diary/") {
			t.Errorf("the full set still reported journal path %q (%s)", all[i].Path, all[i].RuleID)
		}
		for _, m := range all[i].CollisionMembers {
			if strings.HasPrefix(m, "Diary/") {
				t.Errorf("the full set still reported journal collision member %q (%s)", m, all[i].RuleID)
			}
		}
		if all[i].ResolvedTo != nil && strings.HasPrefix(*all[i].ResolvedTo, "Diary/") {
			t.Errorf("the full set still resolved a finding to journal path %q (%s)", *all[i].ResolvedTo, all[i].RuleID)
		}
	}
}

// TestTouchesDiary pins which fields make a finding count as touching the
// private daily journal: the citing path, a collision member, and the note a
// link resolved to all count, but the link's own target text does not — it is
// the citing author's words, so a public note whose link merely reads like a
// journal name is not a journal reference.
func TestTouchesDiary(t *testing.T) {
	t.Parallel()
	diary := "Diary/2026-07-01.md"
	public := "Notes/other.md"
	tests := []struct {
		name string
		f    Finding
		want bool
	}{
		{name: "citing path in journal", f: Finding{Path: diary}, want: true},
		{name: "collision member in journal", f: Finding{Path: public, CollisionMembers: []string{public, diary}}, want: true},
		{name: "resolved into journal", f: Finding{Path: public, ResolvedTo: &diary}, want: true},
		{name: "target text is not counted", f: Finding{Path: public, Target: &diary}, want: false},
		{name: "wholly public", f: Finding{Path: public, ResolvedTo: &public}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := touchesDiary(&tt.f); got != tt.want {
				t.Errorf("touchesDiary(path=%q) = %v, want %v", tt.f.Path, got, tt.want)
			}
		})
	}
}

// TestCheckMatchesReferenceOnRealVault runs the reference tool and this engine
// over the same live vault and asserts the whole check output is byte-identical
// — every rule, not just the frontmatter subset. It is skipped when the
// reference tool or the vault is absent, so it never blocks a build elsewhere.
// Two behaviors depart from the reference (see TestCheckSkipsFileReferencesInComments
// and TestCheckDropsDiaryFindings); the real vault holds no input that triggers
// either — no file reference inside a comment, no finding touching the private
// daily journal — so the whole-vault bytes stay identical.
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
