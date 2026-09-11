package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEmptyPathAndMapGuideFirstRun holds the two empty signals and the one
// completable step each carries on the path and map index pages and on the
// desk blocks that narrow them.
func TestEmptyPathAndMapGuideFirstRun(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		governed bool
		path     wording.Phrase
		mapState wording.Phrase
		folder   wording.Phrase
		step     wording.Phrase
	}{
		{
			name:     "folder with no contract",
			governed: false,
			path:     wording.PathIndexUngoverned,
			mapState: wording.MapIndexUngoverned,
			folder:   wording.FolderIndexUngoverned,
			step:     wording.IndexUngovernedNext,
		},
		{
			name:     "contract that declares none",
			governed: true,
			path:     wording.PathIndexEmpty,
			mapState: wording.MapIndexEmpty,
			folder:   wording.FolderIndexEmpty,
			step:     wording.IndexDeclaredEmptyNext,
		},
	}
	for _, tt := range cases {
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(tt.name+"/"+string(lang), func(t *testing.T) {
				t.Parallel()
				pathView := NewPathIndex(nil, nav.Closure{}, tt.governed, lang, nil)
				mapView := NewMapIndex(nil, nav.Closure{}, tt.governed, lang, nil)
				folderView := NewFolderIndex(&nav.Model{}, tt.governed, lang, nil)
				for _, view := range []struct {
					name  string
					state wording.Phrase
					got   string
				}{
					{name: "paths", state: tt.path, got: pathView.Shelf.Empty},
					{name: "maps", state: tt.mapState, got: mapView.Shelf.Empty},
					{name: "folders", state: tt.folder, got: folderView.Shelf.Empty},
				} {
					assertEmptyGuide(t, view.name, view.got, view.state, tt.step, lang)
				}
				blocks := NewDeskBlocks(&nav.Model{}, tt.governed, lang, nil)
				for _, block := range blocks {
					if block.Mode != pathMode && block.Mode != mapMode && block.Mode != folderMode {
						continue
					}
					state := tt.path
					switch block.Mode {
					case mapMode:
						state = tt.mapState
					case folderMode:
						state = tt.folder
					}
					assertEmptyGuide(t, "desk/"+block.Mode, block.Shelf.Empty, state, tt.step, lang)
				}
			})
		}
	}
}

func assertEmptyGuide(t *testing.T, where, got string, state, step wording.Phrase, lang wording.Lang) {
	t.Helper()
	if !strings.Contains(got, state.In(lang)) {
		t.Errorf("%s empty sentence missing %q; got %q", where, state.In(lang), got)
	}
	if !strings.Contains(got, step.In(lang)) {
		t.Errorf("%s next step missing %q; got %q", where, step.In(lang), got)
	}
	if strings.Contains(got, "宣告") {
		t.Errorf("%s still uses 宣告 jargon: %q", where, got)
	}
}

// TestEmptyPathAndMapIndexPagesRenderTheGuide keeps the empty slot on the mode
// index pages themselves, not only in the view the desk narrows from.
func TestEmptyPathAndMapIndexPagesRenderTheGuide(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		mode  string
		view  ListIndexView
		state wording.Phrase
	}{
		{
			mode:  pathMode,
			view:  NewPathIndex(nil, nav.Closure{}, false, wording.ZhHant, nil),
			state: wording.PathIndexUngoverned,
		},
		{
			mode:  mapMode,
			view:  NewMapIndex(nil, nav.Closure{}, false, wording.ZhHant, nil),
			state: wording.MapIndexUngoverned,
		},
		{
			mode:  folderMode,
			view:  NewFolderIndex(&nav.Model{}, false, wording.ZhHant, nil),
			state: wording.FolderIndexUngoverned,
		},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := ListIndex(tt.view, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if !strings.Contains(html, `data-index-empty`) {
				t.Fatalf("%s index did not render the empty slot: %q", tt.mode, html)
			}
			assertEmptyGuide(t, tt.mode, html, tt.state, wording.IndexUngovernedNext, wording.ZhHant)
		})
	}
}

// TestWithheldNavigationLeavesTheEmptySlotSilent is the third state: a contract
// that loaded but left navigation out withholds the listing and states the
// reason once, without inventing a fourth empty sentence.
func TestWithheldNavigationLeavesTheEmptySlotSilent(t *testing.T) {
	t.Parallel()

	closure := nav.Close(schema.Rejected("contract declares no navigation roles; Paths and Maps disabled until it does"))
	view := NewPathIndex(nil, closure, true, wording.ZhHant, nil)
	if view.Shelf.Empty != "" {
		t.Errorf("withheld path index spoke an empty sentence: %q", view.Shelf.Empty)
	}
	if view.Kicker != "" {
		t.Errorf("withheld path index still carried a count: %q", view.Kicker)
	}
}
