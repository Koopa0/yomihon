package judge_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// Corpus identity is immutable. Finite witnessed debt may shrink; unknown
// differences never authorize reseeding or redefining an oracle.
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
	Property      string
	Identity      string
	Observation   string
	Tuple         agreementCitation
	Direction     string
	Multiplicity  int
	Fragment      string
	Cut           string
	PagePresent   bool
	JudgeAccepted bool
	ExcerptFound  bool
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
		{Name: "control/wrapped-local-heading-code", Body: "# A\n\n`open\n[[#A]]\nclose`\n"},
		{Name: "control/unused-text-address-escape", Body: "\\[[A]]\n\n[^unused]: words ^a\n"},
		{Name: "control/multiple-section-code-fields", Body: "`open\n[[A#A]] [[A#A]] [[B#B]]\nclose`\n"},
		{Name: "control/URL-separated-code", Body: "`open\n[[A]]\nclose`## A\nhttps://example.invalid/`[[A]]` ~~~~\n\ue0020\ue003"},
		{Name: "control/terminal-comment-code", Body: "``[[A]] [[A]]## !\n[[A\nB]]``\\[[A]]```` go [[A]]\nÉ\n<!--"},
		{Name: "control/unused-definition-raw-fields", Body: "[[A\nB]]- item\n\n      ## A\n[^unused]: [[A]]\n[[A\\]]"},
		{Name: "control/comment-made-definition", Body: "A\n<!-- [[A]] -->[^unused]: [[A]]\n"},
		{Name: "control/text-owned-suffix-targets", Body: "[[A\\]]  ```\n\n1. [[B\\]]text \n"},
		{Name: "control/repeated-literal-title-receipts", Body: "> [!note] [[A]]\n> [!note] [[A]]\n"},
		{Name: "control/shared-literal-title-targets", Body: "> [!note] [[A]] [[B]]\n> [!note] [[A]] [[A]] [[B]]\n"},
		{Name: "control/realized-callout-heading-namespace", Body: "> [!note] title\n> words\n\n## A\n## A\n"},
		{Name: "control/realized-comment-heading-namespace", Body: "%%\nhidden\n%%\n## A\n## A\n"},
		{Name: "control/realized-terminal-heading-namespace", Body: "## A\n## A\n<!-- hidden\n"},
		{Name: "control/unwritten-embed-heading", Body: "![[A]]A\n---\n"},
		{Name: "control/unwritten-alias-heading-namespace", Body: "## A-B-Alias-C\n## A![[B|alias]]C\n"},
		{Name: "control/wrapped-heading-widget-code", Body: "`open\n[[A#A]]\nclose`\n"},
		{Name: "control/wrapped-raw-suffix-code", Body: "`open\n[[A\\]]\nclose`\n"},
		{Name: "control/plain-opener-wrapped-code", Body: "> [!note] title\n> `open\n> [[A]]\n> close`\n"},
		{Name: "control/block", Body: "first line\ncontinued ^a\n\nsecond ^b\n"},
		{Name: "control/duplicate-address", Body: "first ^a\n\nsecond ^a\n"},
		{Name: "control/title", Title: "A", Body: "# A\n\n## A\n## A\n"},
		{Name: "control/literal-heading", Body: "## `[[B|alias]]`\n"},
		{Name: "control/headings", Body: "## A\n## A\n\nB\n===\n"},
		{Name: "control/wrapped-widget-heading", Body: "[[A\nB]]A\n=\n"},
		{Name: "control/wrapped-widget-heading-namespace", Body: "A\n---\n[[A\nB]]A\n===\n"},
		{Name: "control/headings-beside-opener", Body: "## A\n  > [!note] title\n## A\n## A\n"},
		{Name: "control/heading-collision-beside-opener", Body: "## A\n> [!note] title\n## A\n## A-2\n"},
		{Name: "control/unused-and-headings-beside-opener-run", Body: "> [!note] one\n> [!note] two\n> [!note] three\n\n[^unused]: [[A]]\n\n## A\n## A\n"},
		{Name: "control/wrapped-code-beside-opener-run", Body: "> [!note] one\n> [!note] two\n> [!note] three\n\n`open\n[[A]]\nclose`\n"},
		{Name: "control/inline-footnote", Body: "paragraph ^[literal]\n"},
		{Name: "control/footnote", Body: "ref[^n]\n\n[^n]: [[A]]\n\n    [[B]]\n\n[^unused]: [[A]]\n"},
		{Name: "control/callout", Body: "> [!note] [[A]]\n> [[B]] ^a\n"},
		{Name: "control/quoted-fence-info", Body: "> ```[[A]]\n"},
		{Name: "control/list-fence-info", Body: "1. ```[[A]]\n"},
		{Name: "control/reference-destination", Body: "[n]: [[A]]\n"},
		{Name: "control/compound-reference-destination", Body: "[n]: [[A]][[B]]\n"},
		{Name: "control/shared-reference-destination", Body: "[n]: [[A]]\n[m]: [[A]][[B]]\n"},
		{Name: "control/unused-beside-opener", Body: "> [!note] title\n\n[^unused]: [[A]]\n"},
		{Name: "control/unused-beside-terminal-comment", Body: "[^n]: [[A]]\n\n    [[B]]\n<!--"},
		{Name: "control/wrapped-code-beside-opener", Body: "> [!note] title\n\n`open\n[[A]]\nclose`\n"},
		{Name: "control/suffix-beside-opener", Body: "> [!note] title\n\n[[A\\]]\n"},
		{Name: "control/suffix-beside-prose", Body: "É\n[[A\\]]"},
		{Name: "control/reference-beside-opener", Body: "> [!note] title\n\n[n]: [[A]]\n"},
		{Name: "control/comments", Body: "%%[[A]]%%\n<!-- [[B]] -->\n[[A]]\n"},
		{Name: "control/containers", Body: "- item\n\n      [[A]] ^a\n\n> ```\n> [[B]]\n> ```\n"},
		{Name: "control/quote-fence-outer-address", Body: "> ```\n^a\n"},
		{Name: "control/quote-fence-outer-after-content", Body: "> ```\n> code\n\n^a\n"},
		{Name: "control/quote-fence-literal-address", Body: "> ```\n> ^a\n"},
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
			want := [4]string{
				"248884c0dd26f0d98cf8c7956f20f586dffaf52b73c262543ec924ef2c45c1cb",
				"d3b1e4ef1a423ce44270e2300553d20e9b6a8cf7da34e36e1c3674969088dbb1",
				"10484a58e1bd6f9a4ab0727c42c4ad1be6169b1a232b7765ce681a61a8f43cbb",
				"bd082b5cf7195d6bb3c9b4ec5ef4fb84dcc3b26779bd17ca78647901f6110e4d",
			}
			got := hex.EncodeToString(digest.Sum(nil))
			if got != want[shard] {
				t.Fatalf("generator identity shard=%d digest=%s, want %s", shard, got, want[shard])
			}
			t.Logf("generator=%s generated=%d sha256=%s", agreementGenerator, count, got)
			if testing.Short() {
				cases = cases[:128]
			}
			agreementBatchCases(t, cases)
		})
	}
}

type agreementCounterexample struct {
	Case    agreementCase
	Failure agreementFailure
}

func agreementBatchCases(t *testing.T, cases []agreementCase) {
	t.Helper()
	page := render.New(graph.BuildFromNotes(nil, nil), capturedBodies{}, noTitlesDeclared{}, everyFileHeld{})
	var counterexamples []agreementCounterexample
	for start := 0; start < len(cases); start += 64 {
		batch := cases[start:min(start+64, len(cases))]
		observed := make([]agreementHTML, len(batch))
		rendered := make([]string, len(batch))
		failures := make([][]agreementFailure, len(batch))
		for i, c := range batch {
			result := page.HTML("Notes/Reading.md", c.Title, c.Body, wording.En)
			rendered[i] = result.HTML
			setup := agreementCapture(t, func(observer agreementTB) {
				observed[i] = agreementObserve(observer, result.HTML)
				if c.Title != "" {
					observed[i] = agreementTitleHTML(observer, c, &result)
				}
				failures[i] = agreementPageFailures(c.Body, &result, &observed[i])
			})
			if setup != "" {
				failures[i] = append(failures[i], agreementFailure{Property: "setup", Identity: "page-observation", Observation: setup})
			}
		}
		var fragments [][]agreementFailure
		fragmentSetup := agreementCapture(t, func(observer agreementTB) {
			fragments = agreementFragmentFailures(observer, batch, observed)
		})
		if fragmentSetup != "" {
			fragments = make([][]agreementFailure, len(batch))
			for i := range batch {
				fragments[i] = append(fragments[i], agreementFailure{Property: "setup", Identity: "fragment-batch", Observation: fragmentSetup})
			}
		}
		for i, c := range batch {
			failures[i] = append(failures[i], fragments[i]...)
			designed := make(map[string]bool)
			designedSetup := agreementCapture(t, func(observer agreementTB) {
				designed = agreementDesignedDifferences(observer, c, &observed[i], failures[i])
				for signature := range agreementMultipleTitleDifferences(observer, c, &observed[i], failures[i]) {
					designed[signature] = true
				}
			})
			if designedSetup != "" {
				failures[i] = append(failures[i], agreementFailure{Property: "setup", Identity: "designed-receipt", Observation: designedSetup})
			}
			unusedTargets := agreementUnusedFootnoteTargets(c.Body)
			var unusedTailTargets map[string]int
			var unusedWidgets agreementReferenceDestinations
			unusedWidgetsObserved := false
			unusedTailObserved := false
			var commentMadeTargets map[string]int
			commentMadeObserved := false
			var referenceDestinations agreementReferenceDestinations
			referenceObserved := false
			var compoundReferences agreementReferenceDestinations
			compoundReferencesObserved := false
			var containerFenceDiagnostics map[agreementCitation]int
			containerFenceObserved := false
			var exclusiveContainerInfo map[agreementCitation]int
			exclusiveContainerInfoObserved := false
			var standaloneTargets agreementStandaloneTargets
			standaloneTargetsObserved := false
			var proseTargets agreementStandaloneTargets
			proseTargetsObserved := false
			var textTargets agreementStandaloneTargets
			textTargetsObserved := false
			var textSuffixFields agreementTextSuffixFields
			textSuffixFieldsObserved := false
			var headingCounts map[string]int
			headingObserved := false
			var collisionIDs map[string]bool
			collisionObserved := false
			var realizedNamespaceIDs map[string]bool
			realizedNamespaceObserved := false
			var mixedNamespaceIDs map[string]bool
			mixedNamespaceObserved := false
			var literalNamespaceIDs map[string]bool
			literalNamespaceObserved := false
			var literalHeadingIDs map[string]bool
			literalHeadingObserved := false
			var wrappedHeadingIDs map[string]bool
			wrappedHeadingObserved := false
			var wrappedHeadingNamespaceIDs map[string]bool
			wrappedHeadingNamespaceObserved := false
			var embedHeadingIDs map[string]bool
			embedHeadingObserved := false
			var codePayload agreementCodePayload
			codePayloadObserved := false
			var pageCodePayload agreementCodePayload
			pageCodePayloadObserved := false
			var codeTargets map[string]int
			var tailCodeTargets map[string]int
			var urlCodeTargets map[string]int
			urlCodeObserved := false
			tailCodeObserved := false
			codeObserved := false
			var localCodeHeadings map[string]int
			localCodeObserved := false
			var declaredWidgetCode agreementWidgetCodeBudget
			declaredWidgetCodeObserved := false
			var calloutWidgetCode agreementWidgetCodeBudget
			var codeFields agreementWidgetCodeBudget
			codeFieldsObserved := false
			var exclusiveCode map[agreementCitation]int
			exclusiveCodeObserved := false
			calloutWidgetCodeObserved := false
			var continuationTargets map[string]int
			continuationObserved := false
			var fenceInfoTargets map[string]int
			fenceInfoObserved := false
			var unusedAddresses map[string]int
			unusedAddressesObserved := false
			var unusedTextAddresses agreementUnusedTextAddresses
			unusedTextAddressesObserved := false
			var unusedParagraphAddresses agreementUnusedTextAddresses
			var unusedOpenerAddresses agreementUnusedTextAddresses
			var unusedBlockAddresses agreementUnusedTextAddresses
			unusedBlockAddressesObserved := false
			var discardedBlockAddresses agreementUnusedTextAddresses
			discardedBlockAddressesObserved := false
			var physicalDiscardedAddresses agreementUnusedTextAddresses
			physicalDiscardedAddressesObserved := false
			unusedOpenerAddressesObserved := false
			unusedParagraphAddressesObserved := false
			var exclusiveUnusedFields map[agreementCitation]int
			var unusedBlockFields map[agreementCitation]int
			var unusedBlockCheckFields map[string]int
			unusedBlockFieldsObserved := false
			var discardedTargetFields map[agreementCitation]int
			var discardedTargetCheckFields map[string]int
			discardedTargetFieldsObserved := false
			exclusiveUnusedFieldsObserved := false
			var outerQuoteAddresses agreementQuoteAddresses
			outerQuoteAddressesObserved := false
			for failureIndex := range failures[i] {
				failure := &failures[i][failureIndex]
				if designed[agreementSignature(failure)] {
					t.Logf("known=designed authority=#1011 callout.title_markup wrong=none case=%s signature=%s", c.Name, agreementSignature(failure))
					continue
				}
				classification, authority, wrong := agreementKnownDifference(c, failure)
				if classification == "" {
					classification, authority, wrong = agreementUnusedFootnoteDifference(c, failure, unusedTargets)
				}
				if classification == "" && (failure.Property == "P0" || failure.Property == "P1") {
					if !unusedTailObserved {
						unusedTailTargets = agreementUnusedFootnoteTailTargets(c.Body)
						unusedTailObserved = true
					}
					classification, authority, wrong = agreementUnusedFootnoteDifference(c, failure, unusedTailTargets)
				}
				if classification == "" && (failure.Property == "P0" || failure.Property == "P1") {
					if !unusedWidgetsObserved {
						unusedWidgets = agreementUnusedFootnoteWidgetBudget(c.Body)
						unusedWidgetsObserved = true
					}
					classification, authority, wrong = agreementUnusedFootnoteWidgetDifference(c, failure, unusedWidgets)
				}
				if classification == "" && (failure.Property == "P0" || failure.Property == "P1") {
					if !exclusiveUnusedFieldsObserved {
						exclusiveUnusedFields = agreementExclusiveUnusedFieldBudget(c.Body)
						exclusiveUnusedFieldsObserved = true
					}
					classification, authority, wrong = agreementExclusiveUnusedDiagnosticDifference(c, failure, &observed[i], exclusiveUnusedFields)
					if classification == "" {
						classification, authority, wrong = agreementExclusiveHiddenCitationDifference(c, failure, &observed[i], exclusiveUnusedFields)
					}
				}
				if classification == "" && (failure.Property == "P0" || failure.Property == "P1") {
					if !unusedBlockFieldsObserved {
						unusedBlockFields, unusedBlockCheckFields = agreementUnusedBlockFieldBudgets(c.Body)
						unusedBlockFieldsObserved = true
					}
					classification, authority, wrong = agreementExclusiveUnusedDiagnosticDifference(c, failure, &observed[i], unusedBlockFields)
					if classification == "" {
						classification, authority, wrong = agreementExclusiveHiddenCitationDifference(c, failure, &observed[i], unusedBlockFields)
					}
					if classification == "" {
						classification, authority, wrong = agreementUnusedBlockCheckDifference(c, failure, &observed[i], unusedBlockCheckFields)
					}
				}

				if classification == "" && (failure.Property == "P0" || failure.Property == "P1") {
					if !discardedTargetFieldsObserved {
						discardedTargetFields, discardedTargetCheckFields = agreementDiscardedTargetFields(c.Body)
						discardedTargetFieldsObserved = true
					}
					classification, authority, wrong = agreementExclusiveUnusedDiagnosticDifference(c, failure, &observed[i], discardedTargetFields)
					if classification == "" {
						classification, authority, wrong = agreementExclusiveHiddenCitationDifference(c, failure, &observed[i], discardedTargetFields)
					}
					if classification == "" {
						classification, authority, wrong = agreementUnusedBlockCheckDifference(c, failure, &observed[i], discardedTargetCheckFields)
					}
				}

				if classification == "" && (failure.Property == "P0" || failure.Property == "P1") {
					if !commentMadeObserved {
						commentMadeTargets = agreementCommentMadeFootnoteTargets(c.Body)
						commentMadeObserved = true
					}
					classification, authority, wrong = agreementCommentMadeFootnoteDifference(c, failure, commentMadeTargets)
				}
				if classification == "" && (failure.Property == "P0" || failure.Property == "P1") {
					if !referenceObserved {
						referenceDestinations = agreementReferenceDestinationBudget(c.Body)
						referenceObserved = true
					}
					classification, authority, wrong = agreementReferenceDestinationDifference(c, failure, referenceDestinations)
				}
				if classification == "" && (failure.Property == "P0" || failure.Property == "P1") {
					if !compoundReferencesObserved {
						compoundReferences = agreementCompoundReferenceBudget(c.Body)
						compoundReferencesObserved = true
					}
					classification, authority, wrong = agreementReferenceDestinationDifference(c, failure, compoundReferences)
				}
				if classification == "" && (failure.Property == "P0" || failure.Property == "P1") {
					combined := agreementSharedReferenceBudget(referenceDestinations, compoundReferences)
					classification, authority, wrong = agreementReferenceDestinationDifference(c, failure, combined)
				}
				if classification == "" && failure.Property == "P0" {
					if !containerFenceObserved {
						containerFenceDiagnostics = agreementContainerFenceDiagnostics(c.Body)
						containerFenceObserved = true
					}
					classification, authority, wrong = agreementContainerFenceDiagnosticDifference(c, failure, containerFenceDiagnostics)
				}
				if classification == "" && failure.Property == "P0" {
					if !exclusiveContainerInfoObserved {
						exclusiveContainerInfo = agreementExclusiveContainerInfoBudget(c.Body)
						exclusiveContainerInfoObserved = true
					}
					classification, authority, wrong = agreementExclusiveContainerInfoDifference(c, failure, &observed[i], exclusiveContainerInfo)
				}
				if classification == "" && failure.Property == "P1" {
					if !standaloneTargetsObserved {
						standaloneTargets = agreementStandaloneTargetDebt(c.Body)
						standaloneTargetsObserved = true
					}
					classification, authority, wrong = agreementStandaloneTargetDifference(c, failure, standaloneTargets)
				}
				if classification == "" && failure.Property == "P1" {
					if !proseTargetsObserved {
						proseTargets = agreementProseTargetDebt(c.Body)
						proseTargetsObserved = true
					}
					classification, authority, wrong = agreementStandaloneTargetDifference(c, failure, proseTargets)
				}
				if classification == "" && failure.Property == "P1" {
					if !textTargetsObserved {
						textTargets = agreementTextTargetDebt(c.Body)
						textTargetsObserved = true
					}
					classification, authority, wrong = agreementStandaloneTargetDifference(c, failure, textTargets)
				}
				if classification == "" && failure.Property == "P1" {
					if !textSuffixFieldsObserved {
						textSuffixFields = agreementTextSuffixFieldBudget(c.Body)
						textSuffixFieldsObserved = true
					}
					classification, authority, wrong = agreementTextSuffixFieldDifference(c, failure, &observed[i], textSuffixFields)
				}
				if classification == "" && failure.Property == "P4" {
					if !headingObserved {
						headingCounts = agreementDeclaredHeadingCounts(c.Body)
						headingObserved = true
					}
					classification, authority, wrong = agreementDuplicateHeadingDifference(c, failure, headingCounts)
				}
				if classification == "" && failure.Property == "P4" {
					if !collisionObserved {
						collisionIDs = agreementHeadingCollisionIDs(c.Body)
						collisionObserved = true
					}
					classification, authority, wrong = agreementHeadingCollisionDifference(c, failure, collisionIDs)
				}
				if classification == "" && failure.Property == "P4" {
					if !realizedNamespaceObserved {
						realizedNamespaceIDs = agreementRealizedNamespaceIDs(c.Body, &observed[i])
						realizedNamespaceObserved = true
					}
					classification, authority, wrong = agreementWrappedHeadingDifference(c, failure, realizedNamespaceIDs)
				}
				if classification == "" && failure.Property == "P4" {
					if !literalHeadingObserved {
						literalHeadingIDs = agreementLiteralHeadingIDs(c.Body)
						literalHeadingObserved = true
					}
					classification, authority, wrong = agreementLiteralHeadingDifference(c, failure, literalHeadingIDs)
				}
				if classification == "" && failure.Property == "P4" {
					if !literalNamespaceObserved {
						literalNamespaceIDs = agreementLiteralNamespaceIDs(c.Body, &observed[i])
						literalNamespaceObserved = true
					}
					classification, authority, wrong = agreementWrappedHeadingDifference(c, failure, literalNamespaceIDs)
				}

				if classification == "" && failure.Property == "P4" {
					if !mixedNamespaceObserved {
						mixedNamespaceIDs = agreementMixedNamespaceIDs(c.Body, &observed[i])
						mixedNamespaceObserved = true
					}
					classification, authority, wrong = agreementWrappedHeadingDifference(c, failure, mixedNamespaceIDs)
				}

				if classification == "" && failure.Property == "P4" {
					if !wrappedHeadingObserved {
						wrappedHeadingIDs = agreementWrappedHeadingIDs(c.Body)
						wrappedHeadingObserved = true
					}
					classification, authority, wrong = agreementWrappedHeadingDifference(c, failure, wrappedHeadingIDs)
				}
				if classification == "" && failure.Property == "P4" {
					if !wrappedHeadingNamespaceObserved {
						wrappedHeadingNamespaceIDs = agreementWrappedHeadingNamespaceIDs(c.Body)
						wrappedHeadingNamespaceObserved = true
					}
					classification, authority, wrong = agreementWrappedHeadingDifference(c, failure, wrappedHeadingNamespaceIDs)
				}
				if classification == "" && failure.Property == "P4" {
					if !embedHeadingObserved {
						embedHeadingIDs = agreementUnwrittenEmbedHeadingIDs(c.Body)
						embedHeadingObserved = true
					}
					classification, authority, wrong = agreementWrappedHeadingDifference(c, failure, embedHeadingIDs)
				}
				if classification == "" && (failure.Property == "P1" || failure.Property == "P2") {
					if !codeObserved {
						codeTargets = agreementCodeDebtTargets(c.Body)
						codeObserved = true
					}
					classification, authority, wrong = agreementCodeDebtDifference(c, failure, codeTargets)
				}
				if classification == "" && (failure.Property == "P1" || failure.Property == "P2") {
					if !tailCodeObserved {
						tailCodeTargets = agreementCodeDebtTailTargets(c.Body)
						tailCodeObserved = true
					}
					classification, authority, wrong = agreementCodeTailDifference(c, failure, tailCodeTargets, &observed[i])
				}
				if classification == "" && (failure.Property == "P1" || failure.Property == "P2") {
					if !urlCodeObserved {
						urlCodeTargets = agreementURLCodeDebtTargets(c.Body)
						urlCodeObserved = true
					}
					classification, authority, wrong = agreementCodeCarrierDifference(c, failure, urlCodeTargets, &observed[i])
				}
				if classification == "" && failure.Property == "P2" {
					if !localCodeObserved {
						localCodeHeadings = agreementLocalCodeHeadings(c.Body)
						localCodeObserved = true
					}
					classification, authority, wrong = agreementLocalCodeDifference(c, failure, localCodeHeadings)
				}
				if classification == "" && (failure.Property == "P1" || failure.Property == "P2") {
					if !declaredWidgetCodeObserved {
						declaredWidgetCode = agreementDeclaredWrappedCode(c.Body)
						declaredWidgetCodeObserved = true
					}
					classification, authority, wrong = agreementWidgetCodeDifference(c, failure, declaredWidgetCode, &observed[i])
				}
				if classification == "" && (failure.Property == "P1" || failure.Property == "P2") {
					if !calloutWidgetCodeObserved {
						calloutWidgetCode = agreementCalloutWrappedCode(c.Body)
						calloutWidgetCodeObserved = true
					}
					classification, authority, wrong = agreementWidgetCodeDifference(c, failure, calloutWidgetCode, &observed[i])
				}
				if classification == "" && (failure.Property == "P1" || failure.Property == "P2") {
					if !codeFieldsObserved {
						codeFields = agreementCodeFieldBudget(c.Body)
						codeFieldsObserved = true
					}
					classification, authority, wrong = agreementWidgetCodeDifference(c, failure, codeFields, &observed[i])
				}
				if classification == "" && (failure.Property == "P1" || failure.Property == "P2") {
					if !codePayloadObserved {
						codePayload = agreementCodeWindowBudget(c.Body, rendered[i])
						codePayloadObserved = true
					}
					classification, authority, wrong = agreementCodeWindowDifference(c, failure, &observed[i], codePayload)
					if classification == "" {
						classification, authority, wrong = agreementCodeCitationDifference(c, failure, &observed[i], codePayload)
					}
				}
				if classification == "" && failure.Property == "P2" {
					if !pageCodePayloadObserved {
						pageCodePayload = agreementPageCodeWindowBudget(c.Body, rendered[i])
						pageCodePayloadObserved = true
					}
					classification, authority, wrong = agreementCodeWindowDifference(c, failure, &observed[i], pageCodePayload)
				}
				if classification == "" && (failure.Property == "P1" || failure.Property == "P2") {
					if !exclusiveCodeObserved {
						exclusiveCode = agreementExclusiveCodeBudget(c.Body)
						exclusiveCodeObserved = true
					}
					classification, authority, wrong = agreementExclusiveCodeDifference(c, failure, exclusiveCode)
					if classification == "" {
						classification, authority, wrong = agreementExclusiveHiddenCitationDifference(c, failure, &observed[i], exclusiveCode)
					}
				}
				if classification == "" && failure.Property == "P1" {
					if !continuationObserved {
						continuationTargets = agreementFootnoteContinuationTargets(c.Body)
						continuationObserved = true
					}
					classification, authority, wrong = agreementFootnoteContinuationDifference(c, failure, continuationTargets)
				}
				if classification == "" && failure.Property == "P1" {
					if !fenceInfoObserved {
						fenceInfoTargets = agreementFenceInfoTargets(c.Body)
						fenceInfoObserved = true
					}
					classification, authority, wrong = agreementFenceInfoDifference(c, failure, fenceInfoTargets)
				}
				if classification == "" && failure.Property == "P3" {
					if !unusedAddressesObserved {
						unusedAddresses = agreementUnusedFootnoteAddresses(c.Body)
						unusedAddressesObserved = true
					}
					classification, authority, wrong = agreementUnusedFootnoteAddressDifference(c, failure, unusedAddresses)
				}
				if classification == "" && failure.Property == "P3" {
					if !unusedTextAddressesObserved {
						unusedTextAddresses = agreementUnusedTextAddressBudget(c.Body)
						unusedTextAddressesObserved = true
					}
					classification, authority, wrong = agreementUnusedTextAddressDifference(c, failure, unusedTextAddresses)
				}
				if classification == "" && failure.Property == "P3" {
					if !unusedParagraphAddressesObserved {
						unusedParagraphAddresses = agreementUnusedParagraphAddressBudget(c.Body)
						unusedParagraphAddressesObserved = true
					}
					classification, authority, wrong = agreementUnusedTextAddressDifference(c, failure, unusedParagraphAddresses)
				}
				if classification == "" && failure.Property == "P3" {
					if !unusedOpenerAddressesObserved {
						unusedOpenerAddresses = agreementUnusedOpenerAddressBudget(c.Body)
						unusedOpenerAddressesObserved = true
					}
					classification, authority, wrong = agreementUnusedTextAddressDifference(c, failure, unusedOpenerAddresses)
				}
				if classification == "" && failure.Property == "P3" {
					if !unusedBlockAddressesObserved {
						unusedBlockAddresses = agreementUnusedBlockAddressBudget(c.Body)
						unusedBlockAddressesObserved = true
					}
					classification, authority, wrong = agreementUnusedTextAddressDifference(c, failure, unusedBlockAddresses)
				}
				if classification == "" && failure.Property == "P3" {
					if !discardedBlockAddressesObserved {
						discardedBlockAddresses = agreementDiscardedBlockAddressBudget(c.Body)
						discardedBlockAddressesObserved = true
					}
					classification, authority, wrong = agreementUnusedTextAddressDifference(c, failure, discardedBlockAddresses)
				}
				if classification == "" && failure.Property == "P3" {
					if !physicalDiscardedAddressesObserved {
						physicalDiscardedAddresses = agreementDiscardedPhysicalAddressBudget(c.Body)
						physicalDiscardedAddressesObserved = true
					}
					classification, authority, wrong = agreementPhysicalDiscardedAddressDifference(c, failure, physicalDiscardedAddresses)
				}

				if classification == "" && failure.Property == "P3" {
					if !outerQuoteAddressesObserved {
						outerQuoteAddresses = agreementQuoteAddressOwnership(c.Body)
						outerQuoteAddressesObserved = true
					}
					classification, authority, wrong = agreementQuoteAddressDifference(c, failure, outerQuoteAddresses)
				}
				if classification != "" {
					t.Logf("known=%s authority=%s wrong=%s case=%s signature=%s", classification, authority, wrong, c.Name, agreementSignature(failure))
					continue
				}
				counterexamples = append(counterexamples, agreementCounterexample{Case: c, Failure: *failure})
			}
		}
	}
	// Evaluate the entire lane before spending the bounded diagnostic replay
	// allowance. Each lane owns its own representatives and candidate budget.
	budget := agreementReplayBudget{}
	for i := range counterexamples {
		example := &counterexamples[i]
		if i >= 8 {
			t.Errorf("caught: %s %s case=%s original-sha256=%x signature=%s body=%q observations=%s minimization=not-attempted representative-budget", example.Failure.Property, example.Failure.Identity, example.Case.Name, sha256.Sum256([]byte(example.Case.Body)), agreementSignature(&example.Failure), example.Case.Body, example.Failure.Observation)
			continue
		}
		body, attempts, checks, stop := agreementMinimizeBudget(t, example, &budget)
		t.Errorf("caught: %s %s case=%s original-sha256=%x signature=%s original=%q minimized=%q observations=%s candidates=%d public-check=%d stop=%s", example.Failure.Property, example.Failure.Identity, example.Case.Name, sha256.Sum256([]byte(example.Case.Body)), agreementSignature(&example.Failure), example.Case.Body, body, example.Failure.Observation, attempts, checks, stop)
	}
}

func agreementSignature(failure *agreementFailure) string {
	return fmt.Sprintf("%s/%s tuple=%+v fragment=%q direction=%s multiplicity=%d cut=%q page=%t judge=%t excerpt=%t", failure.Property, failure.Identity, failure.Tuple, failure.Fragment, failure.Direction, failure.Multiplicity, failure.Cut, failure.PagePresent, failure.JudgeAccepted, failure.ExcerptFound)
}

// Finite witnessed membership is intentionally narrow. Other bodies or
// additional tuples remain red, including additional deltas in these bodies.
func agreementKnownDifference(c agreementCase, failure *agreementFailure) (kind, authority, wrong string) {
	if len(c.Companions) != 0 || failure.Multiplicity != 1 || failure.Cut != "" || failure.JudgeAccepted || failure.ExcerptFound || failure.PagePresent != (failure.Property == "P4") {
		return "", "", ""
	}
	type witness struct {
		title     string
		body      string
		property  string
		identity  string
		tuple     agreementCitation
		fragment  string
		direction string
		wrong     string
		stage     int
	}
	witnesses := []witness{
		{title: "A", body: "# A\n\n## A\n## A\n", property: "P4", identity: "literal-heading-id", fragment: "a-2", direction: "page-only", wrong: "judge", stage: 8},
		{title: "A", body: "# A\n\n## A\n## A\n", property: "P4", identity: "literal-heading-id", fragment: "a-3", direction: "page-only", wrong: "judge", stage: 8},
		{body: "[[Trail\\]]\n", property: "P1", identity: "citation-occurrences", tuple: agreementCitation{Target: "Trail"}, direction: "judge-only", wrong: "page", stage: 3},
		{body: "[[Trail\\]]\n", property: "P1", identity: "citation-occurrences", tuple: agreementCitation{Target: "Trail\\"}, direction: "page-only", wrong: "page", stage: 3},
		{body: "- a list item\n\n    ```\n    [[Nested]]\n    ```\n\n[[Outside List]]\n", property: "P1", identity: "citation-occurrences", tuple: agreementCitation{Target: "Nested"}, direction: "page-only", wrong: "page", stage: 5},
		{body: "- a list item\n\n    ```\n    [[Nested]]\n    ```\n\n[[Outside List]]\n", property: "P2", identity: "wikilink-in-code", tuple: agreementCitation{Target: "Nested", State: "wikilink-broken"}, direction: "page-in-code", wrong: "page", stage: 5},
		{body: "[^unused]: [[A]]\n", property: "P0", identity: "diagnostic-html", tuple: agreementCitation{Target: "A", State: "wikilink-broken"}, direction: "diagnostic-only", wrong: "page-diagnostic", stage: 5},
		{body: "[^unused]: [[A]]\n", property: "P1", identity: "citation-occurrences", tuple: agreementCitation{Target: "A"}, direction: "judge-only", wrong: "judge", stage: 5},
		{body: "``` [[A]]\n", property: "P1", identity: "citation-occurrences", tuple: agreementCitation{Target: "A"}, direction: "judge-only", wrong: "judge", stage: 4},
		{body: "`open\n[[A]]\nclose`", property: "P1", identity: "citation-occurrences", tuple: agreementCitation{Target: "A"}, direction: "page-only", wrong: "page", stage: 5},
		{body: "`open\n[[A]]\nclose`", property: "P2", identity: "wikilink-in-code", tuple: agreementCitation{Target: "A", State: "wikilink-broken"}, direction: "page-in-code", wrong: "page", stage: 5},
		{body: "## A\n## A\n", property: "P4", identity: "literal-heading-id", fragment: "a-2", direction: "page-only", wrong: "judge", stage: 8},
	}
	for i := range witnesses {
		witness := &witnesses[i]
		if c.Title == witness.title && c.Body == witness.body && failure.Property == witness.property && failure.Identity == witness.identity && failure.Tuple == witness.tuple && failure.Fragment == witness.fragment && failure.Direction == witness.direction {
			return "debt", fmt.Sprintf("#1011 stage %d", witness.stage), witness.wrong
		}
	}
	return "", "", ""
}

type agreementReplayBudget struct {
	Candidates int
	Checks     int
}

func agreementMinimize(t *testing.T, original *agreementCounterexample) (minimizedBody string, candidateAttempts, publicChecks int, stopReason string) {
	t.Helper()
	budget := agreementReplayBudget{}
	return agreementMinimizeBudget(t, original, &budget)
}

func agreementMinimizeBudget(t *testing.T, original *agreementCounterexample, budget *agreementReplayBudget) (minimizedBody string, candidateAttempts, publicChecks int, stopReason string) {
	t.Helper()
	body := original.Case.Body
	if original.Failure.Property == "setup" {
		return body, 0, 0, "not-attempted setup-failure"
	}
	if !utf8.ValidString(body) {
		return body, 0, 0, "not-attempted invalid-utf8"
	}
	attempts, checks := 0, 0
	if budget.Checks+2 > 64 {
		return body, 0, 0, "not-attempted public-check-budget"
	}
	before := budget.Checks
	var replay []agreementFailure
	setup := agreementCaptureChecked(t, &budget.Checks, func(observer agreementTB) {
		result, actual := agreementIsolatedPage(observer, original.Case)
		replay = agreementPageFailures(body, &result, &actual)
		fragments := agreementFragmentFailures(observer, []agreementCase{original.Case}, []agreementHTML{actual})
		replay = append(replay, fragments[0]...)
	})
	checks += budget.Checks - before
	if setup != "" {
		return body, 0, checks, "not-attempted isolated-setup-failure: " + setup
	}
	preserved := false
	for failureIndex := range replay {
		failure := &replay[failureIndex]
		if failure.Property == "setup" {
			return body, 0, checks, "not-attempted isolated-setup-failure: " + failure.Observation
		}
		preserved = preserved || agreementSignature(failure) == agreementSignature(&original.Failure)
	}
	if !preserved {
		return body, 0, checks, "not-attempted isolated-different-signature"
	}
	for width := max(1, utf8.RuneCountInString(body)/2); width >= 1; {
		changed := false
		runes := []rune(body)
		for start := 0; start < len(runes); start += width {
			if budget.Candidates >= 128 || budget.Checks+2 > 64 {
				return body, attempts, checks, "budget"
			}
			candidate := string(runes[:start]) + string(runes[min(start+width, len(runes)):])
			attempts++
			budget.Candidates++
			c := original.Case
			c.Body = candidate
			var failures []agreementFailure
			beforeChecks := budget.Checks
			setup := agreementCaptureChecked(t, &budget.Checks, func(observer agreementTB) {
				result, actual := agreementIsolatedPage(observer, c)
				failures = agreementPageFailures(c.Body, &result, &actual)
				fragmentFailures := agreementFragmentFailures(observer, []agreementCase{c}, []agreementHTML{actual})
				failures = append(failures, fragmentFailures[0]...)
			})
			checks += budget.Checks - beforeChecks
			if setup != "" {
				continue
			}
			for failureIndex := range failures {
				failure := &failures[failureIndex]
				if failure.Property == "setup" {
					setup = failure.Observation
				}
			}
			if setup != "" {
				continue
			}
			preserved := false
			for failureIndex := range failures {
				failure := &failures[failureIndex]
				if agreementSignature(failure) == agreementSignature(&original.Failure) {
					preserved = true
				}
			}
			if preserved {
				body = candidate
				changed = true
				break
			}
		}
		if !changed {
			if width == 1 {
				return body, attempts, checks, "chunk-fixed-point-within-budget"
			}
			width = max(1, width/2)
		}
	}
	return body, attempts, checks, "empty"
}

func agreementPageFailures(body string, result *render.Result, actual *agreementHTML) []agreementFailure {
	failures := slices.Clone(actual.Failures)
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
	failures = append(failures, agreementDeltas("P0", "diagnostic-html", "diagnostic", "page", diagnostics, ordered)...)
	failures = append(failures, agreementDeltas("P0", "markdown-diagnostic-html", "diagnostic", "page", markdownDiagnostics, markdown)...)
	judgeTargets := judge.LinkTargets(body)
	pageTargets := make([]string, 0, len(actual.Citations))
	for _, citation := range actual.Citations {
		if citation.SourceRole == "" && citation.Target != "" {
			pageTargets = append(pageTargets, citation.Target)
		}
	}
	judgeCitations := make([]agreementCitation, 0, len(judgeTargets))
	for _, target := range judgeTargets {
		judgeCitations = append(judgeCitations, agreementCitation{Target: target})
	}
	pageCitations := make([]agreementCitation, 0, len(pageTargets))
	for _, target := range pageTargets {
		pageCitations = append(pageCitations, agreementCitation{Target: target})
	}
	failures = append(failures, agreementDeltas("P1", "citation-occurrences", "judge", "page", judgeCitations, pageCitations)...)
	codeFailures := agreementDeltas("P2", "wikilink-in-code", "page-in-code", "none", actual.CodeCitations, nil)
	for i := range codeFailures {
		codeFailures[i].Direction = "page-in-code"
	}
	failures = append(failures, codeFailures...)
	// Resolved code carriers are intentionally outside P1's unresolved target
	// inventory, but remain actual P2 evidence.
	if remaining := actual.CitationsInCode - len(actual.CodeCitations); remaining > 0 {
		failures = append(failures, agreementFailure{Property: "P2", Identity: "wikilink-in-code", Direction: "page-in-code", Multiplicity: remaining, Observation: fmt.Sprintf("resolved-carriers=%d html=%q", remaining, result.HTML)})
	}
	return failures
}

// A difference owns one tuple and one signed occurrence count. A discrepancy
// elsewhere in the same body cannot acquire this tuple's classification.
func agreementDeltas(property, identity, left, right string, a, b []agreementCitation) []agreementFailure {
	counts := make(map[agreementCitation]int)
	for _, tuple := range a {
		counts[tuple]++
	}
	for _, tuple := range b {
		counts[tuple]--
	}
	keys := make([]agreementCitation, 0, len(counts))
	for tuple, count := range counts {
		if count != 0 {
			keys = append(keys, tuple)
		}
	}
	slices.SortFunc(keys, agreementCitationCompare)
	var failures []agreementFailure
	for _, tuple := range keys {
		count := counts[tuple]
		direction := left + "-only"
		if count < 0 {
			direction = right + "-only"
			count = -count
		}
		failures = append(failures, agreementFailure{
			Property: property, Identity: identity, Tuple: tuple,
			Direction: direction, Multiplicity: count,
			Observation: fmt.Sprintf("tuple=%+v direction=%s occurrences=%d", tuple, direction, count),
		})
	}
	return failures
}

// A designed difference is owned by one actual escaped title and one exact
// public receipt. Neither a body-wide callout predicate nor a matching target
// elsewhere supplies ownership.
func agreementDesignedDifferences(t agreementTB, c agreementCase, actual *agreementHTML, failures []agreementFailure) map[string]bool {
	t.Helper()
	allowed := make(map[string]bool)
	if len(actual.CalloutTitles) == 0 || len(c.Companions) != 0 {
		return allowed
	}
	eligible := false
	for failureIndex := range failures {
		failure := &failures[failureIndex]
		eligible = eligible || failure.Property == "P1" && failure.Identity == "citation-occurrences" && failure.Direction == "judge-only"
	}
	if !eligible {
		return allowed
	}
	root := agreementAttributionRoot(t)
	const path = "Notes/Reading.md"
	agreementWrite(t, root, path, agreementEnvelope(t, c.Body))
	findings, err := agreementPublicCheck(t, root)
	if err != nil {
		t.Fatalf("designed receipt public Check setup/refusal: %v", err)
	}
	targetBudget := make(map[string]int)
	targetOwners := make(map[string]int)
	for i := range findings {
		finding := &findings[i]
		if finding.RuleID == "scan.unreadable" || finding.RuleID == "scan.skipped" {
			t.Fatalf("designed receipt incomplete Check: %+v", *finding)
		}
		if finding.RuleID != "callout.title_markup" {
			continue
		}
		if finding.Path != path || finding.Line == nil || finding.Target == nil {
			t.Fatalf("designed receipt missing Path/Line/Target: %+v", *finding)
		}
		title := *finding.Target
		if count := agreementStringCount(actual.CalloutTitles, title); count != 1 {
			continue
		}
		lines := strings.Split(c.Body, "\n")
		line := *finding.Line - 3
		if line < 0 || line >= len(lines) || !render.IsCalloutOpening(lines[line]) {
			continue
		}
		opening := strings.Index(lines[line], "[!")
		closing := strings.IndexByte(lines[line][opening:], ']')
		if closing < 0 {
			continue
		}
		remainder := lines[line][opening+closing+1:]
		if strings.HasPrefix(remainder, "-") || strings.HasPrefix(remainder, "+") {
			remainder = remainder[1:]
		}
		if strings.TrimSpace(remainder) != title {
			continue
		}
		owned := make(map[string]int)
		for _, target := range judge.LinkTargets(title) {
			owned[target]++
		}
		for target, count := range owned {
			targetOwners[target]++
			targetBudget[target] += count
		}
	}
	for failureIndex := range failures {
		failure := &failures[failureIndex]
		if failure.Property == "P1" && failure.Identity == "citation-occurrences" && failure.Direction == "judge-only" && failure.Tuple.SourceRole == "" && failure.Tuple.Section == "" && failure.Tuple.State == "" && failure.Fragment == "" && targetOwners[failure.Tuple.Target] == 1 && targetBudget[failure.Tuple.Target] == failure.Multiplicity {
			allowed[agreementSignature(failure)] = true
		}
	}
	return allowed
}

func agreementStringCount(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

// Setup failure aborts one observation, never masquerading as the selected
// behavioral delta. The embedded test owns disposable files and cleanup.
type agreementTB interface {
	Helper()
	Fatalf(string, ...any)
	Fatal(...any)
	Errorf(string, ...any)
	Error(...any)
	Logf(string, ...any)
	TempDir() string
	Cleanup(func())
	Context() context.Context
}

type agreementSetupFailure struct {
	message string
}

type agreementRecorder struct {
	*testing.T

	checkCalls *int
}

func agreementPublicCheck(t agreementTB, root string) ([]judge.Finding, error) {
	if recorder, ok := t.(agreementRecorder); ok && recorder.checkCalls != nil {
		(*recorder.checkCalls)++
	}
	return judge.Check(t.Context(), root)
}

func (r agreementRecorder) Fatalf(format string, args ...any) {
	panic(agreementSetupFailure{message: fmt.Sprintf(format, args...)})
}

func (r agreementRecorder) Fatal(args ...any) {
	panic(agreementSetupFailure{message: fmt.Sprint(args...)})
}

func (r agreementRecorder) Errorf(format string, args ...any) {
	panic(agreementSetupFailure{message: fmt.Sprintf(format, args...)})
}

func (r agreementRecorder) Error(args ...any) {
	panic(agreementSetupFailure{message: fmt.Sprint(args...)})
}

func (r agreementRecorder) Cleanup(cleanup func()) {
	r.T.Cleanup(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				failure, ok := recovered.(agreementSetupFailure)
				if !ok {
					panic(recovered)
				}
				r.T.Errorf("setup: observation cleanup: %s", failure.message)
			}
		}()
		cleanup()
	})
}

func agreementCapture(t *testing.T, observe func(agreementTB)) (setup string) {
	t.Helper()
	return agreementCaptureChecked(t, nil, observe)
}

func agreementCaptureChecked(t *testing.T, checks *int, observe func(agreementTB)) (setup string) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			failure, ok := recovered.(agreementSetupFailure)
			if !ok {
				panic(recovered)
			}
			setup = failure.message
		}
	}()
	observe(agreementRecorder{T: t, checkCalls: checks})
	return ""
}

func agreementIsolatedPage(t agreementTB, c agreementCase) (render.Result, agreementHTML) {
	t.Helper()
	var inputs []graph.NoteInput
	known := make(map[string]string)
	for path := range c.Companions {
		inputs = append(inputs, graph.NoteInput{RelPath: path})
		known[path] = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	slices.SortFunc(inputs, func(a, b graph.NoteInput) int {
		return strings.Compare(a.RelPath, b.RelPath)
	})
	page := render.New(graph.BuildFromNotes(inputs, nil), c.Companions, noTitlesDeclared{}, everyFileHeld{})
	result := page.HTML("Notes/Reading.md", c.Title, c.Body, wording.En)
	actual := agreementObserveKnown(t, result.HTML, known)
	if c.Title != "" {
		actual = agreementTitleHTML(t, c, &result)
	}
	return result, actual
}
