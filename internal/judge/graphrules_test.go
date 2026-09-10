package judge

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// unlistedSourceKindContract declares the lesson and study-path types, and
// names both a declared source_kind and the old invented curriculum-gap value.
// Declaring the invented word is the worse case: there is no schema.enum to
// notice, only whether map.disk_unlisted still runs.
const unlistedSourceKindContract = `schema_version = "1"

[enums]
type = ["lesson", "study-path"]
source_kind = ["book", "curriculum-gap"]

[enums.status]
note = ["draft", "ready"]
lesson = ["draft", "ready"]

[fields]
required = ["title", "type", "status"]
known = ["title", "type", "status", "domain", "source_kind"]
lesson_only = ["slug"]

[fields.status_group]
lesson = ["lesson"]

[rules]
slug_pattern = "^[a-z]+$"

[scan]
knowledge_dirs = ["Maps", "Writing"]
skip_basenames = []

[navigation]
path_types = ["study-path"]
map_types = []

[artifacts]
non_instance_dirs = []

[privacy]
never_egress_dirs = []

[[lifecycle]]
status = "draft"
applies_to = ["*"]
from = []
owner = ["koopa"]

[[lifecycle]]
status = "ready"
applies_to = ["*"]
from = ["draft"]
owner = ["koopa"]
`

// TestUnlistedLessonIsReportedForAnySourceKind holds that map.disk_unlisted
// answers from listing and draft status, not from a source_kind word. A ready
// lesson the syllabus does not list is reported when it carries a declared
// kind; the curriculum-gap-carrying shape is the same case, not a second way
// to say "not yet". A draft stays quiet, which is the one declared exemption.
func TestUnlistedLessonIsReportedForAnySourceKind(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     string
		sourceKind string
		want       bool
	}{
		{
			name:       "a ready lesson with a declared source kind",
			status:     "ready",
			sourceKind: "book",
			want:       true,
		},
		{
			name:       "a ready lesson carrying curriculum-gap",
			status:     "ready",
			sourceKind: "curriculum-gap",
			want:       true,
		},
		{
			name:       "a draft lesson stays unreported",
			status:     "draft",
			sourceKind: "book",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath, unlistedSourceKindContract)
			write(t, root, "Maps/Japanese Path.md",
				"---\ntitle: Japanese Path\ntype: study-path\nstatus: ready\ndomain: japanese\n---\n\n"+
					"## Main {sequence=primary}\n\n- [[Listed]]\n")
			write(t, root, "Writing/Listed.md",
				"---\ntitle: Listed\ntype: lesson\nstatus: ready\ndomain: japanese\nsource_kind: book\nslug: listed\n---\nbody\n")
			write(t, root, "Writing/Unlisted.md",
				"---\ntitle: Unlisted\ntype: lesson\nstatus: "+tt.status+
					"\ndomain: japanese\nsource_kind: "+tt.sourceKind+"\nslug: unlisted\n---\nbody\n")

			out := string(runCheck(t, root))
			got := strings.Contains(out, `"rule_id":"map.disk_unlisted"`) &&
				strings.Contains(out, `"path":"Writing/Unlisted.md"`)
			if got != tt.want {
				t.Errorf("map.disk_unlisted for status=%q source_kind=%q = %t, want %t; findings:\n%s",
					tt.status, tt.sourceKind, got, tt.want, out)
			}
		})
	}
}

// TestDiskUnlistedUsesTheDomainPathUnion holds that map.disk_unlisted
// answers from the union of a domain's study paths, not from each path
// walking the domain alone. Two paths that each list half the ready
// lessons of one domain produce no finding: those lessons are listed,
// just not on every path.
func TestDiskUnlistedUsesTheDomainPathUnion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, unlistedSourceKindContract)
	write(t, root, "Maps/Path A.md",
		"---\ntitle: Path A\ntype: study-path\nstatus: ready\ndomain: japanese\n---\n\n"+
			"## Main {sequence=primary}\n\n- [[Lesson A]]\n")
	write(t, root, "Maps/Path B.md",
		"---\ntitle: Path B\ntype: study-path\nstatus: ready\ndomain: japanese\n---\n\n"+
			"## Main {sequence=primary}\n\n- [[Lesson B]]\n")
	write(t, root, "Writing/Lesson A.md",
		"---\ntitle: Lesson A\ntype: lesson\nstatus: ready\ndomain: japanese\nsource_kind: book\nslug: lessona\n---\nbody\n")
	write(t, root, "Writing/Lesson B.md",
		"---\ntitle: Lesson B\ntype: lesson\nstatus: ready\ndomain: japanese\nsource_kind: book\nslug: lessonb\n---\nbody\n")

	out := string(runCheck(t, root))
	if strings.Contains(out, `"rule_id":"map.disk_unlisted"`) {
		t.Errorf("map.disk_unlisted fired for lessons listed on a parallel path of the same domain; findings:\n%s", out)
	}
}

// TestDiskUnlistedIsOneFindingPerLesson holds that an unlisted lesson is one
// domain fact, not one row per study path of that domain. Two paths that list
// none of a ready lesson produce one finding, attached to the first path in
// check order — the file the reader opens next to add the lesson.
func TestDiskUnlistedIsOneFindingPerLesson(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, schema.ContractRelPath, unlistedSourceKindContract)
	write(t, root, "Maps/Path A.md",
		"---\ntitle: Path A\ntype: study-path\nstatus: ready\ndomain: japanese\n---\n\n"+
			"## Main {sequence=primary}\n\n- [[Listed]]\n")
	write(t, root, "Maps/Path B.md",
		"---\ntitle: Path B\ntype: study-path\nstatus: ready\ndomain: japanese\n---\n\n"+
			"## Main {sequence=primary}\n\n- [[Listed]]\n")
	write(t, root, "Writing/Listed.md",
		"---\ntitle: Listed\ntype: lesson\nstatus: ready\ndomain: japanese\nsource_kind: book\nslug: listed\n---\nbody\n")
	write(t, root, "Writing/Unlisted.md",
		"---\ntitle: Unlisted\ntype: lesson\nstatus: ready\ndomain: japanese\nsource_kind: book\nslug: unlisted\n---\nbody\n")

	out := string(runCheck(t, root))
	if n := strings.Count(out, `"rule_id":"map.disk_unlisted"`); n != 1 {
		t.Errorf("map.disk_unlisted reported %d times, want 1 for one unlisted lesson; findings:\n%s", n, out)
	}
	if !strings.Contains(out, `"path":"Writing/Unlisted.md"`) {
		t.Errorf("the finding did not name the unlisted lesson; findings:\n%s", out)
	}
	if !strings.Contains(out, "not listed in syllabus Maps/Path A.md") {
		t.Errorf("the finding did not attach to the first path in check order; findings:\n%s", out)
	}
	if strings.Contains(out, "not listed in syllabus Maps/Path B.md") {
		t.Errorf("the finding also attached to a later path of the same domain; findings:\n%s", out)
	}
}
