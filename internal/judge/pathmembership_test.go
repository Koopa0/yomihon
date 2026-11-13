package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestCourseMembershipIsTheListedLesson holds what "the course lists a note"
// means. A course lists a note by naming it as a lesson; a link in its prose, in
// a block declared out of the course, in one nobody declared, or in a row the
// grammar refused is not the course listing anything. The lesson Retired shares
// the course's domain and is not a draft, so the course either lists it or
// reports it as on disk but unlisted.
func TestCourseMembershipIsTheListedLesson(t *testing.T) {
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
		{
			// The branch declares a role nobody can read, so it is not a branch
			// of the course until the author fixes it — and its rows read as
			// perfectly ordinary lessons in the meantime.
			name: "a branch whose declaration cannot be read",
			body: "## 主線 {sequence=bogus}\n\n- [[Retired]]\n",
			want: false,
		},
		{
			// A branch declared part of the course under one declared out of
			// it: the contradiction is the author's to resolve, and nothing
			// projects from it while it stands.
			name: "a branch contradicting the one above it",
			body: "## 日常 {sequence=none}\n\n### 主線 {sequence=primary}\n\n- [[Retired]]\n",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeCourseContract(t, root)
			writeJudgeNote(t, root, courseNotePath, courseNote(tt.body))
			writeJudgeNote(t, root, courseLessonPath("Live"), courseLesson("live"))
			writeJudgeNote(t, root, courseLessonPath("Retired"), courseLesson("retired"))

			out := string(runCheck(t, root))
			if got := courseLists(out, "Retired"); got != tt.want {
				t.Errorf("course lists Retired for %q = %t, want %t; findings:\n%s", tt.body, got, tt.want, out)
			}
		})
	}
}

// TestMapsStillFollowEveryLink is the other side of the same rule: a general
// map has no declared-sequence grammar, so every link it carries is ordinary
// link health — none of them is a lesson row the course rule owns. Narrowing
// courses must not narrow maps.
func TestMapsStillFollowEveryLink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCourseContract(t, root)
	writeJudgeNote(t, root, "Maps/Shelf.md",
		"---\ntype: topic-map\n---\n# Shelf\n\n## 主線 {sequence=primary}\n\n- [[Unwritten]]\n\n關於 [[Absent]] 的說明。\n")

	out := string(runCheck(t, root))
	if n := strings.Count(out, `"rule_id":"link.broken"`); n != 2 {
		t.Errorf("link.broken reported %d times, want 2 — a map's links are all ordinary links; findings:\n%s", n, out)
	}
	if strings.Contains(out, "map.disk_mismatch") {
		t.Errorf("a map's link was reconciled as a course lesson; findings:\n%s", out)
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

// courseNotePath is the course the membership tests write. It sits in the
// golang domain beside its lessons, so a lesson the course does not list is
// reported as on disk but unlisted.
const courseNotePath = "Writing/lessons/golang/Course.md"

func courseLessonPath(name string) string {
	return "Writing/lessons/golang/" + name + ".md"
}

func courseNote(body string) string {
	return "---\ntype: study-path\n---\n# Course\n\n" + body
}

func courseLesson(slug string) string {
	return "---\ntype: lesson\nslug: " + slug + "\n---\nbody\n"
}

// courseLists reports whether the course lists the lesson called name: a lesson
// of the course's domain that it does not list is reported unlisted.
func courseLists(out, name string) bool {
	unlisted := `"rule_id":"map.disk_unlisted","severity":"warn","path":"` + courseLessonPath(name) + `"`
	return !strings.Contains(out, unlisted)
}

// writeCourseContract gives a fixture vault a contract with a course, a lesson
// and a map role, and a domain root, which is what makes course membership
// answerable at all.
func writeCourseContract(t *testing.T, root string) {
	t.Helper()
	writeJudgeNote(t, root, schema.ContractRelPath, courseContract)
}

const courseContract = `schema_version = "2"

[folders]
checked = ["Concepts", "Maps", "Writing"]
templates = []
domain_from = ["Writing/lessons"]

[enums]

[fields]
common = ["type", "draft"]
required = ["type"]

[types.note]

[types.lesson]
role = "lesson"
fields = ["slug"]

[types.study-path]
role = "course"

[types.topic-map]
role = "map"

[privacy]
never_egress_dirs = []
`
