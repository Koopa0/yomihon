package pages

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/wording"
)

// sectionBody is a rendered body as the renderer writes one: the authored level
// stamped on each heading, a question that starts closed under the first, a
// callout of another type, a heading the outline left out because it sits in a
// closed fold, and an anchor inside the first section for a kept place to name.
const sectionBody = `<p>lede</p>
<h3 id="first" data-level="2" data-mark-anchor>First</h3>
<p id="held" data-mark-anchor>one</p>
<details class="callout callout-note" data-callout="question"><summary class="callout-title">Q</summary><div class="callout-body"><p>a</p></div></details>
<details class="callout callout-note" data-callout="note"><summary class="callout-title"><span class="callout-type" lang="zh-Hant">備註</span></summary><div class="callout-body"><p>b</p></div></details>
<h4 id="second" data-level="3" data-mark-anchor>Second</h4>
<p>two</p>
<details class="callout callout-note" data-callout="question" open><summary class="callout-title">Open</summary><div class="callout-body"><p>c</p></div></details>
<h4 id="folded" data-level="3">Inside a closed callout</h4>
`

func probePlan(lang wording.Lang) *SectionPlan {
	return &SectionPlan{
		Lang:    lang,
		RelPath: "Notes/Probe.md",
		Marks:   true,
		Door:    true,
		Sections: map[string]SectionMargin{
			"first":  {Text: "First"},
			"second": {Text: "Second"},
		},
	}
}

func inject(t *testing.T, body string, plan *SectionPlan) string {
	t.Helper()
	got, err := InjectSections(t.Context(), body, plan)
	if err != nil {
		t.Fatalf("InjectSections: %v", err)
	}
	return got
}

// TestASectionStandsInARowWithItsMarginAndNothingElseMoves holds the markup of
// one section byte for byte, and that laying the plan over a body changes
// nothing but what it adds: take the wrappers, the margins and the prompt back
// out and the body is the one that went in.
func TestASectionStandsInARowWithItsMarginAndNothingElseMoves(t *testing.T) {
	t.Parallel()

	plan := &SectionPlan{
		Lang:     wording.ZhHant,
		RelPath:  "Notes/Probe.md",
		Marks:    true,
		Door:     true,
		Sections: map[string]SectionMargin{"first": {Text: "First"}},
	}
	got := inject(t, `<h3 id="first" data-level="2">First</h3><p>one</p>`, plan)
	want := `<div class="y-sect"><h3 id="first" data-level="2">First</h3>` +
		`<div class="y-margin" lang="zh-Hant" data-section-margin>` +
		`<button type="button" class="y-margin__act" data-section-mark data-section-anchor="first" aria-pressed="false" aria-label="還不確定" data-text="還不確定" disabled></button> ` +
		`<time class="y-margin__date" data-section-date hidden datetime=""></time> ` +
		`<a class="y-margin__act" href="/thought/Notes/Probe.md?section=first" aria-label="留下自己的想法：First" data-text="留下自己的想法"></a> ` +
		`<p class="y-margin__place" data-section-place hidden data-text="你留下待續位置的地方"></p>` +
		`<p class="y-reply y-margin__said" role="status" aria-live="polite" aria-atomic="true" data-section-said=""></p>` +
		`</div></div><p>one</p>`
	if got != want {
		t.Errorf("InjectSections(one heading) =\n%s\nwant\n%s", got, want)
	}

	whole := inject(t, sectionBody, probePlan(wording.ZhHant))
	for _, added := range []*regexp.Regexp{
		regexp.MustCompile(`<div class="y-sect">`),
		regexp.MustCompile(`(?s)<div class="y-margin".*?</div>`),
		regexp.MustCompile(`(?s)<p class="y-predict".*?</p>`),
	} {
		whole = added.ReplaceAllString(whole, "")
	}
	whole = strings.ReplaceAll(strings.ReplaceAll(whole, "</h3></div>", "</h3>"), "</h4></div>", "</h4>")
	if whole != sectionBody {
		t.Errorf("laying the plan over the body changed what it did not add:\n%s\nwant\n%s", whole, sectionBody)
	}
}

// TestOnlyTheHeadingsThePlanNamesGetAMargin holds that a heading the outline
// left out — one inside a closed fold, or in an embedded excerpt — stays as the
// renderer wrote it, and that a tag that is not the renderer's own heading is
// never taken for one: the renderer stamps the authored level, which markup an
// author wrote cannot carry.
func TestOnlyTheHeadingsThePlanNamesGetAMargin(t *testing.T) {
	t.Parallel()

	got := inject(t, sectionBody+`<h3 id="first-again">Plain</h3>`, probePlan(wording.ZhHant))
	if strings.Contains(got, `<div class="y-sect"><h4 id="folded"`) || strings.Count(got, `class="y-sect"`) != 2 {
		t.Errorf("a heading outside the plan was wrapped (or an unstamped tag was): %d wrappers\n%s", strings.Count(got, `class="y-sect"`), got)
	}
}

// TestNotSureYetIsOneFixedNameWithItsStatePressed holds the control's contract:
// the name never changes with the state, the pressed attribute carries it, the
// day appears only on a marked section, and a plan that keeps no marks draws no
// control at all.
func TestNotSureYetIsOneFixedNameWithItsStatePressed(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		lang wording.Lang
		name string
	}{
		{wording.ZhHant, "還不確定"},
		{wording.En, "Not sure yet"},
	} {
		plan := probePlan(tc.lang)
		plan.Sections["first"] = SectionMargin{Text: "First", Marked: true, MarkedOn: time.Date(2026, 11, 10, 9, 30, 0, 0, time.UTC)}
		got := inject(t, sectionBody, plan)

		marked := `<button type="button" class="y-margin__act" data-section-mark data-section-anchor="first" aria-pressed="true" aria-label="` + tc.name + `" data-text="` + tc.name + `" disabled></button> ` +
			`<time class="y-margin__date" data-section-date datetime="2026-11-10"></time>`
		unmarked := `<button type="button" class="y-margin__act" data-section-mark data-section-anchor="second" aria-pressed="false" aria-label="` + tc.name + `" data-text="` + tc.name + `" disabled></button> ` +
			`<time class="y-margin__date" data-section-date hidden datetime=""></time>`
		for _, want := range []string{marked, unmarked} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: the page does not carry %q", tc.lang.Tag(), want)
			}
		}

		plan.Marks = false
		if got := inject(t, sectionBody, plan); strings.Contains(got, "data-section-mark") || strings.Contains(got, "<time") {
			t.Errorf("%s: a page that keeps no marks still draws the control or a day:\n%s", tc.lang.Tag(), got)
		}
	}
}

// TestEverySectionHasItsDoorNamedForIt holds R03: the link beside a section is
// named with its visible words and the section's own, in the interface's
// language, and opens the hand-off for that section's address, which is the
// heading's own id without the column's.
func TestEverySectionHasItsDoorNamedForIt(t *testing.T) {
	t.Parallel()

	plan := probePlan(wording.En)
	got := inject(t, sectionBody, plan)
	for _, want := range []string{
		`<a class="y-margin__act" href="/thought/Notes/Probe.md?section=first" aria-label="Leave a thought: First" data-text="Leave a thought"></a>`,
		`<a class="y-margin__act" href="/thought/Notes/Probe.md?section=second" aria-label="Leave a thought: Second" data-text="Leave a thought"></a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the page does not carry the door %q", want)
		}
	}

	column := &SectionPlan{
		Lang: wording.ZhHant, RelPath: "Notes/Probe.md", IDPrefix: "col-", Marks: true, Door: true,
		Sections: map[string]SectionMargin{"col-first": {Text: "First"}},
	}
	got = inject(t, `<h3 id="col-first" data-level="2">First</h3>`, column)
	for _, want := range []string{`href="/thought/Notes/Probe.md?section=first"`, `data-section-anchor="first"`} {
		if !strings.Contains(got, want) {
			t.Errorf("a column's section does not name its own address without the column's prefix: missing %q in\n%s", want, got)
		}
	}

	plan.Door = false
	if got := inject(t, sectionBody, plan); strings.Contains(got, "/thought/") {
		t.Errorf("a vault with no place to write a thought still draws the link:\n%s", got)
	}
}

// TestAClosedQuestionIsPrefacedByOneLineAndNothingAsksForAnAnswer holds the
// prompt: one line directly above a question that starts closed, in the section
// it is in, saying a guess is welcome before the answer opens and offering the
// one link; nothing above an open question or another type; and nothing where
// there is no place to write.
func TestAClosedQuestionIsPrefacedByOneLineAndNothingAsksForAnAnswer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		lang wording.Lang
		line string
	}{
		{wording.ZhHant, `<p class="y-predict" lang="zh-Hant">看答案前，可以先試著回答 <a href="/thought/Notes/Probe.md?section=first">寫成想法</a></p><details class="callout callout-note" data-callout="question"><summary`},
		{wording.En, `<p class="y-predict" lang="en">You can try answering before you open the answer <a href="/thought/Notes/Probe.md?section=first">Write it as a thought</a></p><details class="callout callout-note" data-callout="question"><summary`},
	} {
		got := inject(t, sectionBody, probePlan(tc.lang))
		if strings.Count(got, `class="y-predict"`) != 1 {
			t.Errorf("%s: %d prompts, want one: for the closed question only, none for the open question or the note\n%s", tc.lang.Tag(), strings.Count(got, `class="y-predict"`), got)
		}
		if !strings.Contains(got, tc.line) {
			t.Errorf("%s: the prompt does not stand directly above the closed question as %q\n%s", tc.lang.Tag(), tc.line, got)
		}
	}

	plan := probePlan(wording.ZhHant)
	plan.Door = false
	if got := inject(t, sectionBody, plan); strings.Contains(got, "y-predict") {
		t.Errorf("a vault with no place to write draws the prompt anyway:\n%s", got)
	}

	lede := inject(t, `<details class="callout callout-note" data-callout="question"><summary class="callout-title">Q</summary></details><h3 id="first" data-level="2">First</h3>`, probePlan(wording.ZhHant))
	if !strings.Contains(lede, `<a href="/thought/Notes/Probe.md">寫成想法</a>`) {
		t.Errorf("a question above every heading does not offer a thought about the note as a whole:\n%s", lede)
	}
}

// TestWhatTheReaderLeftIsShownAndNothingOfItIsHidden holds the record the
// reader meets again: the first sentence of each thought of theirs about a
// section, in the words it was written in, with the day, as a link to the thought,
// and the place they left off drawn once in the section that holds it. None of it
// is hidden from a screen reader or taken out of the tab order.
func TestWhatTheReaderLeftIsShownAndNothingOfItIsHidden(t *testing.T) {
	t.Parallel()

	plan := probePlan(wording.ZhHant)
	plan.Sections["first"] = SectionMargin{Text: "First", Thoughts: []ThoughtLine{
		{Href: "/notes/Notes/T1.md", Opening: "我原本以為 cancel() 之後就停了。", Language: "zh-Hant", On: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)},
		{Href: "/notes/Notes/T2.md", Opening: "Second thought, undated."},
	}}
	plan.PlaceAnchor = "held"
	got := inject(t, sectionBody, plan)

	for _, want := range []string{
		`<p class="y-margin__mine"><span class="y-margin__tag" lang="zh-Hant" data-text="想法"></span> <time datetime="2026-10-31"></time> <a class="y-margin__opening" href="/notes/Notes/T1.md" aria-label="我原本以為 cancel() 之後就停了。" data-text="我原本以為 cancel() 之後就停了。" lang="zh-Hant"></a></p>`,
		`<p class="y-margin__mine"><span class="y-margin__tag" lang="zh-Hant" data-text="想法"></span> <a class="y-margin__opening" href="/notes/Notes/T2.md" aria-label="Second thought, undated." data-text="Second thought, undated."></a></p>`,
		`<p class="y-margin__place" data-section-place data-text="你留下待續位置的地方"></p>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the margin does not carry %q\n%s", want, got)
		}
	}
	if strings.Count(got, `data-section-place data-text=`) != 1 || strings.Count(got, `data-section-place hidden data-text=`) != 1 {
		t.Errorf("the kept place is not drawn once, in the section holding it, with the other left hidden:\n%s", got)
	}
	for _, margin := range regexp.MustCompile(`(?s)<div class="y-margin".*?</div>`).FindAllString(got, -1) {
		for _, hidden := range []string{"aria-hidden", `tabindex="-1"`} {
			if strings.Contains(margin, hidden) {
				t.Errorf("the margin hides what the reader left (%s):\n%s", hidden, margin)
			}
		}
	}

	// Where the page offers neither the mark nor the link, only a section that
	// holds something the reader left has a margin to draw.
	plan.Marks, plan.Door = false, false
	got = inject(t, sectionBody, plan)
	if strings.Count(got, `class="y-sect"`) != 1 || strings.Contains(got, "data-section-mark") {
		t.Errorf("with no controls to offer, %d sections drew a margin, want only the one holding thoughts:\n%s", strings.Count(got, `class="y-sect"`), got)
	}
}

// TestABodyThePlanHasNothingToAddIsHandedBackUntouched holds that a plan with
// nothing to draw is the body itself, byte for byte.
func TestABodyThePlanHasNothingToAddIsHandedBackUntouched(t *testing.T) {
	t.Parallel()

	plan := probePlan(wording.ZhHant)
	plan.Marks, plan.Door = false, false
	if got := inject(t, sectionBody, plan); got != sectionBody {
		t.Errorf("a plan with nothing to draw changed the body:\n%s", got)
	}
	if got, err := InjectSections(t.Context(), sectionBody, nil); err != nil || got != sectionBody {
		t.Errorf("no plan changed the body or failed: %v\n%s", err, got)
	}
}

// TestASectionHoldsAClosedQuestionOnlyUnderItsOwnHeading holds the scope the
// draft's opening line is decided by: a question belongs to the section it is
// under, with the sections nested beneath that one, and to the note as a whole
// where no section is named.
func TestASectionHoldsAClosedQuestionOnlyUnderItsOwnHeading(t *testing.T) {
	t.Parallel()

	const body = `<h3 id="a" data-level="2">A</h3><h4 id="a1" data-level="3">A1</h4><details data-callout="question"><summary>Q</summary></details>` +
		`<h3 id="b" data-level="2">B</h3><details data-callout="question" open><summary>Q</summary></details><details data-callout="note"><summary>N</summary></details>` +
		`<h3 id="c" data-level="2">C</h3><details data-callout="question"><summary>Q</summary></details>`
	for _, tc := range []struct {
		section string
		want    bool
	}{
		{"", true},
		{"a", true},
		{"a1", true},
		{"b", false},
		{"c", true},
		{"missing", false},
	} {
		if got := HoldsClosedQuestion(body, tc.section); got != tc.want {
			t.Errorf("HoldsClosedQuestion(body, %q) = %v, want %v", tc.section, got, tc.want)
		}
	}
}
