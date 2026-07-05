package judge

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

// TestReportGolden asserts the human and markdown renderings of the report
// fixture equal their goldens byte for byte. The human golden is the reference
// tool's exact output; the markdown golden is that output with only its two
// tool-identity lines changed to name kurodo, the sole place the format
// deliberately departs from the reference (see the tool identity in
// markdownReport).
func TestReportGolden(t *testing.T) {
	t.Parallel()
	findings, err := Check("testdata/vault-report")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	tests := []struct {
		name   string
		got    []byte
		golden string
	}{
		{name: "human", got: []byte(humanReport(findings)), golden: "testdata/golden/report-human.golden"},
		{name: "markdown", got: []byte(markdownReport(findings)), golden: "testdata/golden/report-md.golden"},
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
