package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/lesson"
)

// Each card is an article, which a screen reader offers as a region to jump
// into, and one with no name announces itself as nothing worth entering. The
// name is the card's own heading, and the heading's id is the card's position
// in the lesson. Both patterns below leave their id empty, which is what a
// sidecar is allowed to do — ids taken from that field would be blank and would
// collide, and every card would answer to the same name.
func TestSlotMachineNamesEachCardByItsOwnHeading(t *testing.T) {
	t.Parallel()
	slots := map[string]lesson.Position{
		"A": {LabelZH: "主題", Color: "topic", Fills: []lesson.Fill{{JP: "私", Reading: "わたし", ZH: "我"}}},
	}
	view := &lesson.Sidecar{Patterns: []lesson.Pattern{
		{Template: "{A}です", GlossZH: "{A}。", Slots: slots},
		{Template: "{A}ですか", GlossZH: "{A}嗎。", Slots: slots},
	}}

	var buf bytes.Buffer
	if err := SlotMachine(view, "nonce").Render(t.Context(), &buf); err != nil {
		t.Fatalf("render slot machine: %v", err)
	}
	html := buf.String()

	for _, want := range []string{
		// The class stays the first attribute: the lesson page asserts this
		// opening as a substring, and reordering would fail it for a reason
		// that has nothing to do with what it is watching.
		`<article class="y-slotcard" aria-labelledby="slot-pattern-1">`,
		`<article class="y-slotcard" aria-labelledby="slot-pattern-2">`,
		`<h3 class="y-slotcard__abstract" id="slot-pattern-1" lang="ja">`,
		`<h3 class="y-slotcard__abstract" id="slot-pattern-2" lang="ja">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("SlotMachine() = %q, want it to contain %q", html, want)
		}
	}
	// Each id occurs exactly twice — once naming the heading, once pointing at
	// it. A third occurrence means two cards share a name, which is the failure
	// an authored id would produce.
	for _, id := range []string{`"slot-pattern-1"`, `"slot-pattern-2"`} {
		if got := strings.Count(html, id); got != 2 {
			t.Errorf("SlotMachine() mentions %s %d times, want exactly 2 (the heading and the reference to it)", id, got)
		}
	}
}

// The region lesson.js speaks the shuffled sentence through has to be on the
// page before the shuffle, and empty: a region created at the moment of the
// announcement is not reliably read, and one that arrives with text in it is
// read on arrival, when nothing has happened yet.
func TestSlotMachineShipsAnEmptyJapaneseLiveRegionPerCard(t *testing.T) {
	t.Parallel()
	slots := map[string]lesson.Position{
		"A": {LabelZH: "主題", Color: "topic", Fills: []lesson.Fill{{JP: "私", Reading: "わたし", ZH: "我"}}},
	}
	view := &lesson.Sidecar{Patterns: []lesson.Pattern{
		{Template: "{A}です", GlossZH: "{A}。", Slots: slots},
		{Template: "{A}ですか", GlossZH: "{A}嗎。", Slots: slots},
	}}

	var buf bytes.Buffer
	if err := SlotMachine(view, "nonce").Render(t.Context(), &buf); err != nil {
		t.Fatalf("render slot machine: %v", err)
	}
	// Written out closed and empty, and counted: one per card, so a card added
	// without one cannot pass. The declared language is Japanese because the
	// only enclosing declaration is the machine's Traditional Chinese chrome.
	const region = `<p class="y-slotlive y-offscreen" role="status" aria-live="polite" aria-atomic="true" lang="ja"></p>`
	if got := strings.Count(buf.String(), region); got != len(view.Patterns) {
		t.Errorf("SlotMachine() = %q, want %d copies of %q, got %d", buf.String(), len(view.Patterns), region, got)
	}
}

func TestSlotMachineRendersPatternData(t *testing.T) {
	t.Parallel()
	view := &lesson.Sidecar{Patterns: []lesson.Pattern{{
		ID:       "plain",
		Template: "{A}です",
		GlossZH:  "{A}。",
		Slots: map[string]lesson.Position{
			"A": {
				Color: "topic",
				Fills: []lesson.Fill{{JP: "私", Reading: "わたし", ZH: "我"}},
			},
		},
	}}}
	data := `{"template":"{A}です","gloss":"{A}。","keys":["A"],"slots":{"A":{"color":"topic","fills":[{"jp":"私","reading":"わたし","zh":"我"}]}}}`
	tests := []struct {
		name      string
		nonce     string
		wantNonce string
	}{
		{name: "response nonce", nonce: "response-nonce", wantNonce: "response-nonce"},
		{name: "attribute delimiter", nonce: `bad" data-injected="true`, wantNonce: `bad&#34; data-injected=&#34;true`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := SlotMachine(view, tt.nonce).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render slot machine: %v", err)
			}
			want := `<script nonce="` + tt.wantNonce + `" type="application/json" class="y-slotdata">` + data + `</script>`
			if html := buf.String(); !strings.Contains(html, want) {
				t.Errorf("SlotMachine() = %q, want slot data %q", html, want)
			}
		})
	}
}
