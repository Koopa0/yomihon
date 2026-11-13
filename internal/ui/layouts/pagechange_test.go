package layouts

import (
	"bytes"
	"context"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// contentBlocks is every block of content a page puts under the header: the
// column of a note, the shelf of a desk or folder or index, a course, the
// listening page, the whole-folder view and the pages a reader reaches by
// accident. A page change plays no animation, so none of them carries one.
const contentBlocks = "article|healthpage|home|listen|recovery|syl"

// squeeze turns every run of white space into one space, so a rule is compared
// by what it says and not by how it is laid out.
func squeeze(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// declarations reads the body of a rule into property and value. A property
// written twice is an error, because a later declaration would quietly
// override the one the caller went on to check.
func declarations(t *testing.T, where, body string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for declaration := range strings.SplitSeq(body, ";") {
		declaration = strings.TrimSpace(declaration)
		if declaration == "" {
			continue
		}
		name, value, ok := strings.Cut(declaration, ":")
		if !ok {
			t.Errorf("%s holds %q, which is not a declaration", where, declaration)
			continue
		}
		name = strings.TrimSpace(name)
		if _, again := out[name]; again {
			t.Errorf("%s declares %s twice", where, name)
		}
		out[name] = squeeze(value)
	}
	return out
}

// ruleBodies returns the body of every rule written with exactly this
// selector, including one nested in @starting-style. The bodies the arrival
// reads hold no nested braces.
func ruleBodies(css, selector string) []string {
	var bodies []string
	for _, match := range regexp.MustCompile(`(?m)^\s*`+regexp.QuoteMeta(selector)+` \{([^}]*)\}`).FindAllStringSubmatch(css, -1) {
		bodies = append(bodies, match[1])
	}
	return bodies
}

// cssRules lists every rule of a sheet whose body holds no nested block, as the
// selector it was written with and its body. A rule inside @media is read like
// any other; an at-rule's own header, which has no body of declarations, is not
// a rule.
func cssRules(css string) []struct{ selector, body string } {
	var rules []struct{ selector, body string }
	for _, match := range regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatch(css, -1) {
		rules = append(rules, struct{ selector, body string }{squeeze(match[1]), match[2]})
	}
	return rules
}

// readSheet reads a stylesheet with its comments blanked, so a rule is compared
// by what it declares and not by what the prose beside it says.
func readSheet(t *testing.T, name string) string {
	t.Helper()
	path := "../../../assets/css/" + name
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return blankComments(string(source))
}

// TestAPageChangePlaysNoAnimation holds the one thing a reader sees on every
// press: the block of content is painted whole, at full opacity and size, and
// nothing declares an animation on it. It reads the declarations and not a
// moment of a running animation, which a check would have to catch mid-flight.
//
// Three things stand for the old arrival and each is refused by name: the
// keyframes, the tokens that tuned it, and the cross-document view transition
// that was measured as an alternative and delays the first paint by about
// 300ms. A rule that animates a content block under another name is caught by
// what it declares and where, so renaming the keyframes does not get past it.
func TestAPageChangePlaysNoAnimation(t *testing.T) {
	t.Parallel()
	components := readSheet(t, "components.css")
	tokens := readSheet(t, "tokens.css")

	for name, sheet := range map[string]string{"components.css": components, "tokens.css": tokens} {
		for _, gone := range []string{"y-come-forward", "--dur-arrival", "--ease-arrival", "@view-transition", "data-prerender"} {
			if strings.Contains(sheet, gone) {
				t.Errorf("%s still names %q; a page change plays no animation, so nothing of the arrival is left", name, gone)
			}
		}
	}

	block := regexp.MustCompile(`\.y-(` + contentBlocks + `)$`)
	var checked int
	for _, rule := range cssRules(components) {
		for selector := range strings.SplitSeq(rule.selector, ",") {
			fields := strings.Fields(strings.TrimSpace(selector))
			if len(fields) == 0 || !block.MatchString(fields[len(fields)-1]) {
				continue
			}
			checked++
			for name := range declarations(t, selector, rule.body) {
				if name == "animation" || strings.HasPrefix(name, "animation-") || name == "transition" || strings.HasPrefix(name, "transition-") {
					t.Errorf("the rule for %q declares %s; the block of content under the header is painted whole and nothing moves it", strings.TrimSpace(selector), name)
				}
			}
		}
	}
	if checked < len(strings.Split(contentBlocks, "|")) {
		t.Errorf("only %d rules for the content blocks %s were read, so this check read almost none of them", checked, contentBlocks)
	}
}

// TestOnlyAKeptPlaceHoldsTheColumnBack holds the one case that is not painted at
// once. A reader returning to a kept place has the column hidden until the
// module has scrolled to it, and the stylesheet reveals it on a timer if the
// module never does, so a script that fails never leaves a blank page. The rule
// is the only one in the sheet that reads the mark, it is held to the scripted
// document, and the fallback is declared as the rule's own animation, so
// removing either the delay or the keyframes leaves the page hidden for good.
func TestOnlyAKeptPlaceHoldsTheColumnBack(t *testing.T) {
	t.Parallel()
	css := readSheet(t, "components.css")

	var marked []struct{ selector, body string }
	for _, rule := range cssRules(css) {
		if strings.Contains(rule.selector, "data-arrival") {
			marked = append(marked, rule)
		}
	}
	if len(marked) != 1 {
		t.Fatalf("components.css has %d rules that read the arrival mark, want 1: the one that holds a kept place's column back", len(marked))
	}
	if got, want := strings.ReplaceAll(marked[0].selector, `"`, "'"), "html[data-js][data-arrival='kept'] .y-main"; got != want {
		t.Errorf("the rule holding the column back selects %q, want %q: only the scripted document the head script marked, and only the column", got, want)
	}
	rule := declarations(t, "the rule holding the column back", marked[0].body)
	if len(rule) != 2 || rule["visibility"] != "hidden" {
		t.Errorf("the rule holding the column back declares %v, want visibility: hidden and its own fallback animation and nothing else", rule)
	}
	if got, want := rule["animation"], "y-kept-reveal 0s 400ms forwards"; got != want {
		t.Errorf("the rule holding the column back animates %q, want %q: a reveal that waits 400ms and then stays, so a page whose script failed is shown", got, want)
	}

	keyframes := regexp.MustCompile(`(?s)@keyframes y-kept-reveal \{(.*?)\n\}`).FindAllStringSubmatch(css, -1)
	if len(keyframes) != 1 {
		t.Fatalf("components.css writes %d @keyframes y-kept-reveal blocks, want 1", len(keyframes))
	}
	if got, want := squeeze(keyframes[0][1]), "to { visibility: visible; }"; got != want {
		t.Errorf("y-kept-reveal keyframes = %q, want %q: only the end frame, which shows what the rule hid", got, want)
	}
}

// TestTheHeadScriptWritesOnlyTheKeptPlaceMark holds the head script to the two
// things it still says: the document is scripted, and the document is a return
// to a kept place. The marks that once told a back, a reload, a fragment or a
// prepared page apart existed only for the arrival, and a page change has none
// to play, so none of them is written.
func TestTheHeadScriptWritesOnlyTheKeptPlaceMark(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Base(Chrome{Title: "測試", Nonce: "response-nonce"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render base: %v", err)
	}
	html := buf.String()
	start := strings.Index(html, "<script")
	end := strings.Index(html, "</script>")
	if start < 0 || end < start {
		t.Fatalf("Base() has no head script to read; html = %q", html)
	}
	// Compared by what it says: the formatter wraps a long statement, and a
	// wrap is not a change of meaning.
	script := squeeze(html[start:end])
	for _, want := range []string{
		`d.dataset.js = "on";`,
		`new URLSearchParams(location.search).has("at")`,
		`d.dataset.arrival = "kept";`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("head script lacks %q; script = %q", want, script)
		}
	}
	if got := strings.Count(script, "dataset.arrival"); got != 1 {
		t.Errorf("head script writes the arrival mark %d times, want 1 (the kept place); script = %q", got, script)
	}
	for _, gone := range []string{"traverse", "reload", "back_forward", "prerender", `"place"`, "getEntriesByType"} {
		if strings.Contains(script, gone) {
			t.Errorf("head script still holds %q, which only served the arrival; script = %q", gone, script)
		}
	}
}

// TestPressAnsweredInPlaceCarriesNoTravel holds the two small surfaces that
// answer a press in place: the status reply is a single line, which only fades,
// because a line of text that moves or changes size reads as a wobble, and the
// palette is summoned, so it keeps the summoned tempo and grows from its top
// edge instead of dropping.
func TestPressAnsweredInPlaceCarriesNoTravel(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := blankComments(string(source))

	reply := regexp.MustCompile(`(?s)@keyframes y-reply-in \{(.*?)\n\}`).FindAllStringSubmatch(css, -1)
	if len(reply) != 1 {
		t.Fatalf("%s writes %d @keyframes y-reply-in blocks, want 1", path, len(reply))
	}
	if got := squeeze(reply[0][1]); !regexp.MustCompile(`^from \{ opacity: [0-9.]+; \}$`).MatchString(got) {
		t.Errorf("y-reply-in keyframes = %q, want a single from frame that sets opacity and nothing else", got)
	}

	var palette []string
	for _, selector := range []string{".y-searchdialog", ".y-searchdialog[open]"} {
		palette = append(palette, ruleBodies(css, selector)...)
	}
	var grows bool
	for _, body := range palette {
		rule := declarations(t, "a palette rule", body)
		for _, travel := range []string{"transform", "translate"} {
			if _, ok := rule[travel]; ok {
				t.Errorf("a palette rule declares %s: %q; the palette comes forward by scale from its top edge and nothing in it climbs or drops", travel, rule[travel])
			}
		}
		if strings.Contains(rule["transition"], "transform") {
			t.Errorf("the palette transitions transform in %q, want scale", rule["transition"])
		}
		if rule["transform-origin"] == "top" && rule["scale"] != "" && strings.Contains(rule["transition"], "scale var(--dur-slow) var(--ease-standard)") {
			grows = true
		}
	}
	if !grows {
		t.Errorf("no palette rule closes at a scale from transform-origin: top and transitions it over --dur-slow and --ease-standard, so the palette does not come forward from its top edge and leave the same way back")
	}
}

// TestTheKeptPlaceNamesTheQueryTheHeadScriptReads holds the head script's
// reading of a returning reader to the two files that agree on the name. A kept
// place travels as an anchor and a distance below it, and the distance rides in
// a query that the page's own module spends once it has scrolled there; the
// address is built in one package and read in another. The head script is a
// third reader of that name, so renaming it on either side without it would
// leave the column showing at the top of a long note before the module moved it
// to the place, and no other test would notice.
func TestTheKeptPlaceNamesTheQueryTheHeadScriptReads(t *testing.T) {
	t.Parallel()
	for _, reader := range []struct{ path, want string }{
		{"../../../assets/js/mark.js", `const OFFSET_PARAM = 'at';`},
		{"../pages/href.go", `const resumeOffsetParam = "at"`},
	} {
		source, err := os.ReadFile(reader.path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", reader.path, err)
		}
		if !strings.Contains(string(source), reader.want) {
			t.Errorf("%s lacks %q; the head script reads the same query name as %q in Base, so the three have to change together", reader.path, reader.want, `new URLSearchParams(location.search).has("at")`)
		}
	}
}

// TestOnlyTheMarkModuleClearsTheKeptPlaceMark holds the root attribute to one
// writer and one reader. The head script sets it, and the module that places
// the page removes it once it has; no other module may keep state under the
// name, because it would overwrite the head script's answer or reveal a
// column that has not been placed. The attribute that served the arrival and
// the preparation of a page are no longer written anywhere.
func TestOnlyTheMarkModuleClearsTheKeptPlaceMark(t *testing.T) {
	t.Parallel()
	const dir = "../../../assets/js"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", dir, err)
	}
	var read int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		source, err := os.ReadFile(dir + "/" + entry.Name())
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", entry.Name(), err)
		}
		read++
		text := string(source)
		for _, spelling := range []string{"dataset.prerender", "data-prerender", "data-arrival"} {
			if strings.Contains(text, spelling) {
				t.Errorf("%s mentions %q; nothing is prepared or arrives any more, and the one mark is the head script's", entry.Name(), spelling)
			}
		}
		for _, set := range regexp.MustCompile(`dataset\.arrival\s*=`).FindAllString(text, -1) {
			t.Errorf("%s writes the arrival mark with %q; the head script alone sets it", entry.Name(), set)
		}
		if entry.Name() != "mark.js" && strings.Contains(text, "dataset.arrival") {
			t.Errorf("%s reads the arrival mark; the module that places a kept place alone removes it", entry.Name())
		}
	}
	if read == 0 {
		t.Fatalf("no client modules found under %s, so nothing was checked", dir)
	}
}

// TestTheReducedMotionBlanketReachesThePageContent holds the claim that a reader
// who asked for less motion is not made to watch any of what still moves. The
// blanket reaches everything under the interface's root, so the content has to
// sit under it: it does when the root element wraps the children a page
// supplies.
func TestTheReducedMotionBlanketReachesThePageContent(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := squeeze(blankComments(string(source)))
	const blanket = "@media (prefers-reduced-motion: reduce) { .yomihon *:not(.y-readline), .yomihon *::before, .yomihon *::after { animation-duration: 0.001ms !important;"
	if !strings.Contains(css, blanket) {
		t.Errorf("%s lacks the reduced-motion blanket %q", path, blanket)
	}

	content := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, `<article class="y-article">content-marker</article>`)
		return err
	})
	var buf bytes.Buffer
	if err := Base(Chrome{Title: "測試", Nonce: "response-nonce"}).Render(templ.WithChildren(t.Context(), content), &buf); err != nil {
		t.Fatalf("render base: %v", err)
	}
	html := buf.String()
	root := strings.Index(html, `<div class="yomihon">`)
	child := strings.Index(html, "content-marker")
	if root < 0 || child < root {
		t.Errorf("page content at %d is not inside the root element at %d, so the blanket would not reach it; html = %q", child, root, html)
	}
}

// TestTheJumpedToHeadingEchoIsInkNotVermilion holds the wash a jump leaves on
// the heading it lands on to the ink the interface's other states use. Vermilion
// belongs to the reader's own marks, the bookmark and the diamond, and a jump
// is not one: a heading that flashed red said "you marked this" about a place
// the reader had only arrived at. The keyframes are read for what they name, so
// a different color token spelled for the same wash is caught too.
func TestTheJumpedToHeadingEchoIsInkNotVermilion(t *testing.T) {
	t.Parallel()
	css := readSheet(t, "components.css")
	frames := regexp.MustCompile(`(?s)@keyframes y-arrive \{(.*?)\n\}`).FindAllStringSubmatch(css, -1)
	if len(frames) != 1 {
		t.Fatalf("components.css writes %d @keyframes y-arrive blocks, want 1", len(frames))
	}
	body := frames[0][1]
	if strings.Contains(body, "--accent") {
		t.Errorf("the jumped-to heading's echo names a vermilion token: %q; vermilion is the reader's own marks and nothing else", squeeze(body))
	}
	if !strings.Contains(body, "var(--wash-strong)") {
		t.Errorf("the jumped-to heading's echo = %q, want the ink wash --wash-strong the interface's other states use", squeeze(body))
	}
}
