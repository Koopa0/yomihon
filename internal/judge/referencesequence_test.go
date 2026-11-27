package judge

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestReferenceSequenceShapes(t *testing.T) {
	t.Parallel()
	contract, err := schema.Load("testdata/vault-reference-sequence")
	if err != nil {
		t.Fatalf("schema.Load() error = %v", err)
	}
	for _, tt := range []struct {
		name   string
		fields string
		want   []string
	}{
		{name: "flow", fields: "based_on: [[Note]]\n", want: []string{"based_on"}},
		{name: "block", fields: "based_on:\n  - [Note]\n", want: []string{"based_on"}},
		{name: "mixed", fields: "based_on: [\"[[Good]]\", [Bad]]\n", want: []string{"based_on"}},
		{name: "several nested items once", fields: "based_on: [[One], [Two], [[Three]]]\n", want: []string{"based_on"}},
		{name: "both fixed references", fields: "based_on: [[One]]\nrelated: [[Two]]\n", want: []string{"based_on", "related"}},
		{name: "a replacement ledger field is not a reference", fields: "lineage: [[Note]]\n"},
		{name: "alias item", fields: "source_locator: &nested [Note]\nbased_on: [*nested]\n", want: []string{"based_on"}},
		{name: "alias outer list", fields: "source_locator: &nested [[Note]]\nbased_on: *nested\n", want: []string{"based_on"}},
		{name: "quoted flow", fields: "based_on: [\"[[Note]]\", \"Note\"]\n"},
		{name: "quoted scalar", fields: "based_on: \"[[Note]]\"\n"},
		{name: "quoted block", fields: "based_on:\n  - '[[Note]]'\n"},
		{name: "flat list", fields: "based_on: [One, Two]\n"},
		{name: "mapping item", fields: "based_on: [{name: Note}]\n"},
		{name: "locator excluded", fields: "source_locator: [[Note]]\n"},
		{name: "predecessor and successors are not references", fields: "predecessor: [[One]]\nsuccessors: [[Two]]\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := []byte("---\ntype: concept\n" + tt.fields + "---\n\nReadable body.\n")
			findings := lintFrontmatter("Concepts/Probe.md", data, contract)
			var got []string
			for _, finding := range findings {
				if finding.RuleID == "schema.reference_nested_sequence" {
					if finding.Field == nil {
						t.Fatal("reference finding has no field")
					}
					got = append(got, *finding.Field)
				}
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("lintFrontmatter() reference fields (-want +got):\n%s", diff)
			}
		})
	}
}
