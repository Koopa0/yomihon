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

// TestTheHeaderMarkIsOneBookmarkThatSaysWhetherItIsSet holds the row's face of
// the control: one button, named by the verb rather than drawn as a bare icon,
// whose pressed state is whether the place already kept is in this note. A
// press on a pressed bookmark keeps the place again, so the state is the
// server's answer about the note and nothing the button toggles away.
func TestTheHeaderMarkIsOneBookmarkThatSaysWhetherItIsSet(t *testing.T) {
	t.Parallel()
	for _, kept := range []bool{false, true} {
		mark := offer()
		mark.Kept = kept
		var buf bytes.Buffer
		if err := header(Chrome{Mark: mark}).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render header: %v", err)
		}
		html := buf.String()
		if got := strings.Count(html, "data-mark-button"); got != 1 {
			t.Fatalf("the header draws %d reading-place buttons, want 1; html = %q", got, html)
		}
		_, button, _ := strings.Cut(html, "data-mark-button")
		button, _, _ = strings.Cut(button, ">")
		want := `aria-pressed="false"`
		if kept {
			want = `aria-pressed="true"`
		}
		if !strings.Contains(button, want) {
			t.Errorf("kept=%v: the bookmark does not carry %s; button = %q", kept, want, button)
		}
		if !strings.Contains(button, `aria-label="`+wording.MarkSetControl.In(wording.ZhHant)+`"`) {
			t.Errorf("kept=%v: the bookmark is not named by its verb; button = %q", kept, button)
		}
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
