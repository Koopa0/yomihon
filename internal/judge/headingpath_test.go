package judge

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHeadingPathFindingsMatchDisplayedAncestors(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, target, citer string
		want                []string
	}{
		{name: "child", target: "## A\n### B\n", citer: "[[N#A#B]]"},
		{name: "skipped level", target: "## A\n###### B\n", citer: "[[N#A#B]]"},
		{name: "omitted middle", target: "## A\n### Middle\n#### B\n", citer: "[[N#A#B]]"},
		{name: "second parent", target: "## First\n### B\n## A\n### B\n", citer: "[[N#A#B]]"},
		{name: "missing child", target: "## A\n### B\n## Other\n### C\n", citer: "[[N#A#C]]", want: []string{"link.section_missing N#A#C"}},
		{name: "closed parent", target: "## A\n## Other\n### B\n", citer: "[[N#A#B]]", want: []string{"link.section_missing N#A#B"}},
		{name: "reversed ancestors", target: "## A\n### Middle\n#### B\n", citer: "[[N#Middle#A#B]]", want: []string{"link.section_missing N#Middle#A#B"}},
		{name: "quote", target: "> ## A\n> ### B\n", citer: "[[N#A#B]]"},
		{name: "inline display", target: "## *A*\n### [B](https://example.invalid/)\n", citer: "[[N#A#B]]"},
		{name: "one level embed", target: "## A\n![[Child]]\n", citer: "[[N#A#B]]"},
		{name: "outside embed cut", target: "## A\n![[Child#C]]\n", citer: "[[N#A#B]]", want: []string{"link.section_missing N#A#B"}},
		{name: "path embed", target: "## A\n### B\n", citer: "![[N#A#B]]"},
		{name: "missing path embed", target: "## A\n### B\n", citer: "![[N#A#C]]", want: []string{"embed.section_missing N#A#C"}},
		{name: "hidden child", target: "## A\n%%\n### B\n%%\n", citer: "[[N#A#B]]", want: []string{"link.section_missing N#A#B"}},
		{name: "fenced child", target: "## A\n```\n### B\n```\n", citer: "[[N#A#B]]", want: []string{"link.section_missing N#A#B"}},
		{name: "literal trailing hash", target: "## A#\n", citer: "[[N#A#]]"},
		{name: "block wins", target: "## A\n### B\n\nwords ^real\n", citer: "[[N^real#Other#Absent]]"},
		{name: "cycle", target: "## A\n### B\n[[Citer#X#Y]]\n", citer: "## X\n### Y\n[[N#A#B]]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ruleTargets(fragmentRun(t, map[string]string{"N.md": tt.target, "Citer.md": tt.citer, "Child.md": "### B\nCHILD\n### C\nOTHER\n"}))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("fragmentFindings() (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHeadingPathRunCheckKeepsMissingRuleAndExit(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, link string
		exit       int
		want       []string
	}{
		{name: "existing child", link: "[[N#A#B]]"},
		{name: "missing child", link: "[[N#A#C]]", exit: 1, want: []string{"link.section_missing N#A#C"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, "Concepts/N.md", "## A\n### B\n")
			write(t, root, "Concepts/Citer.md", tt.link+"\n")
			writeTestContract(t, root, nil)
			out, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Deny: []string{"link.section_missing"}})
			if err != nil {
				t.Fatalf("RunCheck() error = %v", err)
			}
			if exit != tt.exit {
				t.Errorf("RunCheck() exit = %d, want %d; output:\n%s", exit, tt.exit, out)
			}
			var fragments []string
			decoder := json.NewDecoder(bytes.NewReader(out))
			for {
				var finding struct {
					RuleID string `json:"rule_id"`
					Target string `json:"target"`
				}
				err = decoder.Decode(&finding)
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatalf("decode RunCheck() output error = %v", err)
				}
				if finding.RuleID == "link.section_missing" {
					fragments = append(fragments, finding.RuleID+" "+finding.Target)
				}
			}
			if diff := cmp.Diff(tt.want, fragments); diff != "" {
				t.Errorf("RunCheck() fragment findings (-want +got):\n%s", diff)
			}
		})
	}
}
