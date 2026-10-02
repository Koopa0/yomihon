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

// riseTargets is every block of content a page's arrival moves: the column of
// a note, the shelf of a desk or folder or index, a study path, the listening
// page, the whole-folder view and the pages a reader reaches by accident. The
// header, the rails and <main> are not among them, and the test below refuses
// them by name.
const riseTargets = ":is(.y-article, .y-healthpage, .y-home, .y-listen, .y-recovery, .y-syl)"

// riseGate is what keeps the arrival from playing for a document that is not
// the answer to a press, or that opens at a place in it. The two attributes are
// the ones the head script writes, and the other test below holds the two
// halves to each other. The third clause is the stylesheet's own: a fragment
// scroll done while the block is still low would land the heading a few pixels
// off.
const riseGate = "html:not([data-prerender], [data-arrival='traverse'], :has(:target))"

// squeeze turns every run of white space into one space, so a rule is compared
// by what it says and not by how it is laid out.
func squeeze(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// TestPageContentRisesIntoPlace pins the arrival: the keyframes, which say only
// where it starts, the one rule that plays it, and what that rule is allowed to
// name. The keyframes carry no end stop and the fill is held backwards because
// anything left standing after the animation would be a transform on the block,
// which makes it the containing block of every fixed descendant. Writing the
// end stop, or a fill that holds, is how a harmless fade turns into a layout
// change nobody can see until a fixed control jumps.
func TestPageContentRisesIntoPlace(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := blankComments(string(source))

	keyframes := regexp.MustCompile(`(?s)@keyframes y-rise-in \{(.*?)\n\}`).FindAllStringSubmatch(css, -1)
	if len(keyframes) != 1 {
		t.Fatalf("%s writes %d @keyframes y-rise-in blocks, want 1", path, len(keyframes))
	}
	if got, want := squeeze(keyframes[0][1]), "from { opacity: 0.55; transform: translateY(6px); }"; got != want {
		t.Errorf("y-rise-in keyframes = %q, want %q: only a start, so nothing of the animation is left once it ends", got, want)
	}

	const declaration = "animation: y-rise-in var(--dur-slow) var(--ease-standard) backwards;"
	if got := strings.Count(css, "y-rise-in"); got != 2 {
		t.Errorf("%s names y-rise-in %d times, want 2: the keyframes and the one rule that plays them", path, got)
	}
	at := strings.Index(css, declaration)
	if at < 0 || strings.Count(css, declaration) != 1 {
		t.Fatalf("%s writes %q %d times, want exactly 1", path, declaration, strings.Count(css, declaration))
	}
	open := strings.LastIndex(css[:at], "{")
	closed := strings.LastIndex(css[:open], "}")
	// A list may be laid out one name to a line inside its parentheses.
	selector := strings.NewReplacer("( ", "(", " )", ")").Replace(squeeze(css[closed+1 : open]))
	if want := riseGate + " " + riseTargets; selector != want {
		t.Errorf("the rule playing y-rise-in selects %q, want %q", selector, want)
	}
	if body := css[open+1 : at+len(declaration)]; squeeze(body) != declaration {
		t.Errorf("the rule playing y-rise-in carries %q, want only %q", squeeze(body), declaration)
	}
	for _, refused := range []string{"main", ".y-main", ".y-header", ".y-rail", ".y-prefs", ".yomihon"} {
		if strings.Contains(riseTargets, refused) {
			t.Errorf("rise targets %q name %q, which must stay still while the content moves", riseTargets, refused)
		}
	}
}

// TestRiseGateNamesWhatTheHeadScriptWrites holds the stylesheet's two guards to
// the script that sets them. A guard naming an attribute nothing writes is a
// guard that never fires, and a rewritten script would keep passing every other
// test while the arrival played on a page restored from history.
func TestRiseGateNamesWhatTheHeadScriptWrites(t *testing.T) {
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
	script := html[start:end]
	for _, want := range []string{
		`if (entry?.type === "back_forward") d.dataset.arrival = "traverse";`,
		`if (document.prerendering) {`,
		`d.dataset.prerender = "";`,
		`document.onprerenderingchange = () => delete d.dataset.prerender;`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("head script lacks %q; script = %q", want, script)
		}
	}
	for _, want := range []string{"[data-prerender]", "[data-arrival='traverse']", ":has(:target)"} {
		if !strings.Contains(riseGate, want) {
			t.Errorf("rise gate %q lacks %s, which the head script writes", riseGate, want)
		}
	}
}

// TestRiseGateAttributesBelongToTheHeadScriptAlone holds the two root
// attributes to one writer. The head script runs first and the client modules
// run after it, so a module that kept state under the same name would
// overwrite the head script's answer and the arrival would play on a page it
// was gated off. That is not hypothetical: the drawer keeps its state in
// data-nav, which is why the history guard is not called that.
func TestRiseGateAttributesBelongToTheHeadScriptAlone(t *testing.T) {
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
		for _, claimed := range []string{"prerender", "arrival"} {
			for _, spelling := range []string{"dataset." + claimed, "data-" + claimed} {
				if strings.Contains(string(source), spelling) {
					t.Errorf("%s mentions %q; the head script alone writes the attribute the rise gate reads, and a module keeping state under the same name would overwrite it", entry.Name(), spelling)
				}
			}
		}
	}
	if read == 0 {
		t.Fatalf("no client modules found under %s, so nothing was checked", dir)
	}
}

// TestRiseIsAnsweredByTheReducedMotionBlanket holds the claim that a reader who
// asked for less motion is not made to watch the arrival. The blanket reaches
// everything under the interface's root, so the content has to sit under it:
// it does when the root element wraps the children a page supplies.
func TestRiseIsAnsweredByTheReducedMotionBlanket(t *testing.T) {
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
