package judge

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestNameCollisionLeavesTheAliasRuleItsOwnRepairs pins the boundary between
// the two collision rules. Both answer "this name resolves to no single file",
// and an alias two notes declare satisfies both descriptions, so without the
// exclusion the operator would be told to fix one thing under two rule ids.
// The citation is the other half: the link to the shared name is reported by
// nothing else, because a name two files answer to resolves ambiguously rather
// than not at all.
func TestNameCollisionLeavesTheAliasRuleItsOwnRepairs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const complete = "created: 2026-01-01\n---\n"
	write(t, root, "Lessons/dup.md", "---\ntype: lesson\nslug: lessons-dup\n"+complete+"# Lessons 的 dup\n\nThe other one.\n")
	write(t, root, "Notes/dup.md", "---\ntype: writing\n"+complete+"# Notes 的 dup\n\nOne of two files answering to the same name.\n")
	write(t, root, "Notes/alias-a.md", "---\naliases: [shared]\ntype: writing\n"+complete+"# Alias A\n\nOne alias owner.\n")
	write(t, root, "Notes/alias-b.md", "---\naliases: [shared]\ntype: writing\n"+complete+"# Alias B\n\nThe other alias owner.\n")
	write(t, root, "Notes/cite.md", "---\ntype: writing\n"+complete+"# 引用者\n\nThe link nobody can resolve: [[dup]].\n")
	writeTestContract(t, root, nil)

	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("Check(): %v", err)
	}
	byRule := make(map[string][]string)
	for i := range findings {
		if findings[i].Target == nil {
			t.Fatalf("%s finding has no target: %+v", findings[i].RuleID, findings[i])
		}
		byRule[string(findings[i].RuleID)] = append(byRule[string(findings[i].RuleID)], *findings[i].Target)
	}
	if diff := cmp.Diff(map[string][]string{
		"collision.name":  {"dup"},
		"collision.alias": {"shared"},
	}, byRule); diff != "" {
		t.Errorf("Check() findings by rule mismatch (-want +got):\n%s", diff)
	}
}

// TestNameCollisionCountsOnlyDescribableFiles pins the privacy order: members
// are filtered before the count, not censored after it. A name one public file
// shares with a withheld one is no collision this face knows about — reporting
// it would state that something else out there answers to the name, which is a
// description of the withheld file — while a name two public files share is
// reported with those two named and the withheld third absent from every field.
//
// The withheld notes claim the shared name under different forms on purpose —
// one as a filename, one as an alias — because that is what makes the index's
// answer for "pair" and for "pair.md" differ while the describable answer for
// both is the same pair of files. Deciding which of the two names restates the
// other before the withheld claimants are removed splits one repair into two
// rows, each naming the same two files.
func TestNameCollisionCountsOnlyDescribableFiles(t *testing.T) {
	t.Parallel()

	out := runCheck(t, judgeFixtureRootWithPrivacy(t, "testdata/vault-namecollision-privacy", "Private"))
	if bytes.Contains(out, []byte("Private/")) {
		t.Errorf("a withheld member appears in the report:\n%s", out)
	}
	members := map[string][]string{}
	for _, f := range decodeFindings(t, out) {
		if f.RuleID == "collision.name" && f.Target != nil {
			members[*f.Target] = f.CollisionMembers
		}
	}
	want := map[string][]string{"pair": {"Lessons/pair.md", "Notes/pair.md"}}
	if diff := cmp.Diff(want, members); diff != "" {
		t.Errorf("collision.name members by target (-want +got):\n%s", diff)
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
	all, err := runCheckAction(t.Context(), root, nil, true)
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
	write(t, root, schema.ContractRelPath, contractFixture(t, nil,
		[2]string{
			`checked = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`,
			`checked = ["Notes"]`,
		}))
	write(t, root, "Notes/keep.md", "---\ntype: writing\ncreated: 2026-01-01\n---\n# Keep\n")
	write(t, root, "System/reference.md", "# Reference\n\n[[Missing System Target]]\n")

	defaultFindings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatalf("check(default): %v", err)
	}
	if len(defaultFindings) != 0 {
		t.Fatalf("check(default) = %+v, want System-only finding hidden", defaultFindings)
	}

	allFindings, err := runCheckAction(t.Context(), root, nil, true)
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
