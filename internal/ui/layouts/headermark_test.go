package layouts

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// offer is the one reading page these tests speak for: a note whose bytes are
// identified, so a place kept in it points at something.
func offer() *MarkOffer {
	return &MarkOffer{Path: "Notes/alpha.md", Identity: "abc123", Endpoint: "/marks"}
}

// TestTheHeaderMarkIsInsideTheFoldedPanelAndLast holds the seam the wide row
// opens. The panel's children become the header row's own items above the
// width the row folds at, so an item that has no room there has to be inside
// the panel and nowhere else in the header. And it has to come after the six
// that do move, because the reader who tabs into the panel walks it in the
// order it is written.
func TestTheHeaderMarkIsInsideTheFoldedPanelAndLast(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := header(Chrome{Mark: offer()}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := buf.String()

	if strings.Count(html, "y-headermark") == 0 {
		t.Fatalf("the header draws no control for keeping a reading place; html = %q", html)
	}
	panel := elementSubtree(t, html, `id="header-fold"`)
	if !strings.Contains(panel, `class="y-headermark"`) {
		t.Errorf("the control is somewhere in the header other than the folded panel; panel = %q", panel)
	}
	// Outside the panel there is none of it: the row carries the control at no
	// width, which is what the stylesheet's rule is for and what a stray copy
	// in the row would quietly undo.
	if rest := strings.Replace(html, panel, "", 1); strings.Contains(rest, "y-headermark") {
		t.Errorf("the header row carries a second copy of the control; rest = %q", rest)
	}
	theme := strings.Index(panel, "data-theme-toggle")
	mark := strings.Index(panel, "y-headermark")
	if theme < 0 {
		t.Fatalf("the panel carries no theme control to place the mark after; panel = %q", panel)
	}
	if mark < theme {
		t.Errorf("the control is written before the six that move between the row and the panel, so tabbing into the panel meets it first")
	}
}

// TestTheHeaderMarkCarriesTheOfferItWasHanded asks what the press will send.
// The three values are the whole of the post, and a control that renders
// without one of them is a button that stores a place pointing at nothing.
func TestTheHeaderMarkCarriesTheOfferItWasHanded(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := header(Chrome{Mark: offer()}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-mark-control`,
		`data-mark-path="Notes/alpha.md"`,
		`data-mark-identity="abc123"`,
		`data-mark-endpoint="/marks"`,
		`data-mark-button`,
		`data-mark-said`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the header control is missing %s; html = %q", want, html)
		}
	}
}

// TestTheHeaderMarkSpeaksTheInterfaceLanguage checks the words, both of them.
// The sentences the client says back travel on the element, so a language
// missing here is a control that answers in the other one.
func TestTheHeaderMarkSpeaksTheInterfaceLanguage(t *testing.T) {
	t.Parallel()
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		var buf bytes.Buffer
		if err := header(Chrome{Lang: lang, Mark: offer()}).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render header in %v: %v", lang, err)
		}
		html := buf.String()
		for _, want := range []string{
			wording.MarkSetControl.In(lang),
			wording.MarkSaved.In(lang),
			wording.MarkNotStored.In(lang),
		} {
			if !strings.Contains(html, want) {
				t.Errorf("the header control in %v does not carry %q; html = %q", lang, want, html)
			}
		}
	}
}

// TestAHeaderWithNoOfferDrawsNoMarkControl is the other side, so the rule above
// cannot be satisfied by drawing the control everywhere. Every page carries
// this header and only a reading page can offer a place to keep, so a desk or a
// search page holding a button that posts an empty path is the failure this
// forbids.
func TestAHeaderWithNoOfferDrawsNoMarkControl(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := header(Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render header: %v", err)
	}
	html := buf.String()
	for _, gone := range []string{"y-headermark", "data-mark-control", "data-mark-button"} {
		if strings.Contains(html, gone) {
			t.Errorf("a header handed no offer still carries %s; html = %q", gone, html)
		}
	}
}
