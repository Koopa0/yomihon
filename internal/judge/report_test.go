package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

// TestReportGolden asserts the human and markdown renderings of the report
// fixture equal their goldens byte for byte. The human golden is the reference
// tool's exact output; the markdown golden is that output with only its two
// tool-identity lines changed to name yomihon, the sole place the format
// deliberately departs from the reference (see the tool identity in
// markdownReport).
func TestReportGolden(t *testing.T) {
	t.Parallel()
	findings, err := Check(t.Context(), "testdata/vault-report")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// The roots come from the fixture's own contract, the way the command
	// takes them, so this golden is produced by the declaration under test
	// rather than by a list written beside it.
	contract, err := schema.Load("testdata/vault-report")
	if err != nil {
		t.Fatalf("schema.Load: %v", err)
	}
	roots := domainRoots(contract.Definition().Rules.DomainEqualsFolderUnder)
	if len(roots) == 0 {
		t.Fatal("the report fixture declares no domain roots, so a grouping test over it would prove nothing")
	}
	tests := []struct {
		name   string
		got    []byte
		golden string
	}{
		{name: "human", got: []byte(humanReport(findings, roots)), golden: "testdata/golden/report-human.golden"},
		{name: "markdown", got: []byte(markdownReport(findings, roots)), golden: "testdata/golden/report-md.golden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want, err := os.ReadFile(tt.golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if !bytes.Equal(tt.got, want) {
				t.Errorf("%s report differs from golden %s\ngot:\n%s\nwant:\n%s\ngot hex:\n%s\nwant hex:\n%s",
					tt.name, tt.golden, tt.got, want, hex.Dump(tt.got), hex.Dump(want))
			}
		})
	}
}
