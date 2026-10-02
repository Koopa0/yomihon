package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
			got := libraryFindings(findings, false, lang)
			if got == "" {
				t.Errorf("libraryFindings(%d, %q) is blank", findings, lang)
			}
			if findings == 0 && strings.Contains(got, "0") {
				t.Errorf("libraryFindings(0, %q) = %q, which states a number rather than saying there is nothing", lang, got)
			}
			if findings > 0 && !strings.Contains(got, strconv.Itoa(findings)) {
				t.Errorf("libraryFindings(%d, %q) = %q and does not carry the number", findings, lang, got)
			}
		}
		if libraryFindings(1, false, lang) == libraryFindings(2, false, lang) {
			t.Errorf("one finding and two read alike in %q", lang)
		}
	}
	if libraryHealthState(0, false) == libraryHealthState(1, false) {
		t.Error("a folder with nothing to answer for and one with something carry the same dot")
	}
}

// footCount is the words the foot's count carries, read out of the rendered foot.
var footCount = regexp.MustCompile(`<span class="y-railfoot__count">([^<]*)</span>`)

func renderedFoot(t *testing.T, v nav.Vault, lang wording.Lang) (foot, count string) {
	t.Helper()
	var buf bytes.Buffer
	if err := railFoot(v, lang).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	found := footCount.FindStringSubmatch(buf.String())
	if found == nil {
		t.Fatalf("the foot carries no count: %q", buf.String())
	}
	return buf.String(), found[1]
}

// TestTheFootsDotNeverSaysClearOverANotice holds the one case where the number
// and the state part ways. A standing notice is not in the findings count, so
// the number stays the table's own; but a folder whose only trouble is that its
// pages have stopped updating must not read "nothing found" in words with only
// the dot's colour saying otherwise, which is a state carried by colour alone.
// The words say it, in each language, and the dot agrees.
func TestTheFootsDotNeverSaysClearOverANotice(t *testing.T) {
	t.Parallel()

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		stopped := wording.NoticeNamesCollideTitle.In(lang)

		foot, count := renderedFoot(t, nav.Vault{Name: "example-vault", Noticed: true}, lang)
		if count != stopped {
			t.Errorf("the count of a folder whose pages have stopped updating reads %q in %s, want %q", count, lang, stopped)
		}
		if strings.Contains(foot, wording.LibraryNoFindings.In(lang)) {
			t.Errorf("the foot says nothing is found over a notice in %s: %q", lang, foot)
		}
		if !strings.Contains(foot, `data-health="findings"`) {
			t.Errorf("the foot of a folder with a standing notice does not carry the findings dot in %s: %q", lang, foot)
		}
		if !strings.Contains(foot, `data-rail-foot-findings="0"`) {
			t.Errorf("a notice moved the foot's number in %s, which is the table's and not the notice's: %q", lang, foot)
		}

		// Beside findings the number is the answer, and the dot already agrees.
		foot, count = renderedFoot(t, nav.Vault{Name: "example-vault", Findings: 2, Noticed: true}, lang)
		if want := libraryFindings(2, false, lang); count != want {
			t.Errorf("the count of a folder with two findings and a notice reads %q in %s, want %q", count, lang, want)
		}
		if !strings.Contains(foot, `data-health="findings"`) {
			t.Errorf("the foot with findings and a notice does not carry the findings dot in %s: %q", lang, foot)
		}

		// With no notice the foot is what it was.
		foot, count = renderedFoot(t, nav.Vault{Name: "example-vault"}, lang)
		if want := wording.LibraryNoFindings.In(lang); count != want {
			t.Errorf("the count of a clear folder reads %q in %s, want %q", count, lang, want)
		}
		if !strings.Contains(foot, `data-health="clear"`) {
			t.Errorf("the foot of a clear folder does not carry the clear dot in %s: %q", lang, foot)
		}
	}
	if libraryHealthState(0, true) != "findings" || libraryHealthState(2, true) != "findings" {
		t.Error("a standing notice does not make the dot read findings")
	}
	if libraryHealthState(0, false) != "clear" {
		t.Error("a folder with no findings and no notice does not read clear, so the dot could never say it")
	}
}
