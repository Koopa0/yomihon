package render_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestByteOrderMarkBeforeTheFirstHeadingRendersAsAHeading(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"", "---\ntitle: Metadata title\n---\n", "---\r\ntitle: Metadata title\r\n---\r\n"} {
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()
			note := vault.Parse("note.md", []byte("\xef\xbb\xbf"+prefix+"# BOM heading\n\nbody\n"))
			got := newRenderer(t, nil, nil, nil).HTML(note.RelPath, "", note.Body, wording.En)
			if !strings.Contains(got.HTML, `<h2 id="bom-heading" data-level="1">BOM heading</h2>`) || strings.Contains(got.HTML, "<p># BOM heading</p>") {
				t.Errorf("HTML(Parse(BOM heading)) lost the first heading:\n%s", got.HTML)
			}
			want := []render.TOCEntry{{Level: 1, Text: "BOM heading", ID: "bom-heading"}}
			if diff := cmp.Diff(want, got.TOC); diff != "" {
				t.Errorf("HTML(Parse(BOM heading)).TOC mismatch (-want +got):\n%s", diff)
			}
			withTitle := newRenderer(t, nil, nil, nil).HTML(note.RelPath, "BOM heading", note.Body, wording.En)
			if withTitle.TitleAnchor != "bom-heading" || strings.Contains(withTitle.HTML, "BOM heading") {
				t.Errorf("HTML(Parse(BOM heading), matching title) anchor/body = %q/%q, want title anchor without a repeated heading", withTitle.TitleAnchor, withTitle.HTML)
			}
			if prefix != "" && (note.Title() != "Metadata title" || note.FMDiagnostic != "") {
				t.Errorf("Parse(BOM frontmatter) title/diagnostic = %q/%q, want Metadata title and no diagnostic", note.Title(), note.FMDiagnostic)
			}
		})
	}
}
