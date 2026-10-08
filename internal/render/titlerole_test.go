package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// Hidden comments can keep invisible block-role bytes. They do not make a
// visible preface to the page title, but surviving prose still does.
func TestTitleHeadingSkipsInvisibleCommentRoles(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)
	for _, prefix := range []string{"%% hidden %%\n", "<!-- hidden -->\n", "\n"} {
		body := prefix + "# Title\n\nOpening words here.\n"
		page := r.HTML("Notes/title.md", "Title", body, wording.ZhHant)
		if page.TitleAnchor != "title" || len(page.TOC) != 0 || strings.Contains(page.HTML, "Title") || !strings.Contains(page.HTML, "Opening words here.") || !render.DropsTitleHeading("Title", body) {
			t.Errorf("caught: S5 title-comment-page prefix=%q anchor=%q toc=%+v HTML=%q", prefix, page.TitleAnchor, page.TOC, page.HTML)
		}
	}
	body := "%% hidden %%Preface.\n\n# Title\n"
	page := r.HTML("Notes/title.md", "Title", body, wording.ZhHant)
	if page.TitleAnchor != "" || !strings.Contains(page.HTML, "Title") || render.DropsTitleHeading("Title", body) {
		t.Errorf("caught: S5 title-comment-page surviving preface anchor=%q HTML=%q", page.TitleAnchor, page.HTML)
	}
	t.Log("AGREEMENT-INVOKED S5/stage5-title-comment-page")
}
