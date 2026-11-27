package judge

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// The fragment rules must judge a link's "#section" and "#^block" half the
// way the reading page judges it: same fold, same generosity, same one level
// of transclusion, and silence about everything the page is silent about.
// These tests pin the agreement case by case; the golden fixture pins the
// wire bytes over a whole vault.

// fragmentRun parses the given files as a vault, resolves them, and returns
// the fragment findings alone.
func fragmentRun(t *testing.T, files map[string]string) []Finding {
	t.Helper()
	var notes []note
	var resources []string
	for path, body := range files {
		if strings.HasSuffix(path, ".md") {
			notes = append(notes, parseNote(path, []byte(body)))
		} else {
			resources = append(resources, path)
		}
	}
	return newFragmentPaths(notes, nil, buildIndex(notes, nil, resources)).fragmentFindings(notes)
}

// linkAndEmbed cites address from Notes/Citer.md once as a link and once as a
// transclusion, beside files, and compares each run's fragment findings with
// the one rule it should draw, "" for none.
func linkAndEmbed(t *testing.T, files map[string]string, address, wantLink, wantEmbed string) {
	t.Helper()
	for _, face := range []struct{ cite, want string }{
		{cite: "[[" + address + "]]\n", want: wantLink},
		{cite: "![[" + address + "]]\n", want: wantEmbed},
	} {
		vault := map[string]string{"Notes/Citer.md": face.cite}
		maps.Copy(vault, files)
		var want []string
		if face.want != "" {
			want = []string{face.want + " " + address}
		}
		if diff := cmp.Diff(want, findingKeys(fragmentRun(t, vault))); diff != "" {
			t.Errorf("%q fragment findings (-want +got):\n%s", face.cite, diff)
		}
	}
}

// A link's section is judged the way the page resolves it, generously; an
// embed's the way the page cuts an excerpt, by one exact line scan over the
// embedded note's own body. The rows show where the two agree and where an
// embed misses a section a link to the same name reaches.
func TestFragmentSectionMatchingMirrorsTheReadingPage(t *testing.T) {
	t.Parallel()

	const linkMiss, embedMiss = "link.section_missing", "embed.section_missing"
	files := map[string]string{
		"Notes/Other.md": "other\n",
		"Notes/Target.md": "# Top\n" +
			"## Real Section\n" +
			"words\n" +
			"## Twice\n" +
			"## Twice\n" +
			"> ## Quoted Heading\n" +
			"\n" +
			"- ## Listed Heading\n" +
			"\n" +
			"Setext Name\n" +
			"===\n" +
			"\n" +
			"## About [[Other|Bar]]\n" +
			"## がん\n" +
			"## Result<T> 的取捨\n" +
			"## 讀本 ①　<ruby>加藤<rt>かとう</rt></ruby>さん\n" +
			"\n" +
			"```\n" +
			"# Fenced Words\n" +
			"```\n" +
			"\n" +
			"<div>\n" +
			"# Blocked Words\n" +
			"</div>\n" +
			"\n" +
			"%%\n" +
			"## Hidden Heading\n" +
			"%%\n",
	}
	for _, tt := range []struct {
		name, address string
		link, embed   string
	}{
		{name: "a missing section is reported", address: "Target#Nowhere", link: linkMiss, embed: embedMiss},
		{name: "an existing section is not", address: "Target#Real Section"},
		{name: "case folds the way the page's ids fold", address: "Target#REAL SECTION"},
		{name: "punctuation folds the way the page's ids fold", address: "Target#real, section!"},
		{name: "any one of a repeated heading answers", address: "Target#Twice"},
		{name: "a heading inside a quote answers a link but is not cut to", address: "Target#Quoted Heading", embed: embedMiss},
		{name: "a heading inside a list item answers a link but is not cut to", address: "Target#Listed Heading", embed: embedMiss},
		{name: "an underlined heading answers", address: "Target#Setext Name"},
		{name: "a heading citing a note answers by its display words", address: "Target#About Bar"},
		{name: "the display words are the name, not the bracket text", address: "Target#About Other Bar", link: linkMiss, embed: embedMiss},
		{name: "a heading naming a generic type answers", address: "Target#Result<T> 的取捨"},
		{name: "a ruby heading named by its rendered text answers", address: "Target#讀本 ①　加藤さん"},
		{name: "a decomposed spelling reaches a composed heading", address: "Target#がん"},
		{name: "a heading shape inside a fence stays out of link reports and is code to an excerpt", address: "Target#Fenced Words", embed: embedMiss},
		{name: "a heading shape inside an authored html block is text to an excerpt", address: "Target#Blocked Words", embed: embedMiss},
		{name: "a heading hidden in a comment is not on the page", address: "Target#Hidden Heading", link: linkMiss, embed: embedMiss},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			linkAndEmbed(t, files, tt.address, tt.link, tt.embed)
		})
	}
}

// A course declares each branch's part in the course order at the end of
// the heading that opens it, and the reading page takes the declaration off
// before stamping the section's id. This face answers about that same page, so
// a citation naming a branch by the name a reader sees reaches it, and one
// spelling the declaration out reaches no id the page stamps.
//
// A link and an excerpt are answered from different readings of the same note:
// a link may reach an id either reading found, while an excerpt is cut by the
// line scan alone. So a branch every reading has to name the same way is cited
// both ways, and one only the generous reading finds is cited as a link, which
// is the only address that reaches it.
func TestFragmentSectionMatchingReadsABranchByItsName(t *testing.T) {
	t.Parallel()

	target := "# 課程\n" +
		"\n" +
		"## 基本觀念 {sequence=primary}\n" +
		"\n" +
		"words\n" +
		"\n" +
		"## 宣告 `{sequence=primary}`\n" +
		"\n" +
		"words\n" +
		"\n" +
		"不分先後 {sequence=none}\n" +
		"---\n" +
		"\n" +
		"## 收尾 {sequence=primary} ##\n" +
		"\n" +
		"words\n" +
		"\n" +
		// A heading shape inside a fence is code to the parser and to the scan
		// that cuts an excerpt, and a heading to the generous line reading the
		// page runs before it claims a section is missing. Only that reading
		// can name this branch, which is what makes it name it here.
		"```\n" +
		"\n" +
		"只有寬讀認得 {sequence=local}\n" +
		"---\n" +
		"```\n"
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "a branch answers to the name the course lists it under", body: "[[Target#基本觀念]]\n"},
		{name: "an underlined branch declares the same way", body: "[[Target#不分先後]]\n"},
		{name: "a role quoted in code is words about the grammar", body: "[[Target#宣告 {sequence=primary}]]\n"},
		{name: "a run of marks closing the heading is not part of the name either", body: "[[Target#收尾]]\n"},
		{name: "an excerpt is cut to a branch closed by a run of marks", body: "![[Target#收尾]]\n"},
		{name: "an excerpt is cut to a branch by the name the course gives it", body: "![[Target#基本觀念]]\n"},
		{name: "an excerpt is cut to an underlined branch the same way", body: "![[Target#不分先後]]\n"},
		{name: "only the generous reading finds this branch, and names it the same", body: "[[Target#只有寬讀認得]]\n"},
		{
			name: "the declaration is not part of the name",
			body: "[[Target#基本觀念 {sequence=primary}]]\n",
			want: []string{"link.section_missing Target#基本觀念 {sequence=primary}"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := findingKeys(fragmentRun(t, map[string]string{
				"Notes/Target.md": target,
				"Notes/Citer.md":  tt.body,
			}))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("fragment findings mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFragmentBlockMatchingMirrorsTheReadingPage(t *testing.T) {
	t.Parallel()

	const linkMiss, embedMiss = "link.block_missing", "embed.block_missing"
	files := map[string]string{
		"Notes/Target.md": "## Real Section\n" +
			"\n" +
			"some words ^blk1\n" +
			"\n" +
			"| cell | row | ^tabled\n" +
			"\n" +
			"```\n" +
			"code ^fenced\n" +
			"```\n" +
			"\n" +
			"> quoted words ^quoted\n" +
			"\n" +
			// Two stray backticks with a heading between them are two blocks,
			// not one span, so the paragraph in the middle keeps its address.
			"Before `\n" +
			"## Heading\n" +
			"a paragraph ^crossblock\n" +
			"## Other\n" +
			"After `\n",
	}
	for _, tt := range []struct {
		name, address string
		link, embed   string
	}{
		{name: "a missing block is reported", address: "Target#^ghost", link: linkMiss, embed: embedMiss},
		{name: "an address between two blocks holding stray backticks answers", address: "Target#^crossblock"},
		{name: "an existing address is not", address: "Target#^blk1"},
		{name: "the bare caret spelling reads the same address", address: "Target^blk1"},
		{name: "the address folds case like every fragment", address: "Target#^BLK1"},
		{name: "an address at the end of a quoted line answers", address: "Target#^quoted"},
		{name: "an address on a table row is not an anchor", address: "Target#^tabled", link: linkMiss, embed: embedMiss},
		{name: "an address inside a fence is code", address: "Target#^fenced", link: linkMiss, embed: embedMiss},
		{name: "a block beats a section when both are written", address: "Target^ghost2#Real Section", link: linkMiss, embed: embedMiss},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			linkAndEmbed(t, files, tt.address, tt.link, tt.embed)
		})
	}
}

func TestFragmentRulesJudgeOnlyUniquelyResolvedNotes(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		files       map[string]string
		address     string
		link, embed string
	}{
		{name: "a broken name is the broken-link rule's alone", address: "Ghost#Section"},
		{
			name:    "an ambiguous name is the collision rules' alone",
			files:   map[string]string{"Notes/Dup.md": "a\n", "Other/Dup.md": "b\n"},
			address: "Dup#Section",
		},
		{name: "a picture has no sections", files: map[string]string{"Notes/shot.png": ""}, address: "shot.png#Section"},
		{name: "a same-file fragment is never resolved at all", address: "#Nowhere"},
		{name: "a whole note names no fragment", files: map[string]string{"Notes/Target.md": "words\n"}, address: "Target"},
		{
			name:    "a fragment on a resolved note reports under its own rule",
			files:   map[string]string{"Notes/Target.md": "words\n"},
			address: "Target#Nowhere",
			link:    "link.section_missing",
			embed:   "embed.section_missing",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			linkAndEmbed(t, tt.files, tt.address, tt.link, tt.embed)
		})
	}
}

func TestATranscludedNoteAnswersForItsHostsSections(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"Notes/Citer.md":  "[[Host#Inner Heading]]\n[[Host#Deeper Heading]]\n",
		"Notes/Host.md":   "![[Inner]]\n",
		"Notes/Inner.md":  "## Inner Heading\n![[Deeper]]\n",
		"Notes/Deeper.md": "## Deeper Heading\n",
	}
	got := findingKeys(fragmentRun(t, files))
	// The page expands one level, so Inner's headings are on Host's page and
	// Deeper's are not: a citation into the first stays quiet and a citation
	// into the second is a real miss.
	want := []string{"link.section_missing Host#Deeper Heading"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("fragment findings mismatch (-want +got):\n%s", diff)
	}
}

// TestFragmentFindingsNameTheAddressUnderTheirOwnRule holds what a fragment
// finding owes the wire beyond its golden: the target is the written address
// without the display words, and a link and a transclusion of one address
// carry different fingerprints, so a baseline that subtracts one keeps the
// other.
func TestFragmentFindingsNameTheAddressUnderTheirOwnRule(t *testing.T) {
	t.Parallel()

	findings := fragmentRun(t, map[string]string{
		"Notes/Citer.md":  "[[Target#Nowhere|shown words]]\n![[Target#Nowhere|shown words]]\n",
		"Notes/Target.md": "words\n",
	})
	want := []string{"link.section_missing Target#Nowhere", "embed.section_missing Target#Nowhere"}
	if diff := cmp.Diff(want, findingKeys(findings)); diff != "" {
		t.Fatalf("fragment findings (-want +got):\n%s", diff)
	}
	if findings[0].Fingerprint == findings[1].Fingerprint {
		t.Errorf("the link and embed findings for one address share fingerprint %q", findings[0].Fingerprint)
	}
}

// An anchor copied from the rendered heading reaches that heading, as a link
// and as an excerpt.
func TestHeadingAttributeFragments(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, body, address string
	}{
		{name: "double quoted underline", body: "## <u title=\"<\">a</u>\n", address: "u-title-a"},
		{name: "single quoted span", body: "## <span title='a<b'>x</span>\n", address: "span-title-a-b-x-span"},
		{name: "single quoted underline", body: "## <u title='<'>a</u>\n", address: "u-title-a"},
		{name: "double quoted span", body: "## <span title=\"a<b\">x</span>\n", address: "span-title-a-b-x-span"},
		{name: "utf8 prefix", body: "## 漢 <u title=\"<\">a</u>\n", address: "漢-u-title-a"},
		{name: "wikilink alias prefix", body: "## [[Other|日本]] <u title=\"<\">a</u>\n", address: "日本-u-title-a"},
		{name: "multiline setext", body: "Setext <u title=\"<\">a</u>\nsecond line\n===\n", address: "setext-u-title-a-second-line"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			linkAndEmbed(t, map[string]string{
				"Notes/Target.md": tt.body + "\nINSIDE\n",
				"Notes/Other.md":  "Other words.\n",
			}, "Target#"+tt.address, "", "")
		})
	}
}

// TestHeadingAttributeFragmentsRunCheck holds the same through the check
// command: the copied anchor is quiet and a missing section is still reported.
func TestHeadingAttributeFragmentsRunCheck(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	write(t, root, "Concepts/golang/Target.md", "## <u title=\"<\">a</u>\n\nINSIDE\n")
	write(t, root, "Concepts/golang/Citer.md", "[[Target#u-title-a]]\n![[Target#u-title-a]]\n[[Target#Missing Section]]\n![[Target#Missing Section]]\n")
	writeTestContract(t, root, nil)
	out, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
	if err != nil {
		t.Fatalf("RunCheck() error = %v", err)
	}
	want := []string{"link.section_missing Target#Missing Section", "embed.section_missing Target#Missing Section"}
	if diff := cmp.Diff(want, findingKeys(decodeFindings(t, out))); diff != "" {
		t.Errorf("RunCheck() findings (-want +got):\n%s", diff)
	}
}

// TestATransclusionDoesNotAnswerForWhatItsHostEmbeds pins the one level the
// two rule families differ on. The page cuts an excerpt from the embedded
// note's own body and expands nothing inside it while looking, so a heading
// that reaches a link through the host's transclusion never reaches an embed
// of the same address: the link stays quiet and the embed is a miss.
func TestATransclusionDoesNotAnswerForWhatItsHostEmbeds(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"Notes/Citer.md": "[[Host#Inner Heading]]\n![[Host#Inner Heading]]\n",
		"Notes/Host.md":  "![[Inner]]\n",
		"Notes/Inner.md": "## Inner Heading\n",
	}
	got := findingKeys(fragmentRun(t, files))
	want := []string{"embed.section_missing Host#Inner Heading"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("fragment findings mismatch (-want +got):\n%s", diff)
	}
}

// TestExtractKeepsTheFragmentBesideTheTarget pins the extraction half: the
// resolution target stays exactly what it always was, and the fragment
// arrives beside it split the way the resolver splits it.
func TestExtractKeepsTheFragmentBesideTheTarget(t *testing.T) {
	t.Parallel()

	links := extractWikilinks("[[Note#Sec|words]] and [[Note#^blk]] and ![[Note#Part]]\n", 1)
	want := []wikiLink{
		{target: "Note", address: "Note#Sec", heading: "Sec", offset: 0, start: 0, end: 18, line: 1},
		{target: "Note", address: "Note#^blk", block: "blk", offset: 23, start: 23, end: 36, line: 1},
		{target: "Note", address: "Note#Part", heading: "Part", embed: true, offset: 42, start: 41, end: 55, line: 1},
	}
	if diff := cmp.Diff(want, links, cmp.AllowUnexported(wikiLink{})); diff != "" {
		t.Errorf("extractWikilinks mismatch (-want +got):\n%s", diff)
	}
}

// The same indented example, on the face that decides whether a link's
// heading address resolves. Read as an opening fence it hides every heading
// below it, so an address a note really carries is reported as one it does
// not have.
func TestAnchorSurfaceKeepsAHeadingBelowAnIndentedFenceExample(t *testing.T) {
	t.Parallel()

	const below = "## Later Heading\n\ntext under it\n"
	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "an indented backtick example", body: "# Title\n\n    ```\n\n" + below},
		{name: "an indented tilde example", body: "# Title\n\n    ~~~\n\n" + below},
		{name: "ordinary indented text, the control", body: "# Title\n\n    marker\n\n" + below},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, excerpt, _ := anchorSurface(tt.body)
			if !excerpt["later-heading"] {
				t.Errorf("anchorSurface() excerpt headings = %v, want later-heading among them", excerpt)
			}
		})
	}
}

// The same note on the face that decides which addresses a note carries. A
// marker written four spaces in is content CommonMark keeps inside the fence,
// so a walk that ends the block there loses the heading the note really has
// and credits it with one made out of code text.
func TestAnchorSurfaceReadsTheHeadingsAFenceReallyLeaves(t *testing.T) {
	t.Parallel()

	const body = "```\n    ```\n## Code Example\n```\n\n## Real Heading\nreal section body\n"

	_, excerpt, _ := anchorSurface(body)
	if !excerpt["real-heading"] {
		t.Errorf("anchorSurface() excerpt headings = %v, want real-heading among them", excerpt)
	}
	if excerpt["code-example"] {
		t.Errorf("anchorSurface() excerpt headings = %v, want no heading made of code text", excerpt)
	}
}

func TestFragmentSectionAcceptsTheGitHubSpellingThePageFollows(t *testing.T) {
	t.Parallel()
	got := fragmentRun(t, map[string]string{
		"Target.md": "## What's new?\n\n## Intro\n\n## Intro\n\n## Intro\n",
		"Citer.md":  "[[Target#whats-new]]\n[[Target#intro-1]]\n[[Target#intro-3]]\n[[Target#nothing-here]]\n",
	})
	want := []string{"link.section_missing Target#nothing-here"}
	if diff := cmp.Diff(want, findingKeys(got)); diff != "" {
		t.Errorf("fragment findings (-want +got):\n%s", diff)
	}
}

func TestBracketsInANameMakeNoLinkForTheCheckEither(t *testing.T) {
	t.Parallel()
	if got := extractWikilinks("pairs [[1,2],[1,100]] and [[Real]]\n", 1); len(got) != 1 || got[0].target != "Real" {
		t.Errorf("extractWikilinks read %+v, want only [[Real]]", got)
	}
	root := t.TempDir()
	write(t, root, "Notes/Citer.md", "pairs [[1,2],[1,100]]\n")
	writeTestContract(t, root, nil)
	findings, err := runCheckAction(t.Context(), root, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := range findings {
		if strings.HasPrefix(string(findings[i].RuleID), "link.") {
			t.Errorf("an array of numbers drew a link finding: %+v", findings[i])
		}
	}
}

// TestAHeadingCarryingABlockAddressHasOneIdOnBothFaces holds that the id the
// page stamps on such a heading is the id check judges a link against: the
// address is not part of the heading's words on either face.
func TestAHeadingCarryingABlockAddressHasOneIdOnBothFaces(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"T.md": "# T\n\n## Sub heading ^sub-one\n\ntext\n",
		"S.md": "[[T#Sub heading]]\n\n[[T#Sub heading ^sub-one]]\n\n[[T#^sub-one]]\n",
	}
	got := findingKeys(fragmentRun(t, files))
	// The words alone name the heading, as the page's own id does; the words
	// with the address after them name nothing on either face. The address
	// itself still names a block.
	if len(got) != 1 || !strings.Contains(got[0], "Sub heading ^sub-one") {
		t.Errorf("findings = %v, want only the link that spells the address into the heading's words", got)
	}
}

func TestFragmentMarksRemainDistinct(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, heading, missing string }{
		{"Hindi spacing mark", "की", "कि"},
		{"Thai nonspacing mark", "ปู่", "ปู"},
		{"Kana combining mark", "か゚", "か"},
		{"supplementary variation selector", "葛\U000e0100城", "葛城"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{"Notes/Target.md": "## " + tt.heading + "\n\nPassage.\n"}
			linkAndEmbed(t, files, "Target#"+tt.heading, "", "")
			linkAndEmbed(t, files, "Target#"+tt.missing, "link.section_missing", "embed.section_missing")
		})
	}
}

// TestFragmentNormalizationRunCheck holds through the check command that a
// fragment is matched in its canonical form, with a link read generously and
// an excerpt exactly.
func TestFragmentNormalizationRunCheck(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, target, citer string
		want                []string
	}{
		{name: "canonical heading and block", target: "## Y̊\n\nPASSAGE\n\nQuote. ^QUOTE-1\n", citer: "[[N#ẙ]]\n![[N#ẙ]]\n[[N#ẙ]]\n![[N#ẙ]]\n[[N#^quote-1]]\n![[N#^quote-1]]\n"},
		{name: "canonical heading path", target: "## Y̊\n### Y̊\nPASSAGE\n", citer: "[[N#ẙ#ẙ]]\n![[N#ẙ#ẙ]]\n"},
		{name: "generous link and exact embed", target: "```\n## Y̊\n```\n", citer: "[[N#ẙ]]\n![[N#ẙ]]\n", want: []string{"embed.section_missing N#ẙ"}},
		{name: "missing sections and blocks", target: "## Y̊\n\nQuote. ^QUOTE-1\n", citer: "[[N#Absent]]\n![[N#Absent]]\n[[N#^quote1]]\n![[N#^quote1]]\n", want: []string{"embed.block_missing N#^quote1", "embed.section_missing N#Absent", "link.block_missing N#^quote1", "link.section_missing N#Absent"}},
		{name: "hidden and fenced excerpts", target: "%%\n## Y̊\n%%\n\n```\n## Fenced\n```\n", citer: "![[N#ẙ]]\n![[N#Fenced]]\n", want: []string{"embed.section_missing N#Fenced", "embed.section_missing N#ẙ"}},
		{name: "closed parent", target: "## Y̊\n## Other\n### Child\n", citer: "[[N#ẙ#Child]]\n![[N#ẙ#Child]]\n", want: []string{"embed.section_missing N#ẙ#Child", "link.section_missing N#ẙ#Child"}},
		{name: "resource and ambiguous scope", target: "## Y̊\n", citer: "[[Asset.txt#ẙ]]\n[[Twin#ẙ]]\n"},
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
			out, _, err := checkBytes(t.Context(), &CheckOptions{Root: root, Format: FormatJSON})
			if err != nil {
				t.Fatalf("RunCheck() error = %v", err)
			}
			var got []string
			for _, f := range decodeFindings(t, out) {
				switch f.RuleID {
				case "link.section_missing", "embed.section_missing", "link.block_missing", "embed.block_missing":
					got = append(got, string(f.RuleID)+" "+*f.Target)
				default:
					// The other rules are not what this test is about.
				}
			}
			slices.Sort(got)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("RunCheck() fragment findings (-want +got):\n%s", diff)
			}
		})
	}
}
