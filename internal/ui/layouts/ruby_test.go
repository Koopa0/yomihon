package layouts

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestFuriganaIsNeverFainterOrSmallerThanTheProseAllows holds the two things
// that made a reading harder to read than the word it explains. Furigana was
// set at half the body size, which is the smallest text anywhere on the page,
// and in an ink one step lighter than the prose it annotates — so the glyphs a
// learner most needs were both the smallest and the faintest thing in front of
// them, on a product whose charter calls ruby a first-class citizen.
//
// The ink is checked against the prose's own rather than against a value
// written here twice: what matters is not which token it is but that a reading
// is never fainter than the text it belongs to. The size is checked as a
// floor, and in em, because the reader's type choice moves the body and the
// reading has to move with it.
func TestFuriganaIsNeverFainterOrSmallerThanTheProseAllows(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := cssComments.ReplaceAllString(string(source), "")

	ruby := cssDeclarations(t, ruleBody(t, css, ".yomihon rt {"))
	prose := cssDeclarations(t, ruleBody(t, css, ".y-prose {"))
	if prose["color"] == "" {
		t.Fatal("the prose declares no colour, so there is nothing to hold the reading's ink against")
	}
	if ruby["color"] != prose["color"] {
		t.Errorf("furigana is inked %q while the prose it annotates is inked %q; a reading fainter than its own word asks the reader to work hardest on the part they know least", ruby["color"], prose["color"])
	}

	// The floor, and the unit it has to be written in. A length in px or rem
	// would hold still while the reader enlarged everything around it.
	const floor = 0.55
	size := ruby["font-size"]
	if !strings.HasSuffix(size, "em") || strings.HasSuffix(size, "rem") {
		t.Fatalf("furigana is sized %q; it has to be relative to the body it sits over, or the reader's own type choice leaves it behind", size)
	}
	given, err := strconv.ParseFloat(strings.TrimSuffix(size, "em"), 64)
	if err != nil {
		t.Fatalf("furigana is sized %q, which is not a length this check can compare: %v", size, err)
	}
	if given < floor {
		t.Errorf("furigana is set at %vem of the body, want at least %vem; below that it is the smallest text on a page whose reader is learning to read exactly those glyphs", given, floor)
	}

	// One em-relative rule answers for every type step, and it can only do
	// that while no step writes its own. Each step moves the body sizes and
	// nothing else, so a size for the reading appearing inside one would be a
	// second answer the rule above cannot see. The steps live in the token
	// sheet, so that is where this looks, and it counts what it found first —
	// a scan that matched nothing would otherwise report every step clean.
	const tokens = "../../../assets/css/tokens.css"
	tokenSource, err := os.ReadFile(tokens)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", tokens, err)
	}
	steps := regexp.MustCompile(`(?s):root\[data-textsize="[a-z]+"\][^{]*\{[^}]*\}`).
		FindAllString(cssComments.ReplaceAllString(string(tokenSource), ""), -1)
	if len(steps) < 2 {
		t.Fatalf("found %d type steps in %s, want at least the two that move the body away from the default; this scan is looking in the wrong place", len(steps), tokens)
	}
	for _, step := range steps {
		if rubySizedPerStep.MatchString(step) {
			t.Errorf("a type step sizes the reading itself, which the em-relative rule cannot see:\n%s", step)
		}
	}
}

// rubySizedPerStep matches a declaration inside a type step that sizes the
// ruby text. The word boundary keeps ordinary token names out of it.
var rubySizedPerStep = regexp.MustCompile(`\brt\b|--fs-rt`)
