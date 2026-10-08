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
// tool's exact output. The markdown golden is that view with a preamble this
// fixture's contract accepts: it does not declare type report, so the body
// opens by saying so rather than with frontmatter the vault's own check would
// reject.
func TestReportGolden(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fixture string
		format  Format
		golden  string
	}{
		{name: "human", fixture: "testdata/vault-report", format: FormatHuman, golden: "testdata/golden/report-human.golden"},
		{name: "markdown", fixture: "testdata/vault-report", format: FormatMarkdown, golden: "testdata/golden/report-md.golden"},
		{name: "hidden human", fixture: "testdata/vault-report-hidden", format: FormatHuman, golden: "testdata/golden/report-hidden-human.golden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			findings, err := Check(t.Context(), tt.fixture)
			if err != nil {
				t.Fatalf("Check(%q): %v", tt.fixture, err)
			}
			contract, err := schema.Load(tt.fixture)
			if err != nil {
				t.Fatalf("schema.Load(%q): %v", tt.fixture, err)
			}
			roots := domainRoots(contract.Definition().Rules.DomainEqualsFolderUnder)
			if len(roots) == 0 {
				t.Fatal("the report fixture declares no domain roots, so a grouping test over it would prove nothing")
			}
			got := []byte(humanReport(findings, roots))
			if tt.format == FormatMarkdown {
				got = []byte(markdownReport(findings, roots, contract))
			}
			want, err := os.ReadFile(tt.golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			t.Logf("invoked: %s report golden comparison", tt.name)
			if !bytes.Equal(got, want) {
				t.Errorf("caught: %s report differs from golden %s\ngot:\n%s\nwant:\n%s\ngot hex:\n%s\nwant hex:\n%s",
					tt.name, tt.golden, got, want, hex.Dump(got), hex.Dump(want))
			}
		})
	}
}
