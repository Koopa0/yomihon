package layouts

import (
	"os"
	"strings"
	"testing"
)

// TestTheHoverCardNeverWaitsForADiagramItWillNotGet holds the one state a card
// can enter and never leave. The diagram renderer is bound once, over the
// document the page booted on; a card's prose arrives long afterwards and is
// never reached, so the waiting state the stylesheet paints over an undrawn
// diagram — transparent text under an endless animation — would run for as
// long as the card is open, over source text the reader can no longer see. A
// reader who asked for less motion gets the same box with the animation frozen,
// which is a blank grey rectangle.
//
// The release has to outrank the wait rather than merely follow it, so what is
// checked is that a rule naming the card releases all three properties the wait
// sets.
func TestTheHoverCardNeverWaitsForADiagramItWillNotGet(t *testing.T) {
	t.Parallel()
	css := previewStylesheet(t)

	const waiting = "[data-js]:not([data-mermaid-error]) .y-prose .mermaid-diagram:not([data-mermaid-error]):not(:has(svg)) {"
	if !strings.Contains(css, waiting) {
		t.Fatalf("the stylesheet no longer carries the waiting state this release answers to, so its absence would prove nothing:\n%s", waiting)
	}
	const released = "[data-js]:not([data-mermaid-error]) .y-preview .y-prose .mermaid-diagram:not([data-mermaid-error]):not(:has(svg)) {"
	if !strings.Contains(css, released) {
		t.Fatalf("nothing releases a diagram inside a card from the waiting state, so a fence hovered into a card is transparent text under an animation that never ends")
	}
	declarations := cssDeclarations(t, ruleBody(t, css, released))
	for _, want := range []struct {
		property string
		why      string
	}{
		{"color", "the waiting state paints the source transparent, and a card that never draws a diagram has only the source to show"},
		{"background-image", "the waiting state's moving gradient is the animation made visible"},
		{"background-color", "the gradient sits on the waiting state's own ground"},
		{"animation-name", "an animation with nothing coming is a page that never stops saying it is loading"},
	} {
		if declarations[want.property] == "" {
			t.Errorf("a diagram inside a card keeps the waiting state's %s, so %s", want.property, want.why)
		}
	}
	// The release must sit after what it releases, since the two carry the same
	// weight but for the one extra class.
	if strings.Index(css, released) < strings.Index(css, waiting) {
		t.Error("the release is written before the wait it answers, so the cascade keeps the wait")
	}

	// And it has to survive the build, which is not the same question. Written
	// with the declarations the rule below it already had, this rule was folded
	// into that one and lost the two guards that give it its weight — leaving
	// the authored stylesheet correct, this check green, and every card in the
	// browser still waiting. What the reader receives is the built file, so the
	// built file is asked too.
	const built = "../../../assets/css/output.css"
	stylesheet, err := os.ReadFile(built)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", built, err)
	}
	guarded := strings.TrimSuffix(released, " {")
	if !strings.Contains(string(stylesheet), guarded) {
		t.Errorf("the built stylesheet carries no rule reading %s, so whatever the authored one says the browser keeps the waiting state", guarded)
	}
}

// TestTheHoverCardIsShortEnoughToReadWithoutScrollingIt holds the height a card
// opened from the keyboard can be read at. Arrow keys and PageDown scroll the
// page, which dismisses the card, and Tab dismisses it too — so whatever does
// not fit is not reachable by that reader at all. A shorter card also sits
// beside more of the links on a page instead of being flipped away from them.
func TestTheHoverCardIsShortEnoughToReadWithoutScrollingIt(t *testing.T) {
	t.Parallel()
	declarations := cssDeclarations(t, ruleBody(t, previewStylesheet(t), ".y-preview {"))
	const want = "min(18rem, 60vh)"
	if got := declarations["max-block-size"]; got != want {
		t.Errorf("the card is capped at %q, want %q; what does not fit is unreachable to a reader who opened it from the keyboard", got, want)
	}
	if declarations["overflow-y"] != "auto" {
		t.Errorf("the card does not scroll inside itself, so an excerpt past the cap is simply cut off")
	}
}

// TestTheLinkAnOpenCardBelongsToIsStillMarked holds the tie between a card and
// the link it came from. Hovering marks the link, but the pointer has to leave
// it to reach the card — and on a paragraph carrying several wikilinks the tie
// would break at exactly the moment the reader starts reading.
func TestTheLinkAnOpenCardBelongsToIsStillMarked(t *testing.T) {
	t.Parallel()
	css := previewStylesheet(t)
	open := cssDeclarations(t, ruleBody(t, css, ".y-prose a.wikilink[data-preview-open] {"))
	hover := cssDeclarations(t, ruleBody(t, css, ".y-prose a.wikilink:hover {"))
	if hover["border-bottom-color"] == "" {
		t.Fatal("a hovered wikilink no longer marks itself, so there is nothing for the open card's link to keep")
	}
	if open["border-bottom-color"] != hover["border-bottom-color"] {
		t.Errorf("a link with a card open is marked %q while a hovered one is marked %q; the tie between the card and its own link has to outlast the pointer leaving that link",
			open["border-bottom-color"], hover["border-bottom-color"])
	}
	if open["anchor-name"] == "" {
		t.Error("the link with a card open carries no anchor name, so the stylesheet has nothing to place the card against")
	}
}

// previewStylesheet reads the authored stylesheet with its comments removed.
// The authored file rather than the built one: what is under review is what a
// person wrote, and that the build still agrees is the gate's own question.
func previewStylesheet(t *testing.T) string {
	t.Helper()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return cssComments.ReplaceAllString(string(source), "")
}
