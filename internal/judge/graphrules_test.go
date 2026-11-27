package judge

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// unlistedDir is the japanese domain's folder, where the courses and lessons
// below are written.
const unlistedDir = "Writing/lessons/japanese/"

// TestUnlistedLessonExemptsOnlyADraft holds that course.disk_unlisted answers from
// listing and from the draft flag alone. A lesson no course of its domain lists
// is reported unless the author declared it a draft, and only the boolean true
// declares one: a flag written as text, as false, or left out is a finished
// lesson the course has forgotten.
func TestUnlistedLessonExemptsOnlyADraft(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		draft string
		want  bool
	}{
		{name: "no draft flag", draft: "", want: true},
		{name: "a draft", draft: "draft: true\n", want: false},
		{name: "declared finished", draft: "draft: false\n", want: true},
		{name: "true written as text", draft: "draft: \"true\"\n", want: true},
		{name: "yes is not a boolean", draft: "draft: yes\n", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeCourseContract(t, root)
			write(t, root, unlistedDir+"Japanese Path.md",
				"---\ntype: study-path\n---\n# Japanese Path\n\n## Main {sequence=primary}\n\n- [[Listed]]\n")
			write(t, root, unlistedDir+"Listed.md", "---\ntype: lesson\nslug: listed\n---\nbody\n")
			write(t, root, unlistedDir+"Unlisted.md", "---\ntype: lesson\nslug: unlisted\n"+tt.draft+"---\nbody\n")

			out := string(runCheck(t, root))
			got := strings.Contains(out, `"rule_id":"course.disk_unlisted","severity":"warn","path":"`+unlistedDir+`Unlisted.md"`)
			if got != tt.want {
				t.Errorf("course.disk_unlisted for an unlisted lesson with %q = %t, want %t; findings:\n%s",
					tt.draft, got, tt.want, out)
			}
		})
	}
}

// TestDiskUnlistedUsesTheDomainPathUnion holds that course.disk_unlisted
// answers from the union of a domain's courses, not from each course
// walking the domain alone. Two paths that each list half the lessons of one
// domain produce no finding: those lessons are listed, just not on every path.
func TestDiskUnlistedUsesTheDomainPathUnion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCourseContract(t, root)
	write(t, root, unlistedDir+"Path A.md",
		"---\ntype: study-path\n---\n# Path A\n\n## Main {sequence=primary}\n\n- [[Lesson A]]\n")
	write(t, root, unlistedDir+"Path B.md",
		"---\ntype: study-path\n---\n# Path B\n\n## Main {sequence=primary}\n\n- [[Lesson B]]\n")
	write(t, root, unlistedDir+"Lesson A.md", "---\ntype: lesson\nslug: lessona\n---\nbody\n")
	write(t, root, unlistedDir+"Lesson B.md", "---\ntype: lesson\nslug: lessonb\n---\nbody\n")

	out := string(runCheck(t, root))
	if strings.Contains(out, `"rule_id":"course.disk_unlisted"`) {
		t.Errorf("course.disk_unlisted fired for lessons listed on a parallel path of the same domain; findings:\n%s", out)
	}
}

// TestDiskUnlistedIsOneFindingPerLesson holds that an unlisted lesson is one
// domain fact, not one row per course of that domain. Two courses that list
// none of a lesson produce one finding, attached to the first course in check
// order — the file the reader opens next to add the lesson.
func TestDiskUnlistedIsOneFindingPerLesson(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCourseContract(t, root)
	write(t, root, unlistedDir+"Path A.md",
		"---\ntype: study-path\n---\n# Path A\n\n## Main {sequence=primary}\n\n- [[Listed]]\n")
	write(t, root, unlistedDir+"Path B.md",
		"---\ntype: study-path\n---\n# Path B\n\n## Main {sequence=primary}\n\n- [[Listed]]\n")
	write(t, root, unlistedDir+"Listed.md", "---\ntype: lesson\nslug: listed\n---\nbody\n")
	write(t, root, unlistedDir+"Unlisted.md", "---\ntype: lesson\nslug: unlisted\n---\nbody\n")

	var got []string
	for _, f := range decodeFindings(t, runCheck(t, root)) {
		if f.RuleID == "course.disk_unlisted" {
			got = append(got, f.Path+" -> "+*f.ResolvedTo)
		}
	}
	if diff := cmp.Diff([]string{unlistedDir + "Unlisted.md -> " + unlistedDir + "Path A.md"}, got); diff != "" {
		t.Errorf("course.disk_unlisted findings (-want +got):\n%s", diff)
	}
}
