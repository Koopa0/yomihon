package lesson_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/lesson"
)

func TestParseTemplate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tmpl string
		want []lesson.Segment
	}{
		{name: "particles", tmpl: "{A}は {B}です", want: []lesson.Segment{{Key: "A"}, {Text: "は "}, {Key: "B"}, {Text: "です"}}},
		{name: "adjacent", tmpl: "{A}{B}", want: []lesson.Segment{{Key: "A"}, {Key: "B"}}},
		{name: "repeated key", tmpl: "{A}と{A}", want: []lesson.Segment{{Key: "A"}, {Text: "と"}, {Key: "A"}}},
		{name: "no placeholders", tmpl: "no slots", want: []lesson.Segment{{Text: "no slots"}}},
		{name: "single", tmpl: "{N}", want: []lesson.Segment{{Key: "N"}}},
		{name: "leading literal", tmpl: "私は {B}", want: []lesson.Segment{{Text: "私は "}, {Key: "B"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, lesson.ParseTemplate(tt.tmpl)); diff != "" {
				t.Errorf("ParseTemplate(%q) mismatch (-want +got):\n%s", tt.tmpl, diff)
			}
		})
	}
}

func TestTemplateKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tmpl string
		want []string
	}{
		{name: "two keys", tmpl: "{A}は {B}です", want: []string{"A", "B"}},
		{name: "deduped in source order", tmpl: "{A}と{A}も{A}", want: []string{"A"}},
		{name: "multi-letter", tmpl: "{N} 很 {Adj}", want: []string{"N", "Adj"}},
		{name: "none", tmpl: "no keys", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, lesson.TemplateKeys(tt.tmpl)); diff != "" {
				t.Errorf("TemplateKeys(%q) mismatch (-want +got):\n%s", tt.tmpl, diff)
			}
		})
	}
}

func TestAbstractTemplate(t *testing.T) {
	t.Parallel()
	const in = "{A}は {B}です"
	const want = "Aは Bです"
	if got := lesson.AbstractTemplate(in); got != want {
		t.Errorf("AbstractTemplate(%q) = %q, want %q", in, got, want)
	}
}

// sampleA is the canonical two-slot pattern ("我 是 學生") used across the
// gloss/JSON tests — a hand-built literal, never derived from the code.
func sampleA() lesson.Pattern {
	return lesson.Pattern{
		ID:       "a-wa-b",
		Template: "{A}は {B}です",
		GlossZH:  "{A} 是 {B}",
		Slots: map[string]lesson.Position{
			"A": {Color: "topic", Fills: []lesson.Fill{{JP: "わたし", Reading: "わたし", ZH: "我"}}},
			"B": {Color: "pred", Fills: []lesson.Fill{{JP: "学生", Reading: "がくせい", ZH: "學生"}}},
		},
	}
}

func TestGlossInitial(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		gloss string
		want  string
	}{
		{name: "template keys substituted", gloss: "{A} 是 {B}", want: "我 是 學生"},
		{name: "non-template key stays literal", gloss: "{A} 是 {C}", want: "我 是 {C}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := sampleA()
			p.GlossZH = tt.gloss
			if got := lesson.GlossInitial(p); got != tt.want {
				t.Errorf("GlossInitial(%q) = %q, want %q", tt.gloss, got, tt.want)
			}
		})
	}
}

func TestFirstFill(t *testing.T) {
	t.Parallel()
	if diff := cmp.Diff(lesson.Fill{JP: "x"}, lesson.FirstFill(lesson.Position{Fills: []lesson.Fill{{JP: "x"}}})); diff != "" {
		t.Errorf("FirstFill(one fill) mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(lesson.Fill{}, lesson.FirstFill(lesson.Position{})); diff != "" {
		t.Errorf("FirstFill(empty) mismatch (-want +got):\n%s", diff)
	}
}

func TestPatternJSON(t *testing.T) {
	t.Parallel()
	js := lesson.PatternJSON(sampleA())
	for _, want := range []string{`"template":"{A}は {B}です"`, `"keys":["A","B"]`, `"jp":"わたし"`, `"zh":"學生"`} {
		if !strings.Contains(js, want) {
			t.Errorf("PatternJSON missing %q in:\n%s", want, js)
		}
	}

	// Script-tag safety: json.Marshal \u-escapes < > & so the blob cannot break
	// out of <script type="application/json"> even if a fill held markup.
	p := sampleA()
	p.Slots["B"] = lesson.Position{Fills: []lesson.Fill{{JP: "</script><b>", ZH: "x"}}}
	js = lesson.PatternJSON(p)
	for _, bad := range []string{"</script>", "<b>"} {
		if strings.Contains(js, bad) {
			t.Errorf("PatternJSON left literal %q (escaping failed, script-breakout risk):\n%s", bad, js)
		}
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	if problems := (&lesson.Sidecar{Patterns: []lesson.Pattern{sampleA()}}).Validate(); len(problems) != 0 {
		t.Errorf("valid sidecar reported problems: %v", problems)
	}

	// One pattern with four distinct faults: template key {B} has no slot; slot
	// A has no fills; slot A has an unknown colour; gloss key {C} is not in the
	// template. Validate returns exactly one message per fault.
	bad := lesson.Pattern{
		ID:       "bad",
		Template: "{A}は {B}です",
		GlossZH:  "{A} 是 {C}",
		Slots:    map[string]lesson.Position{"A": {Color: "nope", Fills: nil}},
	}
	if problems := (&lesson.Sidecar{Patterns: []lesson.Pattern{bad}}).Validate(); len(problems) != 4 {
		t.Errorf("Validate(bad) = %d problems, want 4: %v", len(problems), problems)
	}

	// An empty color is a neutral (uncoloured) slot, not a fault: only a
	// non-empty unknown token is rejected. This pins that behaviour so a
	// future tightening is a conscious decision, not a drift.
	neutral := lesson.Pattern{
		ID:       "neutral",
		Template: "{A}です",
		GlossZH:  "是 {A}",
		Slots:    map[string]lesson.Position{"A": {Color: "", Fills: []lesson.Fill{{JP: "x", ZH: "y"}}}},
	}
	if problems := (&lesson.Sidecar{Patterns: []lesson.Pattern{neutral}}).Validate(); len(problems) != 0 {
		t.Errorf("Validate(neutral empty-color) = %v, want no problems (empty color is a valid neutral slot)", problems)
	}
}

func slotSidecar(name, slug string) []byte {
	return []byte("lesson: " + strings.TrimSuffix(name, ".yaml") + "\nslug: " + slug + "\ntitle: t\npatterns: []\n")
}

func TestIsSlotSidecar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "direct YAML child", path: "System/slots/L01.yaml", want: true},
		{name: "nested YAML", path: "System/slots/archive/L01.yaml", want: false},
		{name: "wrong extension", path: "System/slots/L01.yml", want: false},
		{name: "wrong directory", path: "Writing/L01.yaml", want: false},
		{name: "directory itself", path: "System/slots", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := lesson.IsSlotSidecar(tt.path); got != tt.want {
				t.Errorf("IsSlotSidecar(%q) = %t, want %t", tt.path, got, tt.want)
			}
		})
	}
}

func TestNewSlotIndexJoinsBySlug(t *testing.T) {
	t.Parallel()
	// Filename and slug deliberately disagree: the index must key on the
	// in-file slug, not the L01/L02 filename.
	files := map[string][]byte{
		"System/slots/L01.yaml":        slotSidecar("L01.yaml", "jp-minna-l01"),
		"System/slots/L02.yaml":        slotSidecar("L02.yaml", "jp-minna-l02"),
		"System/slots/notes.md":        []byte("ignored"),
		"System/slots/nested/L03.yaml": slotSidecar("L03.yaml", "jp-minna-l03"),
		"System/other/L04.yaml":        slotSidecar("L04.yaml", "jp-minna-l04"),
	}

	idx, problems := lesson.NewSlotIndex(files)
	if len(problems) != 0 {
		t.Fatalf("NewSlotIndex() problems = %v", problems)
	}
	if idx.Len() != 2 {
		t.Errorf("NewSlotIndex indexed %d sidecars, want 2 direct YAML children", idx.Len())
	}
	got, ok := idx.Lookup("jp-minna-l01")
	if !ok {
		t.Fatalf("Lookup(jp-minna-l01) not found; index keyed by filename instead of slug?")
	}
	if got.Lesson != "L01" {
		t.Errorf("Lookup(jp-minna-l01).Lesson = %q, want %q", got.Lesson, "L01")
	}
	for _, slug := range []string{"jp-minna-l03", "jp-minna-l04", "jp-minna-l99"} {
		if _, ok := idx.Lookup(slug); ok {
			t.Errorf("Lookup(%q) reported a non-indexed sidecar", slug)
		}
	}
}

func TestSlotIndexLookupReturnsIndependentSidecar(t *testing.T) {
	t.Parallel()
	const sidecar = `lesson: L01
slug: jp-minna-l01
title: Original title
patterns:
  - id: p1
    template: "{A}です"
    gloss_zh: "是 {A}"
    slots:
      A:
        label_zh: 主題
        color: topic
        fills:
          - jp: わたし
            reading: わたし
            zh: 我
`
	idx, problems := lesson.NewSlotIndex(map[string][]byte{"System/slots/L01.yaml": []byte(sidecar)})
	if len(problems) != 0 {
		t.Fatalf("NewSlotIndex() problems = %v", problems)
	}

	first, ok := idx.Lookup("jp-minna-l01")
	if !ok {
		t.Fatal("Lookup(jp-minna-l01) not found")
	}
	first.Title = "mutated"
	first.Patterns[0].Template = "mutated"
	position := first.Patterns[0].Slots["A"]
	position.Fills[0].JP = "mutated"
	first.Patterns[0].Slots["A"] = position

	second, ok := idx.Lookup("jp-minna-l01")
	if !ok {
		t.Fatal("second Lookup(jp-minna-l01) not found")
	}
	if second.Title != "Original title" || second.Patterns[0].Template != "{A}です" || second.Patterns[0].Slots["A"].Fills[0].JP != "わたし" {
		t.Errorf("caller mutation changed the index-owned sidecar: %+v", second)
	}
}

func TestSlotIndexLookupIsSafeForConcurrentCallers(t *testing.T) {
	t.Parallel()
	const sidecar = `lesson: L01
slug: jp-minna-l01
title: Original title
patterns:
  - id: p1
    template: "{A}です"
    gloss_zh: "是 {A}"
    slots:
      A:
        label_zh: 主題
        color: topic
        fills:
          - jp: わたし
            reading: わたし
            zh: 我
`
	idx, problems := lesson.NewSlotIndex(map[string][]byte{"System/slots/L01.yaml": []byte(sidecar)})
	if len(problems) != 0 {
		t.Fatalf("NewSlotIndex() problems = %v", problems)
	}

	var callers sync.WaitGroup
	for range 32 {
		callers.Go(func() {
			got, ok := idx.Lookup("jp-minna-l01")
			if !ok {
				t.Error("Lookup(jp-minna-l01) not found")
				return
			}
			got.Title = "caller-owned"
			got.Patterns[0].Template = "caller-owned"
			position := got.Patterns[0].Slots["A"]
			position.Fills[0].JP = "caller-owned"
			got.Patterns[0].Slots["A"] = position
		})
	}
	callers.Wait()

	got, ok := idx.Lookup("jp-minna-l01")
	if !ok {
		t.Fatal("Lookup(jp-minna-l01) not found after concurrent callers")
	}
	if got.Title != "Original title" || got.Patterns[0].Template != "{A}です" || got.Patterns[0].Slots["A"].Fills[0].JP != "わたし" {
		t.Errorf("concurrent caller mutation changed the index-owned sidecar: %+v", got)
	}
}

func TestNewSlotIndexRejectsDuplicateSlugDeterministically(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{
		"System/slots/L01.yaml":      slotSidecar("L01.yaml", "jp-minna-l01"),
		"System/slots/L01-copy.yaml": slotSidecar("L01-copy.yaml", "jp-minna-l01"),
	}
	idx, problems := lesson.NewSlotIndex(files)
	if len(problems) != 1 {
		t.Fatalf("NewSlotIndex(duplicate slug) problems = %v, want exactly one", problems)
	}
	// The first sidecar in path order keeps the slug and the second is
	// reported: a name collision costs the file that arrived second, not both.
	if idx.Len() != 1 {
		t.Errorf("a duplicate slug cost both sidecars; Len() = %d, want 1", idx.Len())
	}
	const want = `slot slug jp-minna-l01 is already declared by System/slots/L01-copy.yaml`
	if problems[0].Message != want {
		t.Errorf("NewSlotIndex(duplicate slug) problem = %q, want %q", problems[0].Message, want)
	}
}

func TestNewSlotIndexRejectsMissingSlug(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{"System/slots/L01.yaml": []byte("lesson: L01\ntitle: t\npatterns: []\n")}
	if _, err := lesson.NewSlotIndex(files); err == nil {
		t.Fatal("NewSlotIndex accepted a sidecar with no slug")
	}
}

func TestNewSlotIndexRejectsInvalidSidecar(t *testing.T) {
	t.Parallel()
	const invalid = `lesson: L01
slug: jp-minna-l01
title: Invalid
patterns:
  - id: p1
    template: "{A}です"
    gloss_zh: "是 {A}"
    slots:
      A:
        label_zh: 主題
        color: unknown
        fills: []
`
	idx, problems := lesson.NewSlotIndex(map[string][]byte{"System/slots/L01.yaml": []byte(invalid)})
	if len(problems) == 0 {
		t.Fatal("NewSlotIndex accepted structurally invalid slot data")
	}
	if idx.Len() != 0 {
		t.Errorf("an unusable sidecar reached the index; Len() = %d, want 0", idx.Len())
	}
	// One message per problem, so a reader is told every fault in the file
	// rather than the first one.
	joined := strings.Join(problemMessages(problems), " | ")
	for _, want := range []string{
		"slot A has no fills",
		`slot A has unknown color "unknown"`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("NewSlotIndex problems = %q, want %q", joined, want)
		}
	}
	for _, p := range problems {
		if p.Source != "System/slots/L01.yaml" {
			t.Errorf("a problem names %q, want the file it came from", p.Source)
		}
	}
}

func TestNewSlotIndexOwnsCapturedBytes(t *testing.T) {
	t.Parallel()
	source := []byte("lesson: L01\nslug: jp-minna-l01\ntitle: Original\npatterns: []\n")
	files := map[string][]byte{"System/slots/L01.yaml": source}

	idx, problems := lesson.NewSlotIndex(files)
	if len(problems) != 0 {
		t.Fatalf("NewSlotIndex() problems = %v", problems)
	}

	copy(source, "lesson: XX")
	files["System/slots/L01.yaml"] = []byte("lesson: L02\nslug: jp-minna-l02\ntitle: Replacement\npatterns: []\n")

	got, ok := idx.Lookup("jp-minna-l01")
	if !ok {
		t.Fatal("Lookup(jp-minna-l01) not found after caller mutated captured input")
	}
	if got.Lesson != "L01" || got.Title != "Original" {
		t.Errorf("Lookup(jp-minna-l01) = %+v, want captured L01 sidecar", got)
	}
	if _, ok := idx.Lookup("jp-minna-l02"); ok {
		t.Error("Lookup(jp-minna-l02) found bytes installed after construction")
	}
}

func TestSlotIndexZeroValue(t *testing.T) {
	t.Parallel()
	var idx lesson.SlotIndex
	if idx.Len() != 0 {
		t.Errorf("zero SlotIndex.Len() = %d, want 0", idx.Len())
	}
	if got, ok := idx.Lookup("jp-minna-l01"); ok || got != nil {
		t.Errorf("zero SlotIndex.Lookup() = (%+v, %t), want (nil, false)", got, ok)
	}
}

// problemMessages flattens a generation's sidecar problems for assertion.
func problemMessages(problems lesson.Problems) []string {
	out := make([]string, 0, len(problems))
	for _, p := range problems {
		out = append(out, p.Message)
	}
	return out
}

// TestOneUnusableSidecarCostsOnlyItsOwnLesson pins what a file's own fault is
// allowed to cost. A single unknown key in one sidecar used to withhold the
// practice panel from every lesson in the vault — measured on the real one,
// twenty sidecars and none loaded — and the only trace was a line in the
// startup log that named one file and never said what had been lost.
func TestOneUnusableSidecarCostsOnlyItsOwnLesson(t *testing.T) {
	t.Parallel()

	idx, problems := lesson.NewSlotIndex(map[string][]byte{
		"System/slots/L01.yaml": slotSidecar("L01.yaml", "jp-minna-l01"),
		"System/slots/L02.yaml": []byte("lesson: L02\nslug: jp-minna-l02\ntitle: \"broken\"\nunknown_key: x\n"),
		"System/slots/L03.yaml": slotSidecar("L03.yaml", "jp-minna-l03"),
	})

	if idx.Len() != 2 {
		t.Errorf("one unreadable sidecar cost the others; Len() = %d, want 2", idx.Len())
	}
	for _, slug := range []string{"jp-minna-l01", "jp-minna-l03"} {
		if _, ok := idx.Lookup(slug); !ok {
			t.Errorf("a readable sidecar %q is missing from the index", slug)
		}
	}
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want exactly the one unreadable file", problems)
	}
	if problems[0].Source != "System/slots/L02.yaml" {
		t.Errorf("the problem names %q, want the file that could not be read", problems[0].Source)
	}
}

// TestSidecarNoteReachesThePanel covers the sentence a drill cannot express as
// a pattern. A pattern is something whose parts swap; a rule that holds across
// the whole lesson has nowhere to go among them, so a panel with no way to say
// what it leaves out presents itself as the lesson's patterns while its author
// knows it is a subset.
func TestSidecarNoteReachesThePanel(t *testing.T) {
	t.Parallel()

	withNote := append([]byte("note: >\n  もう Vました ↔ まだです 是狀態副詞,不換槽,故不入 pattern。\n"),
		slotSidecar("L07.yaml", "jp-minna-l07")...)
	idx, problems := lesson.NewSlotIndex(map[string][]byte{"System/slots/L07.yaml": withNote})
	if len(problems) != 0 {
		t.Fatalf("a sidecar carrying a note was rejected: %v", problems)
	}
	s, ok := idx.Lookup("jp-minna-l07")
	if !ok {
		t.Fatal("the sidecar did not reach the index")
	}
	if !strings.Contains(s.Note, "不入 pattern") {
		t.Errorf("Sidecar.Note = %q, want the author's own coverage statement", s.Note)
	}
}
