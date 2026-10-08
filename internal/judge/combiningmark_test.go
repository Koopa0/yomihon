package judge

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFragmentMarksRemainDistinct(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, heading, missing string }{
		{"Hindi spacing mark", "की", "कि"},
		{"Thai nonspacing mark", "ปู่", "ปู"},
		{"Kana combining mark", "か\u309a", "か"},
		{"supplementary variation selector", "葛\U000e0100城", "葛城"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, syntax := range []struct{ prefix, rule string }{
				{"", "link.section_missing"},
				{"!", "embed.section_missing"},
			} {
				got := ruleTargets(fragmentRun(t, map[string]string{
					"Notes/Target.md": "## " + tt.heading + "\n\nPassage.\n",
					"Notes/Citer.md":  syntax.prefix + "[[Target#" + tt.heading + "]]\n" + syntax.prefix + "[[Target#" + tt.missing + "]]\n",
				}))
				want := []string{syntax.rule + " Target#" + tt.missing}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("%s fragment findings mismatch (-want +got):\n%s", syntax.rule, diff)
				}
			}
		})
	}
}

func TestFragmentNormalizationRunCheck(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, target, citer string
		exit                int
		want                []string
	}{
		{name: "canonical heading and block", target: "## Y\u030a\n\nPASSAGE\n\nQuote. ^QUOTE-1\n", citer: "[[N#\u1e99]]\n![[N#\u1e99]]\n[[N#y\u030a]]\n![[N#y\u030a]]\n[[N#^quote-1]]\n![[N#^quote-1]]\n"},
		{name: "canonical heading path", target: "## Y\u030a\n### Y\u030a\nPASSAGE\n", citer: "[[N#\u1e99#\u1e99]]\n![[N#\u1e99#\u1e99]]\n"},
		{name: "generous link and exact embed", target: "```\n## Y\u030a\n```\n", citer: "[[N#\u1e99]]\n![[N#\u1e99]]\n", exit: 1, want: []string{"embed.section_missing N#\u1e99"}},
		{name: "missing sections and blocks", target: "## Y\u030a\n\nQuote. ^QUOTE-1\n", citer: "[[N#Absent]]\n![[N#Absent]]\n[[N#^quote1]]\n![[N#^quote1]]\n", exit: 1, want: []string{"embed.block_missing N#^quote1", "embed.section_missing N#Absent", "link.block_missing N#^quote1", "link.section_missing N#Absent"}},
		{name: "hidden and fenced excerpts", target: "%%\n## Y\u030a\n%%\n\n```\n## Fenced\n```\n", citer: "![[N#\u1e99]]\n![[N#Fenced]]\n", exit: 1, want: []string{"embed.section_missing N#Fenced", "embed.section_missing N#\u1e99"}},
		{name: "closed parent", target: "## Y\u030a\n## Other\n### Child\n", citer: "[[N#\u1e99#Child]]\n![[N#\u1e99#Child]]\n", exit: 1, want: []string{"embed.section_missing N#\u1e99#Child", "link.section_missing N#\u1e99#Child"}},
		{name: "resource and ambiguous scope", target: "## Y\u030a\n", citer: "[[Asset.txt#\u1e99]]\n[[Twin#\u1e99]]\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, "Concepts/N.md", tt.target)
			write(t, root, "Concepts/Citer.md", tt.citer)
			write(t, root, "Concepts/Asset.txt", "resource\n")
			write(t, root, "Concepts/A/Twin.md", "one\n")
			write(t, root, "Concepts/B/Twin.md", "two\n")
			writeTestContract(t, root, nil)
			out, exit, err := RunCheck(t.Context(), &CheckOptions{Root: root, Format: FormatJSON, Deny: []string{"link.section_missing", "embed.section_missing", "link.block_missing", "embed.block_missing"}})
			if err != nil {
				t.Fatalf("RunCheck fixture error = %v", err)
			}
			if exit != tt.exit {
				t.Errorf("caught: fragment-normalization RunCheck exit = %d, want %d; output:\n%s", exit, tt.exit, out)
			}
			var got []string
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
					t.Fatalf("decode RunCheck fixture output error = %v", err)
				}
				switch finding.RuleID {
				case "link.section_missing", "embed.section_missing", "link.block_missing", "embed.block_missing":
					got = append(got, finding.RuleID+" "+finding.Target)
				}
			}
			slices.Sort(got)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: fragment-normalization RunCheck findings (-want +got):\n%s", diff)
			}
		})
	}
}
