package layouts

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The mat is the ground an author's drawing sits on: an SVG alone in its
// paragraph, or a figure's frame. These tests read the authored stylesheets and
// hold what the drawing's reader depends on: one inset for both, a scroll
// instead of a shrink, a ground that follows the same light or dark the
// drawing does, and nothing of the mat on paper or in a hover card. The
// expected values are written out here, not read from the rules under test.

const (
	matSelector     = ".y-prose .mat"
	themedSelector  = ".y-prose .mat:has(> .figure-embed__frame[data-figure-scripts])"
	frameSelector   = ".y-prose .figure-embed__frame"
	pictureSelector = ".y-prose img"
)

// matOrFigure matches a selector that is about a mat or a figure's frame.
var matOrFigure = regexp.MustCompile(`\.mat\b|figure-embed`)

// declared returns the rules that carry exactly this selector, outside every
// at-rule when media is empty and inside the one at-rule named otherwise.
func declared(rules []cssRule, selector, media string) []cssRule {
	var out []cssRule
	for _, rule := range rules {
		if rule.selector != selector {
			continue
		}
		if (media == "" && len(rule.within) == 0) || (media != "" && slices.Equal(rule.within, []string{media})) {
			out = append(out, rule)
		}
	}
	return out
}

// theOne returns the single rule written for a selector, so a second copy of
// it, which would put two answers to one question in the sheet, fails here.
func theOne(t *testing.T, rules []cssRule, selector, media string) cssRule {
	t.Helper()
	found := declared(rules, selector, media)
	if len(found) != 1 {
		t.Fatalf("components.css writes %d rules for %q (inside %q), want exactly 1", len(found), selector, media)
	}
	return found[0]
}

func wantValue(t *testing.T, rule cssRule, property, want, why string) {
	t.Helper()
	got := rule.values(property)
	if len(got) != 1 || got[0] != want {
		t.Errorf("%s declares %s: %q, want exactly %q, because %s", rule.selector, property, got, want, why)
	}
}

// TestAMatIsAPlateWithItsInsetWrittenOnce holds the one inset every mat
// shares: a picture's, a diagram's and a figure's frame. Each is something on
// the same ground, so a second inset anywhere is two plates that disagree by a
// few pixels. The inset is 26px above, 28px either side and as much below as
// above, 18px below a caption, and 18px and 16px on a phone; the renderer counts
// a picture's room at each measure without the 28px either side (plateInset),
// so the two are held to one number here. The plate is set into the page by
// its tone, a faint ring and the small radius, with no line taking room. The
// frame inside it takes none of its own: a border or padding on the frame would
// be the frame line the mat does without.
func TestAMatIsAPlateWithItsInsetWrittenOnce(t *testing.T) {
	t.Parallel()
	rules := componentRules(t)

	mat := theOne(t, rules, matSelector, "")
	wantValue(t, mat, "padding", "var(--mat-top) var(--mat-side) var(--mat-foot)", "every mat's inset is written here and nowhere else")
	wantValue(t, mat, "--mat-top", "26px", "the plate's inset above what it holds")
	wantValue(t, mat, "--mat-side", "28px", "the plate's inset either side, which plateInset counts a picture's room without")
	wantValue(t, mat, "--mat-foot", "var(--mat-top)", "with no caption the plate is as deep below as above")
	wantValue(t, mat, "background", "var(--mat)", "a mat is a tone, and the tone is the token")
	wantValue(t, mat, "border-radius", "var(--r-sm)", "the plate's corners are the small radius")
	wantValue(t, mat, "box-shadow", "inset 0 0 0 1px var(--line-faint)", "the plate's edge is a faint ring drawn inside it, which takes no room")
	wantValue(t, theOne(t, rules, matSelector+":has(> figcaption)", ""), "--mat-foot", "18px", "a caption stands closer to the plate's foot")
	narrow := theOne(t, rules, matSelector, "@media (max-width: 720px)")
	wantValue(t, narrow, "--mat-top", "18px", "a phone gives the plate less room")
	wantValue(t, narrow, "--mat-side", "16px", "a phone gives the plate less room")

	plate, err := os.ReadFile("../../../internal/render/plate.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plate), "const plateInset = 2 * 28") {
		t.Error("render/plate.go no longer counts a picture's room without 28px either side, the plate's --mat-side")
	}

	for _, rule := range rules {
		if !matOrFigure.MatchString(rule.selector) {
			continue
		}
		if len(rule.within) != 0 {
			continue
		}
		// The caption's padding is the space between its own rule and its
		// words, inside the mat's inset rather than beside it.
		if rule.selector == ".y-prose figcaption" {
			continue
		}
		for _, property := range []string{"padding", "padding-inline", "padding-block", "padding-left", "padding-right", "padding-top", "padding-bottom"} {
			if rule.selector != matSelector && len(rule.values(property)) != 0 {
				t.Errorf("%s declares %s, a second inset beside the mat's own", rule.selector, property)
			}
		}
	}

	frame := theOne(t, rules, frameSelector, "")
	wantValue(t, frame, "border", "0", "the mat is the figure's only edge, and it is tone")
	wantValue(t, frame, "background", "transparent", "the mat's tone shows through a figure that paints no ground of its own")

	source, err := os.ReadFile("../../../assets/css/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withoutComments(string(source)), "--mat: light-dark(#ece8df, #1b1b18);") {
		t.Error("tokens.css does not declare --mat as the pair #ece8df and #1b1b18, the mat's two tones in the design")
	}
}

// TestAMatIsDrawnAsAFrameWhenForcedColoursFlattenItsTone holds the one place
// a mat has a line. Its tone is a background, which forced colours replaces, so
// with the tone gone nothing would mark where the drawing's ground begins; the
// edge is then drawn as a frame in the system's text colour, and only then.
func TestAMatIsDrawnAsAFrameWhenForcedColoursFlattenItsTone(t *testing.T) {
	t.Parallel()
	rules := componentRules(t)

	if got := theOne(t, rules, matSelector, "").values("border"); len(got) != 0 {
		t.Errorf("%s declares border: %q outside forced colours, want none, because the mat's edge is its tone", matSelector, got)
	}
	forced := theOne(t, rules, matSelector, "@media (forced-colors: active)")
	wantValue(t, forced, "border", "1px solid CanvasText", "forced colours takes the tone away, and the system's text colour is the one edge it leaves")
}

// TestAMatHoldsAWideDrawingAtItsFloorAndScrolls holds the reason the mat
// scrolls. A drawing wider than the screen was shrunk to fit, which took its
// words below the size anyone can read; the mat now holds it at a floor and
// scrolls sideways. The floor is the width of the figure's frame or of the
// picture, so what is held is the drawing and not the page.
func TestAMatHoldsAWideDrawingAtItsFloorAndScrolls(t *testing.T) {
	t.Parallel()
	rules := componentRules(t)

	mat := theOne(t, rules, matSelector, "")
	wantValue(t, mat, "overflow-x", "auto", "a drawing wider than the screen scrolls inside its mat, not the page")
	wantValue(t, mat, "--mat-floor", "560px", "below this a drawing's smallest words are under 9px")

	frame := theOne(t, rules, frameSelector, "")
	wantValue(t, frame, "max-width", "max(100%, var(--mat-floor))", "a frame narrower than its floor is held at it, and a frame the embed made narrower is left alone")
	unsized := theOne(t, rules, frameSelector+":not([width])", "")
	wantValue(t, unsized, "width", "max(100%, var(--mat-floor))", "a frame with no width of its own fills the column, and is held at the floor")

	picture := theOne(t, rules, pictureSelector, "")
	wantValue(t, picture, "max-width", "max(100%, var(--mat-floor, 0%))", "a picture on a mat (an SVG) is held at the floor; a picture in a sentence has no floor, so it is never wider than its column")

	if got := theOne(t, rules, ".y-prose .mat > img", "").values("display"); !slices.Equal(got, []string{"block"}) {
		t.Errorf(".y-prose .mat > img declares display: %q, want block, or the line box leaves a gap under the drawing inside its mat", got)
	}
}

// TestAMatFollowsTheSystemUnlessTheFigureIsToldTheTheme holds which light or
// dark a mat takes, which is the light or dark of what it holds. A picture's
// own stylesheet and a figure that runs no script ask the browser for
// prefers-color-scheme, which answers with the system, so their mat takes
// the system's: both halves agree, and a dark page does not put a light
// drawing on a dark mat. A figure the owner approved is told the page's theme,
// so its mat takes the page's. The selector for that is the one the script
// uses to decide which frames it tells.
func TestAMatFollowsTheSystemUnlessTheFigureIsToldTheTheme(t *testing.T) {
	t.Parallel()
	rules := componentRules(t)

	wantValue(t, theOne(t, rules, matSelector, ""), "color-scheme", "light dark",
		"two schemes named means the system's, whatever the page's own scheme is")
	themed := theOne(t, rules, themedSelector, "")
	wantValue(t, themed, "color-scheme", "inherit", "an approved figure is told the page's theme, so its mat is in the page's scheme")
	wantValue(t, themed, "background", "var(--panel)", "a mat in the page's own scheme is a step off the page, the same step a panel is")
	page := theOne(t, rules, ".y-prose .mat--page", "")
	wantValue(t, page, "color-scheme", "inherit", "a raster picture is the same in either light, and a diagram is drawn again in the page's theme, so their mat is in the page's scheme")
	wantValue(t, page, "--mat-floor", "0%", "a raster picture shrinks to the column as it always has")

	script, err := os.ReadFile("../../../assets/js/diagrams.js")
	if err != nil {
		t.Fatal(err)
	}
	const told = "iframe.figure-embed__frame[data-figure-scripts]"
	if !strings.Contains(string(script), told) {
		t.Fatalf("diagrams.js no longer selects %q, the frames it tells the theme; the mat's selector below follows it and this check no longer says which", told)
	}
	if !strings.Contains(themedSelector, strings.TrimPrefix(told, "iframe")) {
		t.Errorf("the themed mat selector %q does not name the frames the script tells the theme (%q)", themedSelector, told)
	}
}

// TestAMatIsPaperOnPaperAndShrinksInACard holds the two places a mat has
// nowhere to scroll. Paper has no sideways, so a picture shrinks to the sheet,
// and the mat's tone and its dark scheme go, because paper is white. A figure is
// not on paper at all: the printed stand-in is. A hover card is a glance at
// another note and not a place to scroll, so what it shows is fitted to it.
func TestAMatIsPaperOnPaperAndShrinksInACard(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile(componentsPath)
	if err != nil {
		t.Fatal(err)
	}
	css := withoutComments(string(source))
	// The print block's rules are keyed by each selector separately, as the
	// block writes several under one declaration.
	onPaper := rulesInside(t, atRuleBody(t, css, "@media print {"))
	for selector, wants := range map[string]map[string]string{
		matSelector: {
			"background":   "none",
			"color-scheme": "light",
			"overflow-x":   "visible",
			"--mat-floor":  "0%",
		},
		".y-prose .figure-embed .mat": {"display": "none"},
	} {
		got, written := onPaper[selector]
		if !written {
			t.Errorf("the print block says nothing about %s", selector)
			continue
		}
		for property, want := range wants {
			if got[property] != want {
				t.Errorf("%s declares %s: %q for print, want %q", selector, property, got[property], want)
			}
		}
	}

	rules := componentRules(t)
	card := theOne(t, rules, ".y-preview .y-prose .mat", "")
	wantValue(t, card, "--mat-floor", "0%", "a hover card fits what it shows to the card")
}
