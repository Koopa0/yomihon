package judge

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestHiddenFindingsFollowScopeAndBaseline holds which findings a check keeps
// over the hidden-report fixture: all brings back the notes outside the
// knowledge layer, a path keeps its own, and a baseline subtracts every line
// whose fingerprint it names, a repeated one included.
func TestHiddenFindingsFollowScopeAndBaseline(t *testing.T) {
	t.Parallel()
	const (
		missing      = "warn link.broken Writing/golang/A.md:9"
		futureA3     = "info link.broken Writing/golang/A.md:3"
		futureA4     = "info link.broken Writing/golang/A.md:4"
		outsideA     = "info link.broken.path Writing/golang/A.md:8"
		futureB      = "info link.broken Writing/golang/B.md:3"
		outsideB     = "info link.broken.path Writing/golang/B.md:7"
		elsewhere    = "info link.broken Other/Outside.md:3"
		outsideOther = "info link.broken.path Other/Outside.md:7"
	)
	for _, tt := range []struct {
		name     string
		all      bool
		paths    []string
		baseline []string
		want     []string
	}{
		{name: "default", want: []string{futureA3, futureA4, outsideA, missing, futureB, outsideB}},
		{name: "all", all: true, want: []string{elsewhere, outsideOther, futureA3, futureA4, outsideA, missing, futureB, outsideB}},
		{name: "path", all: true, paths: []string{"Writing/golang/B.md"}, want: []string{futureB, outsideB}},
		{
			name:     "a repeated fingerprint",
			baseline: []string{"v1:5afd0b44ba074aae"},
			want:     []string{outsideA, missing, futureB, outsideB},
		},
		{
			name:     "every info fingerprint",
			baseline: []string{"v1:5afd0b44ba074aae", "v1:85c048abb2259e9b", "v1:2fb3f6a685f5135d", "v1:f9fe2e12f0b554ce"},
			want:     []string{missing},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			o := &CheckOptions{Root: "testdata/vault-report-hidden", All: tt.all, Paths: tt.paths, Format: FormatJSON}
			if tt.baseline != nil {
				var lines strings.Builder
				for _, fp := range tt.baseline {
					lines.WriteString(`{"fingerprint":"` + fp + "\"}\n")
				}
				o.Baseline = filepath.Join(t.TempDir(), "baseline.jsonl")
				if err := os.WriteFile(o.Baseline, []byte(lines.String()), 0o600); err != nil {
					t.Fatalf("write baseline: %v", err)
				}
			}
			out, exit, err := checkBytes(t.Context(), o)
			if err != nil {
				t.Fatalf("RunCheck(%+v): %v", o, err)
			}
			if exit != 0 {
				t.Errorf("RunCheck(%+v) exit = %d, want 0", o, exit)
			}
			var got []string
			for _, f := range decodeFindings(t, out) {
				got = append(got, f.Severity.String()+" "+string(f.RuleID)+" "+f.Path+":"+strconv.Itoa(*f.Line))
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("RunCheck(%+v) findings (-want +got):\n%s", o, diff)
			}
		})
	}
}

// TestPackCountsOnlyForwardReferencesAsPlanned asserts the hidden count is
// split by what each informational finding is: a tracked forward-reference, an
// external path, or neither.
func TestPackCountsOnlyForwardReferencesAsPlanned(t *testing.T) {
	t.Parallel()
	findings := []Finding{
		{RuleID: "link.broken", Severity: SeverityInfo, Path: "a.md"},
		{RuleID: "course.disk_mismatch", Severity: SeverityInfo, Path: "a.md"},
		{RuleID: "link.broken.path", Severity: SeverityInfo, Path: "a.md"},
		{RuleID: "link.unused.definition", Severity: SeverityInfo, Path: "a.md"},
		{RuleID: "callout.title_markup", Severity: SeverityInfo, Path: "a.md"},
		{RuleID: "link.broken", Severity: SeverityWarn, Path: "a.md"},
	}
	p := pack(findings, nil)
	if p.planned != 2 || p.external != 1 || p.otherInfo != 2 {
		t.Errorf("pack(findings) planned, external, other = %d, %d, %d, want 2, 1, 2", p.planned, p.external, p.otherInfo)
	}
}
