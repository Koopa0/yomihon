package judge_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
)

type agreementProbe struct {
	Path       string
	Target     string
	ResolvedTo string
	Fragment   string
	Rule       judge.RuleID
	Case       int
	Absent     bool
}

func agreementWrite(t agreementTB, root, path string, data []byte) {
	t.Helper()
	tree, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("open synthetic vault: %v", err)
	}
	defer func() {
		if closeErr := tree.Close(); closeErr != nil {
			t.Errorf("close synthetic vault: %v", closeErr)
		}
	}()
	name := filepath.FromSlash(path)
	if err := tree.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		t.Fatalf("create test directory: %v", err)
	}
	if err := tree.WriteFile(name, data, 0o600); err != nil {
		t.Fatalf("write synthetic note %q: %v", path, err)
	}
}

func agreementCandidates(body string, observed *agreementHTML) []string {
	// This inventory intentionally reads tails inside code/comments too. It
	// discovers potential addresses; none of its reading grants acceptance.
	candidates := []string{"^a", "^A", "^é", "^e\u0301", "^a-2"}
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimRight(line, " \t\r")
		if at := strings.LastIndexByte(trimmed, '^'); at >= 0 {
			tail := trimmed[at:]
			if len(tail) > 1 && !strings.ContainsFunc(tail, unicode.IsSpace) && (at == 0 || trimmed[at-1] == ' ' || trimmed[at-1] == '\t') {
				candidates = append(candidates, tail)
			}
		}
	}
	candidates = append(candidates, observed.Blocks...)
	for i := range candidates {
		candidates[i] = graph.FoldFragment(candidates[i])
	}
	slices.Sort(candidates)
	return slices.Compact(candidates)
}

func agreementFragments(t *testing.T, cases []agreementCase, actual []agreementHTML) {
	t.Helper()
	for index, failures := range agreementFragmentFailures(t, cases, actual) {
		for _, failure := range failures {
			t.Errorf("caught: %s %s case=%s body=%q observations=%s", failure.Property, failure.Identity, cases[index].Name, cases[index].Body, failure.Observation)
		}
	}
}

func agreementFragmentFailures(t agreementTB, cases []agreementCase, actual []agreementHTML) [][]agreementFailure {
	t.Helper()
	failures := make([][]agreementFailure, len(cases))
	if len(cases) != len(actual) {
		t.Fatalf("fragment observation count = %d, want %d", len(actual), len(cases))
	}
	root := t.TempDir()
	contract, err := os.ReadFile("../schema/testdata/contract.toml")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	needle := `knowledge_dirs = ["Concepts", "Sources", "Maps", "Writing", "Synthesis", "Inbox"]`
	if strings.Count(string(contract), needle) != 1 {
		t.Fatal("contract knowledge declaration is not unique")
	}
	prepared := strings.Replace(string(contract), needle, `knowledge_dirs = ["Notes"]`, 1) + "\n[privacy]\nnever_egress_dirs = []\n"
	agreementWrite(t, root, schema.ContractRelPath, []byte(prepared))
	var probes []agreementProbe
	var inputs []graph.NoteInput
	for i := range cases {
		if i >= len(actual) {
			t.Fatal("fragment observation index outside captured batch")
		}
		c := &cases[i]
		observed := &actual[i]
		path := fmt.Sprintf("Notes/agree-%04d.md", i)
		inputs = append(inputs, graph.NoteInput{RelPath: path})
		agreementWrite(t, root, path, agreementEnvelope(t, ""))
		for companion := range c.Companions {
			if len(cases) != 1 {
				t.Fatal("companion controls must own one isolated batch")
			}
			inputs = append(inputs, graph.NoteInput{RelPath: companion})
			agreementWrite(t, root, companion, agreementEnvelope(t, ""))
		}
		absent := "agreement-absent-0"
		for strings.Contains(c.Body, absent) || slices.Contains(observed.Headings, absent) || slices.Contains(observed.Blocks, "^"+absent) {
			absent += "x"
		}
		blocks := append(agreementCandidates(c.Body, observed), "^"+absent)
		headings := append(slices.Clone(observed.Headings), absent)
		slices.Sort(headings)
		headings = slices.Compact(headings)
		for family, fragments := range [][]string{blocks, headings} {
			for index, fragment := range fragments {
				if strings.ContainsAny(fragment, "|#]\r\n") || strings.HasSuffix(fragment, "\\") {
					// Manufactured raw candidates that are not literal addresses do
					// not assert product behavior. An emitted id must be spellable.
					emitted := slices.Contains(observed.Blocks, fragment)
					if family == 1 {
						emitted = slices.Contains(observed.Headings, fragment)
					}
					if emitted {
						failures[i] = append(failures[i], agreementFailure{Property: fmt.Sprintf("P%d", family+3), Identity: "candidate-unspellable", Fragment: fragment, Direction: "page-unspellable", Multiplicity: 1, Observation: fmt.Sprintf("emitted fragment=%q cannot be losslessly probed", fragment)})
					}
					continue
				}
				rule := judge.RuleID("link.block_missing")
				if family == 1 {
					rule = "link.section_missing"
				}
				target := strings.TrimSuffix(path, ".md") + "#" + fragment
				probePath := fmt.Sprintf("Notes/probe-%04d-%d-%04d.md", i, family, index)
				probeBody := "[[" + target + "]]\n"
				wantTarget := strings.TrimSuffix(path, ".md")
				if diff := cmp.Diff([]string{wantTarget}, judge.LinkTargets(probeBody)); diff != "" {
					t.Fatalf("probe citation preflight: %s", diff)
				}
				link, ok := graph.ParseWikilink(target)
				if !ok || link.Target != wantTarget || (family == 0 && "^"+link.Block != fragment) || (family == 1 && link.Heading != fragment) {
					t.Fatalf("probe cannot spell literal fragment %q", fragment)
				}
				agreementWrite(t, root, probePath, agreementEnvelope(t, probeBody))
				probes = append(probes, agreementProbe{Path: probePath, Target: target, ResolvedTo: path, Fragment: fragment, Rule: rule, Case: i, Absent: fragment == absent || fragment == "^"+absent})
			}
		}
	}
	idx := graph.BuildFromNotes(inputs, nil)
	for _, probe := range probes {
		got := idx.Resolve(strings.TrimSuffix(probe.ResolvedTo, ".md"))
		if got.Kind != graph.KindUnique || got.RelPath != probe.ResolvedTo {
			t.Fatalf("probe resolution preflight: %+v", got)
		}
	}
	control := agreementMissing(t, root, probes)
	for _, probe := range probes {
		if control[probe.Path] != 1 {
			t.Fatalf("control scan lacks exact receipt: %+v count=%d", probe, control[probe.Path])
		}
	}
	for i, c := range cases {
		agreementWrite(t, root, fmt.Sprintf("Notes/agree-%04d.md", i), agreementEnvelope(t, c.Body))
		for companion, body := range c.Companions {
			agreementWrite(t, root, companion, agreementEnvelope(t, body))
		}
	}
	missing := agreementMissing(t, root, probes)
	for _, probe := range probes {
		c := cases[probe.Case]
		accepted := missing[probe.Path] == 0
		if probe.Absent && accepted {
			t.Fatalf("absent fragment accepted: %+v", probe)
		}
		if probe.Rule == "link.section_missing" {
			if !probe.Absent && !accepted {
				failures[probe.Case] = append(failures[probe.Case], agreementFailure{Property: "P4", Identity: "literal-heading-id", Fragment: probe.Fragment, Direction: "page-only", Multiplicity: 1, PagePresent: true, Observation: fmt.Sprintf("id=%q check=missing", probe.Fragment)})
			}
			continue
		}
		present := slices.Contains(actual[probe.Case].Blocks, probe.Fragment)
		excerpt, found := render.Excerpt(c.Body, probe.Fragment)
		if present != accepted || present != found {
			observations := []struct {
				name string
				has  bool
			}{{name: "page", has: present}, {name: "judge", has: accepted}, {name: "excerpt", has: found}}
			for _, observation := range observations {
				if observation.has != present {
					direction := observation.name + "-only"
					if present {
						direction = "page-not-" + observation.name
					}
					failures[probe.Case] = append(failures[probe.Case], agreementFailure{Property: "P3", Identity: "block-three-way", Fragment: probe.Fragment, Direction: direction, Multiplicity: 1, Cut: excerpt, PagePresent: present, JudgeAccepted: accepted, ExcerptFound: found, Observation: fmt.Sprintf("id=%q page=%t judge=%t excerpt=%t cut=%q", probe.Fragment, present, accepted, found, excerpt)})
				}
			}
		}
		if !found && excerpt != "" {
			failures[probe.Case] = append(failures[probe.Case], agreementFailure{Property: "P3", Identity: "missing-excerpt-widened", Fragment: probe.Fragment, Direction: "excerpt-widened", Multiplicity: 1, Cut: excerpt, PagePresent: present, JudgeAccepted: accepted, ExcerptFound: found, Observation: fmt.Sprintf("id=%q cut=%q", probe.Fragment, excerpt)})
		}
	}
	return failures
}

func agreementMissing(t agreementTB, root string, probes []agreementProbe) map[string]int {
	t.Helper()
	findings, err := agreementPublicCheck(t, root)
	if err != nil {
		t.Fatalf("public Check setup/refusal: %v", err)
	}
	planned := make(map[string]agreementProbe, len(probes))
	for _, probe := range probes {
		planned[probe.Path] = probe
	}
	counts := make(map[string]int)
	for i := range findings {
		finding := &findings[i]
		if finding.RuleID == "scan.unreadable" || finding.RuleID == "scan.skipped" {
			t.Fatalf("incomplete Check corpus: %+v", *finding)
		}
		probe, isProbe := planned[finding.Path]
		if !isProbe {
			continue
		}
		if strings.HasPrefix(string(finding.RuleID), "link.") && finding.RuleID != "link.block_missing" && finding.RuleID != "link.section_missing" {
			t.Fatalf("probe did not reach fragment verdict: %+v", *finding)
		}
		if finding.RuleID != "link.block_missing" && finding.RuleID != "link.section_missing" {
			continue
		}
		if finding.RuleID != probe.Rule || finding.Target == nil || *finding.Target != probe.Target || finding.ResolvedTo == nil || *finding.ResolvedTo != probe.ResolvedTo {
			t.Fatalf("misattributed receipt: %+v want=%+v", *finding, probe)
		}
		counts[probe.Path]++
		if counts[probe.Path] > 1 {
			t.Fatalf("duplicate probe receipt: %+v", *finding)
		}
	}
	return counts
}
