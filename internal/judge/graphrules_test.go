package judge

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// unlistedSourceKindContract declares the lesson and study-path types, and
// names both a declared source_kind and the old invented curriculum-gap value.
// Declaring the invented word is the worse case: there is no schema.enum to
// notice, only whether map.disk_unlisted still runs. Its lessons start at
// "draft" because that is the word this vault happens to use; the exemption
// follows the row, not the spelling, which the initial-status test holds.
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
// answers from listing and from the status a lesson starts at, not from a
// source_kind word. A ready lesson the syllabus does not list is reported when
// it carries a declared kind; the curriculum-gap-carrying shape is the same
// case, not a second way to say "not yet". The one exemption is the status the
// lifecycle starts a lesson at, which this contract spells "draft".
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
			name:       "a lesson at the status the lifecycle starts one at stays unreported",
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

// unlistedSeedContract starts a lesson at "seed" and names the initial key
// nowhere, so which status a lesson is given first is read off the row that
// names no predecessor. It declares no status spelled "draft" at all.
const unlistedSeedContract = `schema_version = "1"

[enums]
type = ["lesson", "study-path"]

[enums.status]
note = ["seed", "ready"]
lesson = ["seed", "ready"]

[fields]
required = ["title", "type", "status"]
known = ["title", "type", "status", "domain"]
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
status = "seed"
applies_to = ["*"]
from = []
owner = ["koopa"]

[[lifecycle]]
status = "ready"
applies_to = ["*"]
from = ["seed"]
owner = ["koopa"]
`

// unlistedSeedDeclaredContract starts its lessons at the same word and writes
// the initial key down on every row, so the same question is put to a contract
// that states the answer rather than one it is read off.
const unlistedSeedDeclaredContract = `schema_version = "1"

[enums]
type = ["lesson", "study-path"]

[enums.status]
note = ["seed", "ready"]
lesson = ["seed", "ready"]

[fields]
required = ["title", "type", "status"]
known = ["title", "type", "status", "domain"]
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
status = "seed"
applies_to = ["*"]
initial = true
from = []
owner = ["koopa"]

[[lifecycle]]
status = "ready"
applies_to = ["*"]
initial = false
from = ["seed"]
owner = ["koopa"]
`

// TestUnlistedLessonExemptsTheStatusALessonStartsAt holds that the single
// exemption map.disk_unlisted grants comes from the vault's own lifecycle and
// not from a word this package knows. Under a contract that starts its lessons
// at "seed", a seed lesson no syllabus lists stays quiet and a lesson spelled
// "draft" — a status that contract never declares — is reported like any
// other. Both answers hold whether the contract writes the initial key down or
// leaves it to be read off the row naming no predecessor.
func TestUnlistedLessonExemptsTheStatusALessonStartsAt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		contract string
		status   string
		want     bool
	}{
		{
			name:     "the inferred first status is exempt",
			contract: unlistedSeedContract,
			status:   "seed",
			want:     false,
		},
		{
			name:     "a word this contract never declares is not exempt",
			contract: unlistedSeedContract,
			status:   "draft",
			want:     true,
		},
		{
			name:     "a status past the first one is not exempt",
			contract: unlistedSeedContract,
			status:   "ready",
			want:     true,
		},
		{
			name:     "the declared first status is exempt",
			contract: unlistedSeedDeclaredContract,
			status:   "seed",
			want:     false,
		},
		{
			name:     "a word a declaring contract never declares is not exempt",
			contract: unlistedSeedDeclaredContract,
			status:   "draft",
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, schema.ContractRelPath, tt.contract)
			write(t, root, "Maps/Japanese Path.md",
				"---\ntitle: Japanese Path\ntype: study-path\nstatus: ready\ndomain: japanese\n---\n\n"+
					"## Main {sequence=primary}\n\n- [[Listed]]\n")
			write(t, root, "Writing/Listed.md",
				"---\ntitle: Listed\ntype: lesson\nstatus: ready\ndomain: japanese\nslug: listed\n---\nbody\n")
			write(t, root, "Writing/Unlisted.md",
				"---\ntitle: Unlisted\ntype: lesson\nstatus: "+tt.status+
					"\ndomain: japanese\nslug: unlisted\n---\nbody\n")

			out := string(runCheck(t, root))
			got := strings.Contains(out, `"rule_id":"map.disk_unlisted"`) &&
				strings.Contains(out, `"path":"Writing/Unlisted.md"`)
			if got != tt.want {
				t.Errorf("map.disk_unlisted for an unlisted lesson at status=%q = %t, want %t; findings:\n%s",
					tt.status, got, tt.want, out)
			}
		})
	}
}
