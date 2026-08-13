package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSupersessionFollowsCourseMembership holds what "the course still points
// at an archived note" means. A course points at a note by listing it as a
// lesson; a link in its prose, in a block declared out of the course, in one
// nobody declared, or in a row the grammar refused is not the course pointing
// at anything. Reporting those told the author to fix a course that never
// listed the note.
func TestSupersessionFollowsCourseMembership(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "a lesson on the main line",
			body: "## 主線 {sequence=primary}\n\n- [[Retired]]\n",
			want: true,
		},
		{
			name: "a lesson in a side branch",
			body: "## 主線 {sequence=primary}\n\n- [[Live]]\n\t- 支線 {sequence=local}\n\t\t- [[Retired]]\n",
			want: true,
		},
		{
			name: "a link in the course's prose",
			body: "## 主線 {sequence=primary}\n\n關於 [[Retired]] 的說明。\n\n- [[Live]]\n",
			want: false,
		},
		{
			name: "a block declared out of the course",
			body: "## 主線 {sequence=primary}\n\n- [[Live]]\n\n## 日常 {sequence=none}\n\n- [[Retired]]\n",
			want: false,
		},
		{
			name: "a branch nobody declared",
			body: "## 主線 {sequence=primary}\n\n- [[Live]]\n\n## 還沒歸\n\n- [[Retired]]\n",
			want: false,
		},
		{
			name: "a row whose link is not first",
			body: "## 主線 {sequence=primary}\n\n- 補充 [[Retired]]\n",
			want: false,
		},
		{
			name: "a row naming two notes",
			body: "## 主線 {sequence=primary}\n\n- [[Retired]] 與 [[Live]]\n",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeSupersessionContract(t, root)
			writeJudgeNote(t, root, "Maps/Course.md",
				"---\ntitle: Course\ntype: study-path\nstatus: ready\n---\n\n"+tt.body)
			writeJudgeNote(t, root, "Writing/Live.md",
				"---\ntitle: Live\ntype: lesson\nstatus: ready\n---\nbody\n")
			writeJudgeNote(t, root, "Writing/Retired.md",
				"---\ntitle: Retired\ntype: lesson\nstatus: archived\n---\nbody\n")

			out := string(runCheck(t, root))
			got := strings.Contains(out, archivedNavigationRule)
			if got != tt.want {
				t.Errorf("%s reported for %q = %t, want %t; findings:\n%s",
					archivedNavigationRule, tt.body, got, tt.want, out)
			}
		})
	}
}

// TestMapsStillFollowEveryLink is the other side of the same rule: a general
// map has no declared-sequence grammar, so it keeps pointing at a note through
// any link it carries. Narrowing courses must not narrow maps.
func TestMapsStillFollowEveryLink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeSupersessionContract(t, root)
	writeJudgeNote(t, root, "Maps/Shelf.md",
		"---\ntitle: Shelf\ntype: topic-map\nstatus: ready\n---\n\n關於 [[Retired]] 的說明。\n")
	writeJudgeNote(t, root, "Writing/Retired.md",
		"---\ntitle: Retired\ntype: lesson\nstatus: archived\n---\nbody\n")

	out := string(runCheck(t, root))
	if !strings.Contains(out, archivedNavigationRule) {
		t.Errorf("a map's prose link to an archived note stopped being reported; findings:\n%s", out)
	}
}

// writeJudgeNote writes one fixture note into a temporary vault.
func writeJudgeNote(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil { // #nosec G703 -- rel is a fixed literal from this test, rooted in its own TempDir
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil { // #nosec G703 -- same fixed literal under the same TempDir
		t.Fatalf("write %s: %v", rel, err)
	}
}

// writeSupersessionContract gives a fixture vault the contract that declares
// the supersession vocabulary, which is what makes the archived-target rule
// answerable at all.
func writeSupersessionContract(t *testing.T, root string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "vault-supersession", "System", "schemas", "vault-schema.toml"))
	if err != nil {
		t.Fatalf("read supersession contract: %v", err)
	}
	writeJudgeNote(t, root, "System/schemas/vault-schema.toml", string(data))
}
