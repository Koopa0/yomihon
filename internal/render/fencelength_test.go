package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// nestedShorterFence is the CommonMark sample the close-length rule exists
// for: a four-marker opener may hold a three-marker line as literal code.
func nestedShorterFence(marker byte) string {
	outer := strings.Repeat(string(marker), 4)
	inner := strings.Repeat(string(marker), 3)
	return outer + "text\n" + inner + "\n[[Missing]]\n%% literal percent %%\n" + outer + "\n\nAfter.\n"
}

// ordinaryFence is the three-marker control: the same payload stays literal
// inside a matching fence, and the closer still ends the block.
func ordinaryFence(marker byte) string {
	fence := strings.Repeat(string(marker), 3)
	return fence + "text\n[[Missing]]\n%% literal percent %%\n" + fence + "\n\nAfter.\n"
}

// ordinaryFenceThenProse is the other half of the ordinary control: a
// matching closer still returns the scan to prose, so a wikilink and a
// comment after the block are processed rather than shown as code.
func ordinaryFenceThenProse(marker byte) string {
	fence := strings.Repeat(string(marker), 3)
	return fence + "text\ncode only\n" + fence + "\n\n[[Missing]]\n%% hidden percent %%\nAfter.\n"
}

func markerName(marker byte) string {
	if marker == '~' {
		return "tilde"
	}
	return "backtick"
}

// TestAShorterFenceInsideALongerFenceStaysLiteral locks the reproduction:
// a longer outer fence with a shorter inner fence line keeps the bracketed
// name and the percent line as code on every projection that reads the
// body. Both marker families are the same rule.
func TestAShorterFenceInsideALongerFenceStaysLiteral(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	for _, marker := range []byte{'`', '~'} {
		t.Run(markerName(marker), func(t *testing.T) {
			t.Parallel()
			body := nestedShorterFence(marker)
			excerpt, _ := render.Excerpt(body, "")
			page := r.HTML("note.md", "", body, wording.ZhHant)
			projections := map[string]string{
				"HTML":      page.HTML,
				"PlainText": render.PlainText(body),
				"Excerpt":   excerpt,
			}
			for name, got := range projections {
				if !strings.Contains(got, "[[Missing]]") {
					t.Errorf("%s = %q, want the literal wikilink kept inside the longer fence", name, got)
				}
				if !strings.Contains(got, "%% literal percent %%") {
					t.Errorf("%s = %q, want the literal percent line kept inside the longer fence", name, got)
				}
				if !strings.Contains(got, "After.") {
					t.Errorf("%s = %q, want the paragraph after the fence", name, got)
				}
			}
			if strings.Contains(page.HTML, "wikilink-broken") || strings.Contains(page.HTML, "還沒有「Missing」") {
				t.Errorf("HTML rewrote the fenced wikilink as a missing note:\n%s", page.HTML)
			}
			for _, d := range page.Diagnostics {
				if strings.Contains(d.Target, "Missing") {
					t.Errorf("a fenced target was reported as a broken link: %+v", d)
				}
			}
		})
	}
}

// TestAnOrdinaryFenceStillGuardsAndCloses is the matching-length control:
// content inside a three-marker fence stays literal, and a matching closer
// still returns later lines to prose.
func TestAnOrdinaryFenceStillGuardsAndCloses(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	for _, marker := range []byte{'`', '~'} {
		t.Run(markerName(marker)+" inside", func(t *testing.T) {
			t.Parallel()
			body := ordinaryFence(marker)
			excerpt, _ := render.Excerpt(body, "")
			page := r.HTML("note.md", "", body, wording.ZhHant)
			projections := map[string]string{
				"HTML":      page.HTML,
				"PlainText": render.PlainText(body),
				"Excerpt":   excerpt,
			}
			for name, got := range projections {
				if !strings.Contains(got, "[[Missing]]") {
					t.Errorf("%s = %q, want the literal wikilink kept inside an ordinary fence", name, got)
				}
				if !strings.Contains(got, "%% literal percent %%") {
					t.Errorf("%s = %q, want the literal percent line kept inside an ordinary fence", name, got)
				}
				if !strings.Contains(got, "After.") {
					t.Errorf("%s = %q, want the paragraph after the ordinary fence", name, got)
				}
			}
			if strings.Contains(page.HTML, "wikilink-broken") || strings.Contains(page.HTML, "還沒有「Missing」") {
				t.Errorf("HTML rewrote a wikilink that was inside an ordinary fence:\n%s", page.HTML)
			}
		})

		t.Run(markerName(marker)+" after", func(t *testing.T) {
			t.Parallel()
			body := ordinaryFenceThenProse(marker)
			excerpt, _ := render.Excerpt(body, "")
			page := r.HTML("note.md", "", body, wording.ZhHant)
			plain := render.PlainText(body)

			if !strings.Contains(page.HTML, "還沒有「Missing」") && !strings.Contains(page.HTML, "wikilink-broken") {
				t.Errorf("HTML = %q, want the wikilink after an ordinary fence still resolved", page.HTML)
			}
			if strings.Contains(page.HTML, "[[Missing]]") {
				t.Errorf("HTML kept a prose wikilink literal after an ordinary fence:\n%s", page.HTML)
			}
			if strings.Contains(page.HTML, "hidden percent") || strings.Contains(plain, "hidden percent") || strings.Contains(excerpt, "hidden percent") {
				t.Errorf("a comment after an ordinary fence reached a projection; HTML=%q PlainText=%q Excerpt=%q", page.HTML, plain, excerpt)
			}
			if strings.Contains(plain, "[[Missing]]") {
				t.Errorf("PlainText = %q, want the prose wikilink after an ordinary fence rewritten", plain)
			}
			if !strings.Contains(plain, "Missing") {
				t.Errorf("PlainText = %q, want the prose wikilink's name after an ordinary fence", plain)
			}
			if !strings.Contains(page.HTML, "After.") || !strings.Contains(plain, "After.") || !strings.Contains(excerpt, "After.") {
				t.Errorf("lost the paragraph after an ordinary fence; HTML=%q PlainText=%q Excerpt=%q", page.HTML, plain, excerpt)
			}
		})
	}
}
