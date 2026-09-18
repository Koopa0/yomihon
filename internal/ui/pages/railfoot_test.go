package pages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// TestTheFootAnswersInSentences holds the two states a folder can be in that
// have no number to show. Both are ordinary — a folder whose path yields no
// name, and a folder with nothing to answer for — and a face that answered
// either with a blank or a bare 0 would read as a count nobody had taken.
func TestTheFootAnswersInSentences(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		if got := libraryName("", lang); got == "" {
			t.Errorf("libraryName(%q) is blank for a folder with no name to show", lang)
		}
		if got := libraryName("example-vault", lang); got != "example-vault" {
			t.Errorf("libraryName() = %q, want the folder's own name", got)
		}
		for _, findings := range []int{0, 1, 2} {
			got := libraryFindings(findings, lang)
			if got == "" {
				t.Errorf("libraryFindings(%d, %q) is blank", findings, lang)
			}
			if findings == 0 && strings.Contains(got, "0") {
				t.Errorf("libraryFindings(0, %q) = %q, which states a number rather than saying there is nothing", lang, got)
			}
			if findings > 0 && !strings.Contains(got, string(rune('0'+findings))) {
				t.Errorf("libraryFindings(%d, %q) = %q and does not carry the number", findings, lang, got)
			}
		}
		if libraryFindings(1, lang) == libraryFindings(2, lang) {
			t.Errorf("one finding and two read alike in %q", lang)
		}
	}
	if libraryHealthState(0) == libraryHealthState(1) {
		t.Error("a folder with nothing to answer for and one with something carry the same dot")
	}
}
