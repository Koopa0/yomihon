package layouts

import (
	"bytes"
	"context"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// arrivalTargets is every block of content a page's arrival moves: the column
// of a note, the shelf of a desk or folder or index, a study path, the
// listening page, the whole-folder view and the pages a reader reaches by
// accident. The header, the rails and <main> are not among them, and the test
// below refuses them by name.
const arrivalTargets = ":is(.y-article, .y-healthpage, .y-home, .y-listen, .y-recovery, .y-syl)"

// arrivalGate is what keeps the arrival from playing for a document that is not
// the answer to a press, or that opens at a place in it. Both attributes are
// the ones the head script writes, once, and the other test below holds the two
// halves to each other. The gate reads only attributes: a condition the
// stylesheet worked out for itself, such as the fragment's own target, stops
// and starts matching as the reader moves about the page, which plays the
// arrival again each time it starts.
const arrivalGate = "html:not([data-prerender], [data-arrival])"

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

// TestPageContentComesForward pins the arrival: the keyframes, which say only
// where it starts, the one rule that plays it, and what that rule is allowed to
// name. It pins what the arrival is made of and the bounds that keep it small,
// and not the starting values: those are tuned by eye, one line at a time, and
// a lock that spelled them out would turn every round of tuning into a change
// to two files.
//
// The keyframes carry no end stop and the fill is held backwards because
// anything left standing after the animation would be a scale on the block,
// which makes it the containing block of every fixed descendant. Writing the
// end stop, or a fill that holds, is how a harmless fade turns into a layout
// change nobody can see until a fixed control jumps.
//
// A scale moves each point by its distance from the origin, so the origin is as
// much a part of the arrival as the scale is. At the block's own centre the
// first line of a note three thousand pixels tall travelled about twenty
// pixels, against two on a short page, and the rule has to keep naming a
// vertical origin that a viewport-sized cap holds near the top.
func TestPageContentComesForward(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := blankComments(string(source))

	keyframes := regexp.MustCompile(`(?s)@keyframes y-come-forward \{(.*?)\n\}`).FindAllStringSubmatch(css, -1)
	if len(keyframes) != 1 {
		t.Fatalf("%s writes %d @keyframes y-come-forward blocks, want 1", path, len(keyframes))
	}
	start := regexp.MustCompile(`^from \{ (.*) \}$`).FindStringSubmatch(squeeze(keyframes[0][1]))
	if start == nil {
		t.Fatalf("y-come-forward keyframes = %q, want a single from frame: only a start, so nothing of the animation is left once it ends", squeeze(keyframes[0][1]))
	}
	frame := declarations(t, "the y-come-forward start frame", start[1])
	if len(frame) != 2 || frame["opacity"] == "" || frame["scale"] == "" {
		t.Errorf("y-come-forward start frame declares %v, want opacity and scale and nothing else: no translate, because nothing climbs, and no blur, because a filter over a layer as tall as the column is repainted on every press", frame)
	}
	if opacity, err := strconv.ParseFloat(frame["opacity"], 64); err != nil || opacity <= 0 || opacity >= 1 {
		t.Errorf("y-come-forward starts at opacity %q, want a number above 0 and below 1: a fade, not a blank frame and not none", frame["opacity"])
	}
	// The scale's floor is the arrival's other half of the same bound: a point
	// moves by (1 - scale) times its distance from the origin, so a smaller
	// start than this sends the lines at the edge of the screen several times
	// further than a few pixels.
	if scale, err := strconv.ParseFloat(frame["scale"], 64); err != nil || scale < 0.97 || scale >= 1 {
		t.Errorf("y-come-forward starts at scale %q, want a number from 0.97 up to but not including 1, so the lines on screen move only a few pixels", frame["scale"])
	}

	const playing = "animation: y-come-forward "
	if got := strings.Count(css, "y-come-forward"); got != 2 {
		t.Errorf("%s names y-come-forward %d times, want 2: the keyframes and the one rule that plays them", path, got)
	}
	at := strings.Index(css, playing)
	if at < 0 || strings.Count(css, playing) != 1 {
		t.Fatalf("%s writes %q %d times, want exactly 1", path, playing, strings.Count(css, playing))
	}
	open := strings.LastIndex(css[:at], "{")
	closed := strings.LastIndex(css[:open], "}")
	end := at + strings.Index(css[at:], "}")
	// A list may be laid out one name to a line inside its parentheses.
	selector := strings.NewReplacer("( ", "(", " )", ")").Replace(squeeze(css[closed+1 : open]))
	if want := arrivalGate + " " + arrivalTargets; selector != want {
		t.Errorf("the rule playing y-come-forward selects %q, want %q", selector, want)
	}
	rule := declarations(t, "the rule playing y-come-forward", css[open+1:end])
	if len(rule) != 2 {
		t.Errorf("the rule playing y-come-forward declares %v, want the animation and its transform-origin and nothing else", rule)
	}
	// The tempo is the arrival's own pair, so tuning how a page arrives moves
	// neither the drawer, the sheet nor the palette, which share the other one.
	if got, want := rule["animation"], "y-come-forward var(--dur-arrival) var(--ease-arrival) backwards"; got != want {
		t.Errorf("the rule playing y-come-forward animates %q, want %q: the arrival's own duration and curve, and a backwards fill so nothing is left standing", got, want)
	}
	if !regexp.MustCompile(`^50% min\(50%, \d+(\.\d+)?vh\)$`).MatchString(rule["transform-origin"]) {
		t.Errorf("the rule playing y-come-forward sets transform-origin %q, want 50%% min(50%%, <n>vh): the centre of the part of the column on screen at the top, or of the block where it is shorter, so a long note and a short page arrive alike", rule["transform-origin"])
	}

	// The gate is attributes and nothing else. A pseudo-class that follows the
	// reader (:target, :has, :hover, :focus-within) would drop out of matching
	// and back in, and the animation would start again each time it came back.
	gate := regexp.MustCompile(`^html:not\((.*?)\) :is\(`).FindStringSubmatch(selector)
	if gate == nil {
		t.Fatalf("the rule playing y-come-forward selects %q, which has no html:not(...) gate to read", selector)
	}
	for clause := range strings.SplitSeq(gate[1], ", ") {
		if !regexp.MustCompile(`^\[data-[a-z]+(='[a-z]+')?\]$`).MatchString(clause) {
			t.Errorf("arrival gate clause %q is not a data attribute the head script wrote once; a condition that follows the reader replays the arrival when it matches again", clause)
		}
	}
	for _, refused := range []string{"main", ".y-main", ".y-header", ".y-rail", ".y-prefs", ".yomihon"} {
		if strings.Contains(arrivalTargets, refused) {
			t.Errorf("arrival targets %q name %q, which must stay still while the content moves", arrivalTargets, refused)
		}
	}
}

// TestArrivalOwnsItsTempo holds the arrival's duration and curve to the pair of
// tokens made for it. It checks whose tempo it is and not what the tempo is,
// so any duration or curve may be tried in one line: each token is declared
// once and is not an alias, because one that aliased --dur-slow would move the
// drawer, the sheet and the palette whenever the arrival was tuned, and it is
// read once, because a second reader would move with it.
func TestArrivalOwnsItsTempo(t *testing.T) {
	t.Parallel()
	var tokens, components string
	{
		const path = "../../../assets/css/tokens.css"
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		tokens = blankComments(string(source))
	}
	{
		const path = "../../../assets/css/components.css"
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		components = blankComments(string(source))
	}

	for _, token := range []string{"--dur-arrival", "--ease-arrival"} {
		declared := regexp.MustCompile(`(?m)^\s*`+token+`:\s*([^;]+);$`).FindAllStringSubmatch(tokens, -1)
		switch {
		case len(declared) != 1:
			t.Errorf("tokens.css declares %s %d times, want 1", token, len(declared))
		case strings.Contains(declared[0][1], "var("):
			t.Errorf("tokens.css declares %s as %q, an alias; the arrival's tempo is its own, so tuning it moves nothing else", token, declared[0][1])
		}
		if got := strings.Count(components, "var("+token+")"); got != 1 {
			t.Errorf("components.css reads %s %d times, want 1 (the arrival), because anything else reading it moves when the arrival is tuned", token, got)
		}
	}
}

// TestPressAnsweredInPlaceCarriesNoTravel holds the two smaller surfaces that
// share the page's rule: what answers a press in place comes forward, and
// what lives at an edge slides from that edge. The status reply is a single
// line, which only fades, because a line of text that moves or changes size
// reads as a wobble. The palette is summoned, so it keeps the summoned
// tempo, and it grows from its top edge instead of dropping.
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

// TestArrivalGateNamesWhatTheHeadScriptWrites holds the stylesheet's two guards
// to the script that sets them. A guard naming an attribute nothing writes is a
// guard that never fires, and a rewritten script would keep passing every other
// test while the arrival played on a page restored from history or reloaded
// where the reader had scrolled to.
func TestArrivalGateNamesWhatTheHeadScriptWrites(t *testing.T) {
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
		`if (entry?.type === "back_forward") d.dataset.arrival = "traverse";`,
		`else if (entry?.type === "reload") d.dataset.arrival = "reload";`,
		`const kept = new URLSearchParams(location.search).has("at");`,
		`else if (kept || entry?.name.includes("#")) d.dataset.arrival = "place";`,
		`if (document.prerendering) {`,
		`d.dataset.prerender = "";`,
		`document.onprerenderingchange = () => delete d.dataset.prerender;`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("head script lacks %q; script = %q", want, script)
		}
	}
	for _, want := range []string{"[data-prerender]", "[data-arrival]"} {
		if !strings.Contains(arrivalGate, want) {
			t.Errorf("arrival gate %q lacks %s, which the head script writes", arrivalGate, want)
		}
	}
}

// TestArrivalPlaceNamesTheQueryAKeptPlaceCarries holds the head script's
// reading of a returning reader to the two files that agree on the name. A kept
// place travels as an anchor and a distance below it, and the distance rides in
// a query that the page's own module spends once it has scrolled there; the
// address is built in one package and read in another. The head script is a
// third reader of that name, so renaming it on either side without it would
// send a reader back to a long note with the arrival playing over text moved by
// the scale, and no other test would notice.
func TestArrivalPlaceNamesTheQueryAKeptPlaceCarries(t *testing.T) {
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

// TestArrivalGateAttributesBelongToTheHeadScriptAlone holds the two root
// attributes to one writer. The head script runs first and the client modules
// run after it, so a module that kept state under the same name would
// overwrite the head script's answer and the arrival would play on a page it
// was gated off. That is not hypothetical: the drawer keeps its state in
// data-nav, which is why the history guard is not called that.
func TestArrivalGateAttributesBelongToTheHeadScriptAlone(t *testing.T) {
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
					t.Errorf("%s mentions %q; the head script alone writes the attribute the arrival gate reads, and a module keeping state under the same name would overwrite it", entry.Name(), spelling)
				}
			}
		}
	}
	if read == 0 {
		t.Fatalf("no client modules found under %s, so nothing was checked", dir)
	}
}

// TestArrivalIsAnsweredByTheReducedMotionBlanket holds the claim that a reader
// who asked for less motion is not made to watch the arrival. The blanket
// reaches everything under the interface's root, so the content has to sit
// under it: it does when the root element wraps the children a page supplies.
func TestArrivalIsAnsweredByTheReducedMotionBlanket(t *testing.T) {
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
