package judge

import (
	"bytes"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// atlasContract declares a vault whose maps are called "atlas". A vault is free
// to name its map types anything its contract lists, so the coverage face has
// to read the vocabulary from the contract rather than recognise a fixed set.
const atlasContract = `schema_version = "1"

[enums]
type = ["concept", "atlas"]

[enums.status]
note = ["draft", "ready"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status", "based_on"]

[rules]
concept_requires_provenance = ["based_on"]

[scan]
knowledge_dirs = ["Concepts", "Atlases"]
skip_basenames = []

[navigation]
path_types = []
map_types = ["atlas"]

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

// noConceptContract declares a vault with no concept type at all: its notes are
// called "note" and its maps "atlas". Nothing in it can be a concept, so the
// concept tally has nothing to be about.
const noConceptContract = `schema_version = "1"

[enums]
type = ["note", "atlas"]

[enums.status]
note = ["draft", "ready"]

[fields]
required = ["title", "type"]
known = ["title", "type", "status"]

[scan]
knowledge_dirs = ["Notes", "Atlases"]
skip_basenames = []

[navigation]
path_types = []
map_types = ["atlas"]

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

// TestCoverageMountsOnContractMapTypes asserts a concept is mounted by an edge
// from the type its own contract declares as a map. The vault below calls its
// maps "atlas"; a face that recognised a fixed list of map type names would
// call the mounted concept unmounted and report a gap the vault does not have.
func TestCoverageMountsOnContractMapTypes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, atlasContract)
	write(t, root, "Concepts/Slice.md", "---\ntitle: Slice\ntype: concept\nbased_on:\n  - \"[[Go Atlas]]\"\n---\n\nBody.\n")
	write(t, root, "Atlases/Go Atlas.md", "---\ntitle: Go Atlas\ntype: atlas\n---\n\n- [[Slice]]\n")

	got, exit, err := RunCoverage(&CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage() error = %v", err)
	}
	if exit != 0 {
		t.Errorf("RunCoverage() exit = %d, want 0", exit)
	}
	want := `{"total_concepts":1,"domains":[{"domain":"(none)","concepts":1,"mounted":1,"pending_mount":0,"orphan":0}],"pending_mount":[],"orphans":[],"unrouted":[]}` + "\n"
	if string(got) != want {
		t.Errorf("RunCoverage() =\n%s\nwant\n%s", got, want)
	}
}

// TestCoverageWithholdsTheRouteFromAnUndeclaredType asserts a vault whose
// contract never names the research brief is not told to file one under the
// index note of the vault this route was written for. The route cannot be
// derived from a contract, so it is offered only where the contract declares
// the type it is about.
func TestCoverageWithholdsTheRouteFromAnUndeclaredType(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, atlasContract)
	write(t, root, "Concepts/Brief.md", "---\ntitle: Brief\ntype: research-brief\n---\n\nBody.\n")

	got, _, err := RunCoverage(&CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage() error = %v", err)
	}
	if !bytes.Contains(got, []byte(`"unrouted":[]`)) {
		t.Errorf("a vault that never declared the type was given a route:\n%s", got)
	}
	if bytes.Contains(got, []byte("Brief")) {
		t.Errorf("coverage named a note of a type the contract does not declare:\n%s", got)
	}
}

// TestCoverageSaysSoWhenNoConceptTypeIsDeclared asserts a vault whose contract
// names no concept type is told the concept tally does not apply to it, rather
// than handed a zero tally that reads as a clean vault.
func TestCoverageSaysSoWhenNoConceptTypeIsDeclared(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, schema.ContractRelPath, noConceptContract)
	write(t, root, "Notes/Alpha.md", "---\ntitle: Alpha\ntype: note\n---\n\nBody.\n")
	write(t, root, "Atlases/Go Atlas.md", "---\ntitle: Go Atlas\ntype: atlas\n---\n\n- [[Alpha]]\n")

	got, exit, err := RunCoverage(&CoverageOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCoverage() error = %v", err)
	}
	if exit != 0 {
		t.Errorf("RunCoverage() exit = %d, want 0", exit)
	}
	want := `{"total_concepts":0,"domains":[],"pending_mount":[],"orphans":[],"unrouted":[],` +
		`"not_applicable":"contract declares no \"concept\" type, so there is no concept corpus to judge"}` + "\n"
	if string(got) != want {
		t.Errorf("RunCoverage() =\n%s\nwant\n%s", got, want)
	}

	human, _, err := RunCoverage(&CoverageOptions{Root: root, Format: FormatHuman})
	if err != nil {
		t.Fatalf("RunCoverage(human) error = %v", err)
	}
	wantHuman := "contract declares no \"concept\" type, so there is no concept corpus to judge\n"
	if string(human) != wantHuman {
		t.Errorf("RunCoverage(human) = %q, want %q", human, wantHuman)
	}
}
