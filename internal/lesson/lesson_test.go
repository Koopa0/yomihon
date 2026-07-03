package lesson_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/kurodo/internal/lesson"
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

	// An empty color is a neutral (uncoloured) slot, not a fault: kurodo keeps
	// yomihon's validation contract here (D04 — correctness may not be
	// reinvented), where only a non-empty unknown token is rejected. This pins
	// that behaviour so a future tightening is a conscious decision, not a drift.
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

// writeSidecar writes a minimal valid sidecar YAML to dir/name and returns
// nothing — a test helper for the index tests.
func writeSidecar(t *testing.T, dir, name, slug string) {
	t.Helper()
	body := "lesson: " + strings.TrimSuffix(name, ".yaml") + "\nslug: " + slug + "\ntitle: t\npatterns: []\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write sidecar %s: %v", name, err)
	}
}

func TestBuildSlotIndexJoinsBySlug(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Filename and slug deliberately disagree (D29): the index must key on the
	// in-file slug, not the L01/L02 filename.
	writeSidecar(t, dir, "L01.yaml", "jp-minna-l01")
	writeSidecar(t, dir, "L02.yaml", "jp-minna-l02")
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("ignored"), 0o600); err != nil {
		t.Fatalf("write decoy: %v", err)
	}

	idx, err := lesson.BuildSlotIndex(dir)
	if err != nil {
		t.Fatalf("BuildSlotIndex(%q) = %v", dir, err)
	}
	if len(idx) != 2 {
		t.Errorf("BuildSlotIndex indexed %d sidecars, want 2 (non-yaml must be skipped)", len(idx))
	}
	got, ok := idx.Lookup("jp-minna-l01")
	if !ok {
		t.Fatalf("Lookup(jp-minna-l01) not found; index keyed by filename instead of slug?")
	}
	if got.Lesson != "L01" {
		t.Errorf("Lookup(jp-minna-l01).Lesson = %q, want %q", got.Lesson, "L01")
	}
	if _, ok := idx.Lookup("jp-minna-l99"); ok {
		t.Errorf("Lookup(jp-minna-l99) reported found for a slug with no sidecar")
	}
}

func TestBuildSlotIndexRejectsDuplicateSlug(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSidecar(t, dir, "L01.yaml", "jp-minna-l01")
	writeSidecar(t, dir, "L01-copy.yaml", "jp-minna-l01") // same slug, second file
	if _, err := lesson.BuildSlotIndex(dir); err == nil {
		t.Fatal("BuildSlotIndex accepted two sidecars with the same slug; want an ambiguity error")
	}
}

func TestBuildSlotIndexRejectsMissingSlug(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "L01.yaml"), []byte("lesson: L01\ntitle: t\npatterns: []\n"), 0o600); err != nil {
		t.Fatalf("write slugless sidecar: %v", err)
	}
	if _, err := lesson.BuildSlotIndex(dir); err == nil {
		t.Fatal("BuildSlotIndex accepted a sidecar with no slug; want an error (the join key would be empty)")
	}
}
