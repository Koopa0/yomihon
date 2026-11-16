package judge

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// unlistedContract declares the lesson and course roles and derives each
// note's domain from the folder under Writing/lessons, so a course and the
// lessons beside it share a domain without either writing one.
const unlistedContract = `schema_version = "2"

[folders]
checked = ["Maps", "Writing"]
templates = []
domain_from = ["Writing/lessons"]

[enums]

[fields]
common = ["type", "draft"]
required = ["type"]

[types.lesson]
role = "lesson"
fields = ["slug"]
slug_pattern = '^[a-z]+$'

[types.study-path]
role = "course"

[privacy]
never_egress_dirs = []
`

// unlistedDir is the japanese domain's folder, where the courses and lessons
// below are written.
const unlistedDir = "Writing/lessons/japanese/"

// TestUnlistedLessonExemptsOnlyADraft holds that map.disk_unlisted answers from
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
			write(t, root, schema.ContractRelPath, unlistedContract)
			write(t, root, unlistedDir+"Japanese Path.md",
				"---\ntype: study-path\n---\n# Japanese Path\n\n## Main {sequence=primary}\n\n- [[Listed]]\n")
			write(t, root, unlistedDir+"Listed.md", "---\ntype: lesson\nslug: listed\n---\nbody\n")
			write(t, root, unlistedDir+"Unlisted.md", "---\ntype: lesson\nslug: unlisted\n"+tt.draft+"---\nbody\n")

			out := string(runCheck(t, root))
			got := strings.Contains(out, `"rule_id":"map.disk_unlisted","severity":"warn","path":"`+unlistedDir+`Unlisted.md"`)
			if got != tt.want {
				t.Errorf("map.disk_unlisted for an unlisted lesson with %q = %t, want %t; findings:\n%s",
					tt.draft, got, tt.want, out)
			}
		})
	}
}

// TestDiskUnlistedUsesTheDomainPathUnion holds that map.disk_unlisted
// answers from the union of a domain's study paths, not from each path
// walking the domain alone. Two paths that each list half the lessons of one
// domain produce no finding: those lessons are listed, just not on every path.
func TestDiskUnlistedUsesTheDomainPathUnion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, unlistedContract)
	write(t, root, unlistedDir+"Path A.md",
		"---\ntype: study-path\n---\n# Path A\n\n## Main {sequence=primary}\n\n- [[Lesson A]]\n")
	write(t, root, unlistedDir+"Path B.md",
		"---\ntype: study-path\n---\n# Path B\n\n## Main {sequence=primary}\n\n- [[Lesson B]]\n")
	write(t, root, unlistedDir+"Lesson A.md", "---\ntype: lesson\nslug: lessona\n---\nbody\n")
	write(t, root, unlistedDir+"Lesson B.md", "---\ntype: lesson\nslug: lessonb\n---\nbody\n")

	out := string(runCheck(t, root))
	if strings.Contains(out, `"rule_id":"map.disk_unlisted"`) {
		t.Errorf("map.disk_unlisted fired for lessons listed on a parallel path of the same domain; findings:\n%s", out)
	}
}

// TestDiskUnlistedIsOneFindingPerLesson holds that an unlisted lesson is one
// domain fact, not one row per study path of that domain. Two paths that list
// none of a lesson produce one finding, attached to the first path in check
// order — the file the reader opens next to add the lesson.
func TestDiskUnlistedIsOneFindingPerLesson(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, unlistedContract)
	write(t, root, unlistedDir+"Path A.md",
		"---\ntype: study-path\n---\n# Path A\n\n## Main {sequence=primary}\n\n- [[Listed]]\n")
	write(t, root, unlistedDir+"Path B.md",
		"---\ntype: study-path\n---\n# Path B\n\n## Main {sequence=primary}\n\n- [[Listed]]\n")
	write(t, root, unlistedDir+"Listed.md", "---\ntype: lesson\nslug: listed\n---\nbody\n")
	write(t, root, unlistedDir+"Unlisted.md", "---\ntype: lesson\nslug: unlisted\n---\nbody\n")

	out := string(runCheck(t, root))
	if n := strings.Count(out, `"rule_id":"map.disk_unlisted"`); n != 1 {
		t.Errorf("map.disk_unlisted reported %d times, want 1 for one unlisted lesson; findings:\n%s", n, out)
	}
	if !strings.Contains(out, `"path":"`+unlistedDir+`Unlisted.md"`) {
		t.Errorf("the finding did not name the unlisted lesson; findings:\n%s", out)
	}
	if !strings.Contains(out, "not listed in syllabus "+unlistedDir+"Path A.md") {
		t.Errorf("the finding did not attach to the first path in check order; findings:\n%s", out)
	}
	if strings.Contains(out, "not listed in syllabus "+unlistedDir+"Path B.md") {
		t.Errorf("the finding also attached to a later path of the same domain; findings:\n%s", out)
	}
}
