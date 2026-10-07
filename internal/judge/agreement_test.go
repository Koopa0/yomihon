package judge_test

import (
	"crypto/sha256"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// This initial baseline has no allowances. A difference is evidence to retain,
// not permission to reseed the corpus or redefine an oracle until it is green.
const agreementGenerator = "pcg-agreement-v1"

func agreementSeedPairs() [4][2]uint64 {
	return [4][2]uint64{
		{0x4a75646765416772, 0x65656d656e743031},
		{0x4a75646765416772, 0x65656d656e743032},
		{0x4a75646765416772, 0x65656d656e743033},
		{0x4a75646765416772, 0x65656d656e743034},
	}
}

// Ordered and immutable: even a reorder changes every subsequent PCG body.
func agreementTokens() []string {
	return []string{
		"\n", "\n\n", " ", "\t", "    ", "text ", "A\n", "B\n", "E\u0301\n", "É\n", "章節\n",
		"[[A]]", "[[A]] [[A]]", "[[B|alias]]", "[[A\\|alias]]", "[[A\\]]", "[[#A]]", "[[A#A]]", "[[A#^a]]", "![[A]]", "![[A#A]]", "[[image.png]]", "![[image.png]]", "\\[[A]]", "\\\\[[A]]", "[[A\nB]]",
		"`", "``", "`[[A]]`", "`open\n[[A]]\nclose`", "\\`", "```\n", "````\n", "``` [[A]]\n", "```` go [[A]]\n", "~~~\n", "~~~~\n", " ```\n", "  ```\n", "   ```\n", "    ```\n", "\t```\n",
		"> ", "> > ", "- ", "1. ", "  - ", "- item\n\n      ", "> [!note] title\n", "> [!note] [[A]]\n", "> [!note] one\n> [!note] two\n> [!note] three\n", "> [!unknown] title\n", "  > [!note] title\n", "- > [!note] title\n",
		"%%", "%%[[A]]%%", "<!--", "-->", "<!-- [[A]] -->", "<!--\n[[A]]\n-->",
		"# A\n", "## A\n", "## A\n## A\n", "## A-2\n## A\n## A\n", "## !\n", "## [[A|alias]]\n", "## <em>A</em>\n", "A\n===\n", "A\n---\n", " ^a\n", " ^A\n", " ^é\n", " ^e\u0301\n", " ^a-2\n", "^absent-prose ",
		"ref[^n]\n", "[^n]: [[A]]\n\n    [[B]]\n", "[^unused]: [[A]]\n", "- [ ] [[A]]\n", "- [x] [[B]]\n", "https://example.invalid/`[[A]]` ", "| a | b |\n|---|---|\n| [[A]] | ^a |\n", "<div>\n[[A]]\n</div>\n", "\ue0000\ue001", "\ue0020\ue003",
	}
}

type agreementCase struct {
	Name       string
	Body       string
	Title      string
	Companions capturedBodies
}

type agreementFailure struct {
	Property    string
	Identity    string
	Observation string
}

func TestAgreement(t *testing.T) {
	t.Parallel()
	t.Run("envelope-identity", agreementEnvelopeIdentity)
	t.Run("bounded-excerpts", agreementExcerptCuts)
	t.Run("ordered-occurrences", agreementOrderedOccurrences)
	t.Run("known-controls", agreementKnownControls)
	t.Run("notice-projection", agreementNoticeProjection)
	fixtures := agreementFixtures(t)
	controls := []agreementCase{
		{Name: "control/citations", Body: "[[A]] [[A]] [[B|alias]]\n"},
		{Name: "control/code", Body: "`[[A]]`\n\n``` go\n[[A]]\n```\n"},
		{Name: "control/block", Body: "first line\ncontinued ^a\n\nsecond ^b\n"},
		{Name: "control/duplicate-address", Body: "first ^a\n\nsecond ^a\n"},
		{Name: "control/title", Title: "A", Body: "# A\n\n## A\n## A\n"},
		{Name: "control/headings", Body: "## A\n## A\n\nB\n===\n"},
		{Name: "control/inline-footnote", Body: "paragraph ^[literal]\n"},
		{Name: "control/footnote", Body: "ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n\n[^unused]: [[A]]\n"},
		{Name: "control/callout", Body: "> [!note] [[A]]\n> [[B]] ^a\n"},
		{Name: "control/comments", Body: "%%[[A]]%%\n<!-- [[B]] -->\n[[A]]\n"},
		{Name: "control/containers", Body: "- item\n\n      [[A]] ^a\n\n> ```\n> [[B]]\n> ```\n"},
	}
	for _, c := range controls {
		t.Logf("control=%s sha256=%x", c.Name, sha256.Sum256([]byte(c.Body)))
	}
	t.Run("fixtures", func(t *testing.T) {
		agreementBatchCases(t, append(fixtures, controls...))
	})
	for shard, seeds := range agreementSeedPairs() {
		t.Run(fmt.Sprintf("shard-%d", shard), func(t *testing.T) {
			t.Parallel()
			rng := rand.New(rand.NewPCG(seeds[0], seeds[1])) // #nosec G404 -- judge_test owns reproducible corpus generation, not security; crypto randomness changes frozen body digests
			alphabet := agreementTokens()
			count := 2048
			if testing.Short() {
				count = 128
			}
			cases := make([]agreementCase, 0, count)
			digest := sha256.New()
			for index := range count {
				var body strings.Builder
				for range 1 + rng.IntN(32) {
					body.WriteString(alphabet[rng.IntN(len(alphabet))])
				}
				text := body.String()
				if _, err := fmt.Fprintf(digest, "%d:%s", len(text), text); err != nil {
					t.Fatalf("frame corpus digest: %v", err)
				}
				cases = append(cases, agreementCase{Name: fmt.Sprintf("%s/seeds-%016x-%016x/shard-%d/index-%04d", agreementGenerator, seeds[0], seeds[1], shard, index), Body: text})
			}
			t.Logf("generator=%s count=%d sha256=%x", agreementGenerator, count, digest.Sum(nil))
			agreementBatchCases(t, cases)
		})
	}
}

func agreementBatchCases(t *testing.T, cases []agreementCase) {
	t.Helper()
	page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
	for start := 0; start < len(cases); start += 64 {
		batch := cases[start:min(start+64, len(cases))]
		observed := make([]agreementHTML, len(batch))
		for i, c := range batch {
			result := page.HTML("Notes/Reading.md", c.Title, c.Body, wording.En)
			observed[i] = agreementObserve(t, result.HTML)
			if c.Title != "" {
				observed[i] = agreementTitleHTML(t, c, &result)
			}
			for _, failure := range agreementPageFailures(c.Body, &result, &observed[i]) {
				t.Errorf("caught: %s %s case=%s body=%q observations=%s", failure.Property, failure.Identity, c.Name, c.Body, failure.Observation)
			}
		}
		agreementFragments(t, batch, observed)
	}
}

func agreementPageFailures(body string, result *render.Result, actual *agreementHTML) []agreementFailure {
	var failures []agreementFailure
	var diagnostics []agreementCitation
	var markdownDiagnostics []agreementCitation
	for _, d := range result.Diagnostics {
		if d.Kind == render.DiagRenderFailed {
			failures = append(failures, agreementFailure{Property: "setup", Identity: "render-failed", Observation: fmt.Sprintf("%+v", d)})
		}
		if d.Kind == render.DiagWikilinkBroken {
			diagnostics = append(diagnostics, agreementCitation{Target: d.Target, Section: d.Section, State: "wikilink-broken"})
		}
		if d.Kind == render.DiagMarkdownBroken {
			markdownDiagnostics = append(markdownDiagnostics, agreementCitation{SourceRole: agreementOutsideMarkdown, Target: d.Target, Section: d.Section, State: "wikilink-broken"})
		}
	}
	var ordered, markdown []agreementCitation
	wikiCount, markdownCount := 0, 0
	for _, citation := range actual.Citations {
		switch citation.SourceRole {
		case "":
			wikiCount++
			if citation.State == "wikilink-broken" {
				ordered = append(ordered, citation)
			}
		case agreementOutsideMarkdown:
			markdownCount++
			markdown = append(markdown, citation)
		default:
			failures = append(failures, agreementFailure{Property: "setup", Identity: "unknown-source-role", Observation: fmt.Sprintf("%+v", citation)})
		}
	}
	if wikiCount+markdownCount != len(actual.Citations) {
		failures = append(failures, agreementFailure{Property: "P0", Identity: "carrier-partition", Observation: fmt.Sprintf("wiki=%d markdown=%d carriers=%d", wikiCount, markdownCount, len(actual.Citations))})
	}
	slices.SortFunc(diagnostics, agreementCitationCompare)
	slices.SortFunc(ordered, agreementCitationCompare)
	if diff := cmp.Diff(diagnostics, ordered); diff != "" {
		failures = append(failures, agreementFailure{Property: "P0", Identity: "diagnostic-html", Observation: diff})
	}
	slices.SortFunc(markdownDiagnostics, agreementCitationCompare)
	slices.SortFunc(markdown, agreementCitationCompare)
	if diff := cmp.Diff(markdownDiagnostics, markdown); diff != "" {
		failures = append(failures, agreementFailure{Property: "P0", Identity: "markdown-diagnostic-html", Observation: diff})
	}
	judgeTargets := judge.LinkTargets(body)
	pageTargets := make([]string, 0, len(actual.Citations))
	for _, citation := range actual.Citations {
		if citation.SourceRole == "" && citation.Target != "" {
			pageTargets = append(pageTargets, citation.Target)
		}
	}
	slices.Sort(judgeTargets)
	slices.Sort(pageTargets)
	if diff := cmp.Diff(judgeTargets, pageTargets); diff != "" {
		failures = append(failures, agreementFailure{Property: "P1", Identity: "citation-occurrences", Observation: diff})
	}
	if actual.CitationsInCode != 0 {
		failures = append(failures, agreementFailure{Property: "P2", Identity: "wikilink-in-code", Observation: fmt.Sprintf("carriers=%d html=%q", actual.CitationsInCode, result.HTML)})
	}
	return failures
}
