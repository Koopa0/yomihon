package judge

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

func suffixExistsVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "Notes/Projects/Atlas/README.md", "---\n---\n# Atlas page\nbody\n")
	write(t, root, "Notes/Other/README.md", "---\n---\n# Other page\nbody\n")
	write(t, root, "README.md", "---\n---\n# Root page\nbody\n")
	write(t, root, "Diary/Private/Secret.md", "---\n---\n# Kept private\nbody\n")
	write(t, root, schema.ContractRelPath, contractFixture(t, []string{"Diary"}, [2]string{`skipped_files = ["README.md"]`, "skipped_files = []"}))
	return root
}

func TestRunExistsPathSuffixKeepsMetadataAndPrivacy(t *testing.T) {
	t.Parallel()
	root := suffixExistsVault(t)
	write(t, root, "Notes/Alias.md", "---\naliases: [Atlas/README]\n---\n# Alias holder\nbody\n")
	write(t, root, "Notes/Title.md", "---\n---\n# Atlas/README\nbody\n")
	write(t, root, "Notes/Mixed/Secret.md", "---\n---\n# Public secret\nbody\n")
	write(t, root, "Diary/Mixed/Secret.md", "---\n---\n# Private secret\nbody\n")
	write(t, root, "Notes/Near.md", "---\n---\n# Ｗｉｄｅ/Ｎａｍｅ\nbody\n")
	write(t, root, "Diary/Ｗｉｄｅ/Ｎａｍｅ.md", "---\n---\n# Hidden width\nbody\n")
	tests := []struct {
		query, want string
		exit        int
	}{
		{query: "Atlas/README", want: `{"query":"Atlas/README","matches":[{"path":"Notes/Alias.md","field":"alias","value":"Atlas/README"},{"path":"Notes/Title.md","field":"title","value":"Atlas/README"}]}`, exit: 0},
		{query: "Mixed/Secret", want: `{"query":"Mixed/Secret","matches":[{"path":"Notes/Mixed/Secret.md","field":"path","value":"Notes/Mixed/Secret.md"}],"withheld":true}`, exit: 0},
		{query: "wide/name", want: `{"query":"wide/name","matches":[],"near_matches":[{"path":"Notes/Near.md","field":"title","value":"Ｗｉｄｅ/Ｎａｍｅ"}]}`, exit: 1},
	}
	// A width-fold-only location is not an exact name; it may not create the
	// private-name bit even though lexical matching finds another public title.
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()
			got, exit, err := existsBytes(t.Context(), &ExistsOptions{Root: root, Name: tt.query, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunExists() error = %v", err)
			}
			if exit != tt.exit || string(got) != tt.want+"\n" {
				t.Errorf("RunExists(%q) = (%q, %d), want (%q, %d)", tt.query, got, exit, tt.want+"\n", tt.exit)
			}
		})
	}
}

// TestRunExistsRefusesOverAnUnreadableExactClaimant holds that a path query
// whose exact claimant could not be read is refused rather than answered by a
// deeper note that merely ends the same way.
func TestRunExistsRefusesOverAnUnreadableExactClaimant(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Notes/Atlas/README.md", "---\n---\n# Exact claimant\nbody\n")
	write(t, root, "Extra/Notes/Atlas/README.md", "---\n---\n# Deeper claimant\nbody\n")
	write(t, root, schema.ContractRelPath, contractFixture(t, nil, [2]string{`skipped_files = ["README.md"]`, "skipped_files = []"}))
	unreadable(t, filepath.Join(root, "Notes", "Atlas", "README.md"))
	if got, _, err := existsBytes(t.Context(), &ExistsOptions{Root: root, Name: "Notes/Atlas/README.md", Format: FormatJSON}); err == nil {
		t.Errorf("RunExists() = %q, want a refusal over the unreadable exact claimant", got)
	}
}

// TestRunExistsPathSuffixIgnoresASkippedClaimant holds that a path query whose
// claimants the rules file skips by basename matches nothing, rather than a
// file the scan holds as a resource becoming a note answer.
func TestRunExistsPathSuffixIgnoresASkippedClaimant(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Notes/Atlas/README.md", "---\n---\n# Exact claimant\nbody\n")
	write(t, root, "Extra/Notes/Atlas/README.md", "---\n---\n# Deeper claimant\nbody\n")
	write(t, root, "Notes/Public.md", "---\n---\n# Public\nbody\n")
	write(t, root, schema.ContractRelPath, contractFixture(t, nil))
	got, exit, err := existsBytes(t.Context(), &ExistsOptions{Root: root, Name: "Notes/Atlas/README.md", Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunExists() error = %v", err)
	}
	if want := "{\"query\":\"Notes/Atlas/README.md\",\"matches\":[]}\n"; string(got) != want || exit != 1 {
		t.Errorf("RunExists() = (%q, %d), want (%q, 1)", got, exit, want)
	}
}

func TestRunExistsPathSuffix(t *testing.T) {
	t.Parallel()
	root := suffixExistsVault(t)
	tests := []struct {
		name   string
		format Format
		want   string
		exit   int
	}{
		{name: "Atlas/README", format: FormatJSON, want: `{"query":"Atlas/README","matches":[{"path":"Notes/Projects/Atlas/README.md","field":"path","value":"Notes/Projects/Atlas/README.md"}]}`, exit: 0},
		{name: "Atlas/README.md", format: FormatJSON, want: `{"query":"Atlas/README.md","matches":[{"path":"Notes/Projects/Atlas/README.md","field":"path","value":"Notes/Projects/Atlas/README.md"}]}`, exit: 0},
		{name: "Notes/Projects/Atlas/README", format: FormatJSON, want: `{"query":"Notes/Projects/Atlas/README","matches":[{"path":"Notes/Projects/Atlas/README.md","field":"path","value":"Notes/Projects/Atlas/README.md"}]}`, exit: 0},
		{name: "Nope/README", format: FormatJSON, want: `{"query":"Nope/README","matches":[]}`, exit: 1},
		{name: "Private/Secret", format: FormatJSON, want: `{"query":"Private/Secret","matches":[],"withheld":true}`, exit: 0},
		{name: "Atlas/README", format: FormatHuman, want: "\"Atlas/README\" exists in 1 note(s):\n  Notes/Projects/Atlas/README.md (matched path)\n", exit: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.format.String(), func(t *testing.T) {
			t.Parallel()
			want := tt.want
			if tt.format == FormatJSON {
				want += "\n"
			}
			got, exit, err := existsBytes(t.Context(), &ExistsOptions{Root: root, Name: tt.name, Format: tt.format})
			if err != nil {
				t.Fatalf("RunExists() error = %v", err)
			}
			if diff := cmp.Diff(struct {
				Body string
				Exit int
			}{Body: want, Exit: tt.exit}, struct {
				Body string
				Exit int
			}{Body: string(got), Exit: exit}); diff != "" {
				t.Errorf("RunExists(%q) mismatch (-want +got):\n%s", tt.name, diff)
			}
			if strings.Contains(string(got), "Diary/") {
				t.Errorf("RunExists(%q) disclosed a withheld path: %s", tt.name, got)
			}
		})
	}
}

func TestRunExistsPathSuffixResourcePriority(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "Assets/chart.svg", "resource")
	write(t, root, "Notes/Assets/chart.svg.md", "---\n---\n# Competing note\nbody\n")
	write(t, root, schema.ContractRelPath, contractFixture(t, nil))
	got, exit, err := existsBytes(t.Context(), &ExistsOptions{Root: root, Name: "Assets/chart.svg", Format: FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	if exit != 1 || string(got) != "{\"query\":\"Assets/chart.svg\",\"matches\":[]}\n" {
		t.Errorf("RunExists(Assets/chart.svg) = (%q, %d), want no note match and exit 1", got, exit)
	}
}

func TestRunCheckPathSuffixConsumers(t *testing.T) {
	t.Parallel()
	root := suffixExistsVault(t)
	write(t, root, "Concepts/Source.md", "---\nbased_on: ['[[Atlas/README]]', '[[Nope/Source]]']\n---\n[[Atlas/README#Part]]\n[[Atlas/README#Absent]]\n[[Atlas/README#^absent]]\n![[Atlas/README#Absent]]\n[[Shared/Twin]]\n[[Nope/README]]\n[[Container/Page#Part]]\n")
	write(t, root, "Notes/Projects/Atlas/README.md", "## Part\n\nBlock payload. ^piece\n")
	write(t, root, "A/Shared/Twin.md", "body\n")
	write(t, root, "B/Shared/Twin.md", "body\n")
	write(t, root, "Notes/Container/Page.md", "![[Atlas/README#Part]]\n")
	got, exit, err := checkBytes(t.Context(), &CheckOptions{Root: root, All: true, Format: FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	if exit != 0 {
		t.Fatalf("check exit = %d", exit)
	}
	var rows []string
	for _, f := range decodeFindings(t, got) {
		switch f.RuleID {
		case "link.broken", "link.section_missing", "link.block_missing", "embed.section_missing", "provenance.unresolved":
			target := *f.Target
			if f.Field != nil {
				target = *f.Field + ":" + target
			}
			resolved := ""
			if f.ResolvedTo != nil {
				resolved = *f.ResolvedTo
			}
			rows = append(rows, string(f.RuleID)+"@"+target+"->"+resolved)
		default:
			// The other rules are not what this test is about.
		}
	}
	want := []string{
		"embed.section_missing@Atlas/README#Absent->Notes/Projects/Atlas/README.md",
		"link.block_missing@Atlas/README#^absent->Notes/Projects/Atlas/README.md",
		"link.broken@Nope/README->",
		"link.broken@Shared/Twin->",
		"link.section_missing@Atlas/README#Absent->Notes/Projects/Atlas/README.md",
		"provenance.unresolved@based_on:[[Nope/Source]]->",
	}
	slices.Sort(want)
	slices.Sort(rows)
	if diff := cmp.Diff(want, rows); diff != "" {
		t.Errorf("RunCheck() suffix findings (-want +got):\n%s", diff)
	}
}

func TestRunCoveragePathSuffix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, atlasContract(t, `["Concepts", "Atlases"]`))
	write(t, root, "Concepts/Atlas/Page.md", "---\ntype: concept\n---\n# Page\nbody\n")
	write(t, root, "Atlases/Map.md", "---\ntype: atlas\nbased_on: '[[Atlas/Page]]'\n---\n# Map\n[[Shared/Twin]]\n")
	write(t, root, "Concepts/A/Shared/Twin.md", "---\ntype: concept\n---\n# A\nbody\n")
	write(t, root, "Concepts/B/Shared/Twin.md", "---\ntype: concept\n---\n# B\nbody\n")
	wantMounts(t, coverageOf(t, root), []domainCoverage{
		{Domain: "A", Concepts: 1, Mounted: 1},
		{Domain: "Atlas", Concepts: 1, Mounted: 1},
		{Domain: "B", Concepts: 1, Mounted: 1},
	}, nil, nil)
}

func TestRunCheckPathSuffixCourseDisk(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, contractFixture(t, nil, [2]string{`domain_from = ["Concepts"]`, `domain_from = ["Writing/lessons"]`}))
	write(t, root, "Writing/lessons/golang/Atlas/Lesson.md", "---\ntype: lesson\ncreated: 2026-01-01\nslug: lesson\n---\nbody\n")
	write(t, root, "Writing/lessons/golang/Course.md", "---\ntype: study-path\ncreated: 2026-01-01\n---\n## Main {sequence=primary}\n- [[Atlas/Lesson]]\n- [[Nope/Lesson]]\n")
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, f := range findings {
		if (f.RuleID == "course.disk_mismatch" || f.RuleID == "course.disk_unlisted") && f.Target != nil {
			rows = append(rows, string(f.RuleID)+"@"+*f.Target)
		}
	}
	if diff := cmp.Diff([]string{"course.disk_mismatch@Nope/Lesson"}, rows); diff != "" {
		t.Errorf("Check() course disk findings (-want +got):\n%s", diff)
	}
}
