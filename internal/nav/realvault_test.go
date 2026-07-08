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
// (~/obsidian, or KURODO_ROOT) and asserts the navigation acceptance
// criteria against the two real study-path files, the lifecycle
// folder tree, and the reports list. It follows the same
// t.Skipf-when-vault-absent pattern as internal/render's realvault_test and
// internal/schema's TestLoadRealContract, so it runs whenever the vault is
// present and is skipped loudly (not vacuously green) when it is not.
//
// The exact lesson counts (115 Go, 20 大家) are hand-derived from the two
// files on 2026-07-02: 115 "- [[...]]" bullets in Maps/Go 課綱.md, and 20
// wikilink-bearing "- **L..**" bullets in Maps/大家...學習路徑.md (L1-L20;
// L21-L25 are "待建" with no wikilink, and the daily-loop/table/gaps
// sections carry no lesson bullets). Update these literals if the syllabi
// grow — a mismatch means the parser dropped or gained a lesson, which is
// exactly what this guards.
func TestBuildRealVault(t *testing.T) {
	t.Parallel()

	root := realVaultRoot(t)

	idx, err := graph.Build(root)
	if err != nil {
		t.Fatalf("graph.Build(%q) = %v", root, err)
	}
	m, err := Build(root, idx)
	if err != nil {
		t.Fatalf("Build(%q) = %v", root, err)
	}

	goSyl := findSyllabus(t, m, "Go 課綱")
	minnaSyl := findSyllabus(t, m, "大家")

	// --- structure: 大家 is NOT isomorphic to Go ---
	// Go: 9 H2 parts, 27 H3 modules, 115 lessons — all headings kept.
	if got := len(goSyl.Sections); got != 9 {
		t.Errorf("Go syllabus top-level parts = %d, want 9", got)
	}
	if got := countSubSections(goSyl.Sections); got != 27 {
		t.Errorf("Go syllabus modules (H3) = %d, want 27", got)
	}
	if got := countLessons(goSyl.Sections); got != 115 {
		t.Errorf("Go syllabus lessons = %d, want 115", got)
	}
	// 大家: only the course-sequence part survives pruning (loop/table/gaps
	// drop out), with 5 learning-stage H3s and 20 lessons.
	if got := len(minnaSyl.Sections); got != 1 {
		t.Errorf("大家 syllabus top-level parts = %d, want 1 (only the course sequence)", got)
	}
	if got := countLessons(minnaSyl.Sections); got != 20 {
		t.Errorf("大家 syllabus lessons = %d, want 20", got)
	}
	if len(minnaSyl.Sections) == 1 {
		if got := len(minnaSyl.Sections[0].Sub); got != 5 {
			t.Errorf("大家 course-sequence stages = %d, want 5", got)
		}
	}

	// --- order matches file order (the sidebar must mirror the file's own listing order) ---
	goLessons := flattenLessons(goSyl.Sections)
	wantGoHead := []string{
		"Slices- Strings and Slices", "Arrays- Mechanical Sympathy",
		"Variables", "Constants", "Struct Types",
	}
	assertLeadingTargets(t, "Go", goLessons, wantGoHead)

	minnaLessons := flattenLessons(minnaSyl.Sections)
	wantMinnaHead := []string{
		"L01 〜は〜です", "L02 これ・それ・あれ", "L03 ここ・そこ・あそこ",
	}
	assertLeadingTargets(t, "大家", minnaLessons, wantMinnaHead)

	// Every 大家 L01-L20 lesson resolves to a real note (they exist on
	// disk); a resolved lesson carries a status badge.
	resolved := 0
	for _, l := range minnaLessons {
		if l.Resolution == graph.Unique {
			resolved++
		}
	}
	if resolved != 20 {
		t.Errorf("大家 resolved lessons = %d, want 20 (all L01-L20 exist)", resolved)
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

	t.Logf("real vault: Go=%d lessons, 大家=%d lessons (%d resolved), %d reports, %d top-level folders",
		countLessons(goSyl.Sections), countLessons(minnaSyl.Sections), resolved, len(m.Reports), len(m.Folders))
}

func realVaultRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("KURODO_ROOT")
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

func findSyllabus(t *testing.T, m *Model, relPathSubstr string) Syllabus {
	t.Helper()
	for _, s := range m.Syllabi {
		if strings.Contains(s.RelPath, relPathSubstr) {
			return s
		}
	}
	t.Fatalf("no study-path note whose path contains %q; found %d syllabi", relPathSubstr, len(m.Syllabi))
	return Syllabus{}
}

func countLessons(sections []Section) int {
	n := 0
	for _, s := range sections {
		n += len(s.Lessons) + countLessons(s.Sub)
	}
	return n
}

func countSubSections(sections []Section) int {
	n := 0
	for _, s := range sections {
		n += len(s.Sub) + countSubSections(s.Sub)
	}
	return n
}

func flattenLessons(sections []Section) []Lesson {
	var out []Lesson
	for _, s := range sections {
		out = append(out, s.Lessons...)
		out = append(out, flattenLessons(s.Sub)...)
	}
	return out
}

func assertLeadingTargets(t *testing.T, name string, lessons []Lesson, want []string) {
	t.Helper()
	if len(lessons) < len(want) {
		t.Fatalf("%s: only %d lessons, want at least %d to check order", name, len(lessons), len(want))
	}
	for i, w := range want {
		if lessons[i].Target != w {
			t.Errorf("%s lesson[%d].Target = %q, want %q (order must match file order)", name, i, lessons[i].Target, w)
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
