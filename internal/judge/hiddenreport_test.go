package judge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHiddenReportZeroInfo(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		findings []Finding
		want     string
	}{
		{
			name: "empty",
			want: "0 findings: 0 error, 0 warn, 0 hidden (0 planned forward-refs, 0 external paths)\n",
		},
		{
			name:     "warning",
			findings: []Finding{{RuleID: "link.broken", Severity: SeverityWarn, Path: "Writing/golang/A.md", Message: "warning text"}},
			want: "1 findings: 0 error, 1 warn, 0 hidden (0 planned forward-refs, 0 external paths)\n" +
				"\ndebt by domain:\n  golang               0 error · 1 warn\n" +
				"\n▌ golang\n  [warn] warning text  (Writing/golang/A.md)\n",
		},
		{
			name:     "error",
			findings: []Finding{{RuleID: "schema.required", Severity: SeverityError, Path: "Writing/golang/A.md", Message: "error text"}},
			want: "1 findings: 1 error, 0 warn, 0 hidden (0 planned forward-refs, 0 external paths)\n" +
				"\ndebt by domain:\n  golang               1 error · 0 warn\n" +
				"\n▌ golang\n  [error] error text  (Writing/golang/A.md)\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, humanReport(tt.findings, domainRoots{"Writing"})); diff != "" {
				t.Errorf("humanReport(no info) bytes differ (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHiddenReportRetention(t *testing.T) {
	t.Parallel()
	golden, err := os.ReadFile("testdata/golden/report-hidden-human.golden")
	if err != nil {
		t.Fatalf("read hidden human golden: %v", err)
	}
	for _, tt := range []struct {
		name  string
		all   bool
		paths []string
		want  string
	}{
		{name: "default", want: string(golden)},
		{
			name: "all", all: true,
			want: "8 findings: 0 error, 1 warn, 7 hidden (4 planned forward-refs, 3 external paths)\n" +
				"\ndebt by domain:\n  golang               0 error · 1 warn\n" +
				"\nmost leveraged (create one, resolve many):\n  ×3 [[Future]] (planned) — golang\n" +
				"\n▌ golang\n  [warn] [[Missing]] resolves to no note  (Writing/golang/A.md)\n" +
				"\nhidden (info):\n" +
				"  [link.broken] [[Elsewhere]] resolves to no note — Other/Outside.md:3\n" +
				"  [link.broken.path] link to ../../outside-other.md points outside the vault root — Other/Outside.md:7\n" +
				"  [link.broken] [[Future]] resolves to no note — Writing/golang/A.md:3\n" +
				"  [link.broken] [[Future]] resolves to no note — Writing/golang/A.md:4\n" +
				"  [link.broken.path] link to ../../../outside.md points outside the vault root — Writing/golang/A.md:8\n" +
				"  [link.broken] [[Future]] resolves to no note — Writing/golang/B.md:3\n" +
				"  [link.broken.path] link to ../../../outside.md points outside the vault root — Writing/golang/B.md:7\n",
		},
		{
			name: "path", all: true, paths: []string{"Writing/golang/B.md"},
			want: "2 findings: 0 error, 0 warn, 2 hidden (1 planned forward-refs, 1 external paths)\n" +
				"\nhidden (info):\n" +
				"  [link.broken] [[Future]] resolves to no note — Writing/golang/B.md:3\n" +
				"  [link.broken.path] link to ../../../outside.md points outside the vault root — Writing/golang/B.md:7\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hiddenReportOutput(t, &CheckOptions{Root: "testdata/vault-report-hidden", All: tt.all, Paths: tt.paths, Format: FormatHuman}, tt.want)
		})
	}
}

func TestHiddenReportBaseline(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name         string
		fingerprints []string
		want         string
	}{
		{
			name:         "repeated identity",
			fingerprints: []string{"v1:5afd0b44ba074aae"},
			want: "4 findings: 0 error, 1 warn, 3 hidden (1 planned forward-refs, 2 external paths)\n" +
				"\ndebt by domain:\n  golang               0 error · 1 warn\n" +
				"\n▌ golang\n  [warn] [[Missing]] resolves to no note  (Writing/golang/A.md)\n" +
				"\nhidden (info):\n" +
				"  [link.broken.path] link to ../../../outside.md points outside the vault root — Writing/golang/A.md:8\n" +
				"  [link.broken] [[Future]] resolves to no note — Writing/golang/B.md:3\n" +
				"  [link.broken.path] link to ../../../outside.md points outside the vault root — Writing/golang/B.md:7\n",
		},
		{
			name:         "no retained info",
			fingerprints: []string{"v1:5afd0b44ba074aae", "v1:85c048abb2259e9b", "v1:2fb3f6a685f5135d", "v1:f9fe2e12f0b554ce"},
			want: "1 findings: 0 error, 1 warn, 0 hidden (0 planned forward-refs, 0 external paths)\n" +
				"\ndebt by domain:\n  golang               0 error · 1 warn\n" +
				"\n▌ golang\n  [warn] [[Missing]] resolves to no note  (Writing/golang/A.md)\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var baseline strings.Builder
			for _, fp := range tt.fingerprints {
				baseline.WriteString("{\"fingerprint\":\"" + fp + "\"}\n")
			}
			path := filepath.Join(t.TempDir(), "baseline.jsonl")
			if err := os.WriteFile(path, []byte(baseline.String()), 0o600); err != nil {
				t.Fatalf("write baseline: %v", err)
			}
			hiddenReportOutput(t, &CheckOptions{Root: "testdata/vault-report-hidden", Baseline: path, Format: FormatHuman}, tt.want)
		})
	}
}

func hiddenReportOutput(t *testing.T, options *CheckOptions, want string) {
	t.Helper()
	stdout, exit, err := RunCheck(t.Context(), options)
	if err != nil {
		t.Fatalf("RunCheck(hidden report): %v", err)
	}
	if exit != 0 {
		t.Fatalf("RunCheck(hidden report) exit = %d, want 0", exit)
	}
	if diff := cmp.Diff(want, string(stdout)); diff != "" {
		t.Errorf("caught: retained hidden report bytes differ (-want +got):\n%s", diff)
	}
}
