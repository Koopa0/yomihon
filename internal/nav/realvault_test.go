package nav

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
)

// TestBuildRealVault builds the navigation model against the real vault
// (~/obsidian, or YOMIHON_ROOT) and asserts the navigation acceptance
// criteria against the two real study-path files, the lifecycle
// folder tree, and the reports list. It follows the same
// t.Skipf-when-vault-absent pattern as internal/render's realvault_test and
// internal/schema's TestLoadRealContract, so it runs whenever the vault is
// present and is skipped loudly (not vacuously green) when it is not.
//
// The exact entry counts (115 Go, 20 大家) are hand-derived from the two
// files on 2026-07-02: 115 "- [[...]]" bullets in Maps/Go 課綱.md, and 20
// wikilink-bearing "- **L..**" bullets in Maps/大家...學習路徑.md (L1-L20;
// L21-L25 are "待建" with no wikilink, and the daily-loop/table/gaps
// branches carry no entry bullets). Update these literals if the study paths
// grow — a mismatch means the parser dropped or gained an entry, which is
// exactly what this guards.
func TestBuildRealVault(t *testing.T) {
	t.Parallel()

	root := realVaultRoot(t)

	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("graph.Build(%q) = %v", root, err)
	}
	m, err := Build(root, idx, nil)
	if err != nil {
		t.Fatalf("Build(%q) = %v", root, err)
	}

	goPath := findPath(t, m, "Go 課綱")
	minnaPath := findPath(t, m, "大家")

	// --- structure: 大家 is NOT isomorphic to Go ---
	// Go: 9 H2 parts, 27 H3 modules, 115 entries — all headings kept.
	if got := len(goPath.Branches); got != 9 {
		t.Errorf("Go syllabus top-level parts = %d, want 9", got)
	}
	if got := countSubBranches(goPath.Branches); got != 27 {
		t.Errorf("Go syllabus modules (H3) = %d, want 27", got)
	}
	if got := countEntries(goPath.Branches); got != 115 {
		t.Errorf("Go syllabus entries = %d, want 115", got)
	}
	// 大家: only the course-sequence part survives pruning (loop/table/gaps
	// drop out), with 5 learning-level H3s and 20 entries.
	if got := len(minnaPath.Branches); got != 1 {
		t.Errorf("大家 syllabus top-level parts = %d, want 1 (only the course sequence)", got)
	}
	if got := countEntries(minnaPath.Branches); got != 20 {
		t.Errorf("大家 syllabus entries = %d, want 20", got)
	}
	if len(minnaPath.Branches) == 1 {
		if got := len(minnaPath.Branches[0].Sub); got != 5 {
			t.Errorf("大家 course-sequence levels = %d, want 5", got)
		}
	}

	// --- order matches file order (the sidebar must mirror the file's own listing order) ---
	goEntries := flattenEntries(goPath.Branches)
	wantGoHead := []string{
		"Slices- Strings and Slices", "Arrays- Mechanical Sympathy",
		"Variables", "Constants", "Struct Types",
	}
	assertLeadingTargets(t, "Go", goEntries, wantGoHead)

	minnaEntries := flattenEntries(minnaPath.Branches)
	wantMinnaHead := []string{
		"L01 〜は〜です", "L02 これ・それ・あれ", "L03 ここ・そこ・あそこ",
	}
	assertLeadingTargets(t, "大家", minnaEntries, wantMinnaHead)

	// Every entry in navigation is resolved by construction; these real lesson
	// targets also carry their status badges.
	for _, entry := range minnaEntries {
		if entry.Status == "" {
			t.Errorf("大家 entry %q has no status", entry.Target)
		}
	}

	// --- lifecycle folder tree: expected top-level folders, in order ---
	assertLifecycleFolders(t, m)

	// --- reports: non-empty and includes latest.html ---
	if len(m.Reports) == 0 {
		t.Error("reports list is empty")
	}
	if !slices.ContainsFunc(m.Reports, func(r Report) bool { return r.Name == "latest.html" && r.Latest }) {
		t.Errorf("reports list does not include latest.html marked latest; got %d reports", len(m.Reports))
	}
	if len(m.Journal) != 0 {
		t.Errorf("real-vault Journal = %v, want empty while Diary/ is absent", m.Journal)
	}

	t.Logf("real vault: Go=%d entries, 大家=%d entries, %d maps, %d reports, %d top-level folders",
		countEntries(goPath.Branches), countEntries(minnaPath.Branches), len(m.Maps), len(m.Reports), len(m.Folders))
}

func realVaultRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("YOMIHON_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home dir: %v", err)
		}
		root = filepath.Join(home, "obsidian")
	}
	if _, err := os.Stat(root); err != nil { // #nosec G703 -- probing the operator's own vault to decide whether to skip
		t.Skipf("real vault not available: %v", err)
	}
	return root
}

func findPath(t *testing.T, m *Model, relPathSubstr string) Map {
	t.Helper()
	for _, s := range m.Paths {
		if strings.Contains(s.RelPath, relPathSubstr) {
			return s
		}
	}
	t.Fatalf("no study-path note whose path contains %q; found %d paths", relPathSubstr, len(m.Paths))
	return Map{}
}

func countEntries(branches []Branch) int {
	n := 0
	for _, s := range branches {
		n += len(s.Entries) + countEntries(s.Sub)
	}
	return n
}

func countSubBranches(branches []Branch) int {
	n := 0
	for _, s := range branches {
		n += len(s.Sub) + countSubBranches(s.Sub)
	}
	return n
}

func flattenEntries(branches []Branch) []Entry {
	var out []Entry
	for _, s := range branches {
		out = append(out, s.Entries...)
		out = append(out, flattenEntries(s.Sub)...)
	}
	return out
}

func assertLeadingTargets(t *testing.T, name string, entries []Entry, want []string) {
	t.Helper()
	if len(entries) < len(want) {
		t.Fatalf("%s: only %d entries, want at least %d to check order", name, len(entries), len(want))
	}
	for i, w := range want {
		if entries[i].Target != w {
			t.Errorf("%s entry[%d].Target = %q, want %q (order must match file order)", name, i, entries[i].Target, w)
		}
	}
}

func assertLifecycleFolders(t *testing.T, m *Model) {
	t.Helper()
	names := make([]string, 0, len(m.Folders))
	for _, f := range m.Folders {
		names = append(names, f.Name)
	}

	// The folder tree is files-driven, so an empty lifecycle directory
	// (currently Inbox and Drafts, both holding 0 files) has nothing to
	// browse and is omitted; it reappears the moment it holds a file. So
	// assert two things rather than "all nine present":
	//   1. every lifecycle folder that IS present appears in lifecycle order
	//      (a subsequence of lifecycleOrder), and
	//   2. the content-bearing lifecycle folders are all present.
	last := -1
	for _, n := range names {
		i := slices.Index(lifecycleOrder, n)
		if i < 0 {
			continue // an unknown folder (e.g. a future one) sorts after the known block
		}
		if i <= last {
			t.Errorf("lifecycle folder %q is out of order; got top level %v", n, names)
		}
		last = i
	}
	for _, w := range []string{"Sources", "Concepts", "Maps", "Synthesis", "Writing", "System", "Views", "Diagrams"} {
		if !slices.Contains(names, w) {
			t.Errorf("expected top-level lifecycle folder %q missing; got %v", w, names)
		}
	}
}
