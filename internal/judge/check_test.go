package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

// TestCheckGolden drives the whole check engine — extraction, resolution, the
// graph rules, the disk-reference rule, and the frontmatter checks — over
// fixture vaults and asserts the emitted bytes equal each golden. The inherited
// rule goldens are copied from the predecessor's sorted output over the same
// fixture. The configured-supersession golden covers fields defined only by
// this repository's vault contract extension, so its bytes come from the frozen
// local wire contract. Deliberate inherited divergences live in dedicated tests
// below rather than being mislabelled as predecessor output.
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
		// The authoring contract's own diagnostics: one course written every
		// way the grammar refuses, so each rule has a line in the golden.
		{name: "study-path structure", fixture: "testdata/vault-paths", golden: "testdata/golden/paths.jsonl"},
		// The same grammar under a vault that calls its courses something else.
		// This fixture is the evidence that the type comes from the contract:
		// nothing in it is named "study-path".
		{name: "a vault that renames its courses", fixture: "testdata/vault-course", golden: "testdata/golden/course.jsonl"},
		{name: "extraction edges", fixture: "testdata/vault-edges", golden: "testdata/golden/edges.jsonl"},
		{name: "report surface", fixture: "testdata/vault-report", golden: "testdata/golden/report.jsonl"},
		// This fixture covers the local vault contract's configured fields.
		{name: "configured supersession", fixture: "testdata/vault-supersession", golden: "testdata/golden/supersession.jsonl"},
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

// TestCheckDropsDiaryFindings pins the privacy boundary across direct output
// and secondary title evidence. Findings sourced by or resolved to the
// configured private directory are removed. A public link whose only title
// owner is private remains the same public broken-link finding it would be if
// that private note did not exist; the private title neither appears in
// evidence nor suppresses the author's own finding.
func TestCheckDropsDiaryFindings(t *testing.T) {
	t.Parallel()
	root := judgeFixtureRootWithPrivacy(t, "testdata/vault-diary", "Diary")
	got := runCheck(t, root)
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
	if !bytes.Contains(got, []byte("Notes/links-to-diary.md")) {
		t.Error("a public link must not disappear because a private note owns the same title")
	}
	// The drop holds even when the full, unfiltered set is requested, checked
	// against the raw paths rather than the engine's own helper.
	all, err := check(root, nil, true)
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

func TestCheckAllRestoresSystemOnlyFindings(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestContract(t, root, nil)
	write(t, root, "System/reference.md", "# Reference\n\n[[Missing System Target]]\n")

	defaultFindings, err := check(root, nil, false)
	if err != nil {
		t.Fatalf("check(default): %v", err)
	}
	if len(defaultFindings) != 0 {
		t.Fatalf("check(default) = %+v, want System-only finding hidden", defaultFindings)
	}

	allFindings, err := check(root, nil, true)
	if err != nil {
		t.Fatalf("check(--all): %v", err)
	}
	for i := range allFindings {
		if allFindings[i].RuleID == "link.broken" && allFindings[i].Path == "System/reference.md" {
			return
		}
	}
	t.Fatalf("check(--all) = %+v, want System-only broken link restored", allFindings)
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
	authority := testScanAuthority(t, "Diary")
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
			if got := touchesEgressDenied(&tt.f, authority); got != tt.want {
				t.Errorf("touchesEgressDenied(path=%q) = %v, want %v", tt.f.Path, got, tt.want)
			}
		})
	}
}

// runCheck runs the whole check engine over root and returns the wire bytes.
func runCheck(t *testing.T, root string) []byte {
	t.Helper()
	root = judgeFixtureRoot(t, root)
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
