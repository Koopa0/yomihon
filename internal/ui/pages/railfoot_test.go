package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestEveryLeftRailEndsWithTheFoot walks the templates rather than the three
// rails anyone happens to remember. Three of them draw the same aside today —
// the whole-vault tree, one book, a course's parts — and at narrow widths the
// stylesheet turns whichever one a page carries into the drawer, so a rail
// without the foot is a drawer without it too. A fourth rail would arrive with
// its own recording and nothing else would notice what it was missing.
func TestEveryLeftRailEndsWithTheFoot(t *testing.T) {
	t.Parallel()

	sources, err := filepath.Glob("*.templ")
	if err != nil {
		t.Fatalf("Glob(*.templ) error = %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no templates sit beside this test, so walking them proves nothing")
	}
	rails := 0
	for _, source := range sources {
		data, readErr := os.ReadFile(source) // #nosec G304 -- a template name from this package's own directory listing
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", source, readErr)
		}
		rest := string(data)
		for {
			opened := strings.Index(rest, `class="y-rail-left"`)
			if opened < 0 {
				break
			}
			rest = rest[opened+len(`class="y-rail-left"`):]
			closed := strings.Index(rest, "</aside>")
			if closed < 0 {
				t.Fatalf("%s opens a left rail that never closes, so this test cannot read where it ends", source)
			}
			rails++
			if !strings.Contains(rest[:closed], "@railFoot(") {
				t.Errorf("the left rail in %s does not end with the foot, so that page — and its drawer — stops where its tree stops", source)
			}
			rest = rest[closed:]
		}
	}
	// The three that draw one today. A rail removed without this number moving
	// would leave the walk above passing over whatever is left.
	if rails != 3 {
		t.Errorf("the templates draw %d left rails, and this test was written against 3", rails)
	}
}

// TestTheFootAnswersInSentences holds the state a folder can be in that has
// no name to show: a folder whose path yields none is still named, because a
// blank at the end of the rail reads as something broken.
func TestTheFootAnswersInSentences(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		if got := libraryName("", lang); got == "" {
			t.Errorf("libraryName(%q) is blank for a folder with no name to show", lang)
		}
		if got := libraryName("example-vault", lang); got != "example-vault" {
			t.Errorf("libraryName() = %q, want the folder's own name", got)
		}
	}
}

// TestTheFootLeavesTheHealthViewToTheDesk holds the rail to reading. The
// whole-folder view is the author's maintenance page, and its way in is on the
// desk; a rail that pointed at it again would put an author's count beside
// every note a reader opens.
func TestTheFootLeavesTheHealthViewToTheDesk(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		var buf bytes.Buffer
		if err := railFoot(nav.Vault{Name: "example-vault", Notes: 2}, lang).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render: %v", err)
		}
		foot := buf.String()
		if !strings.Contains(foot, "example-vault") {
			t.Fatalf("the foot does not name the folder, so it is not the foot this test means to read: %q", foot)
		}
		for _, gone := range []string{`href="/health"`, wording.HealthTitle.In(lang), "data-health"} {
			if strings.Contains(foot, gone) {
				t.Errorf("the foot in %s still carries %q: %q", lang, gone, foot)
			}
		}
	}
}
