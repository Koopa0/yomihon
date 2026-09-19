package layouts

import (
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// readingScaleSteps are the editorial sizes the reading column is built from.
// The names are written out rather than read back from the stylesheet, because
// a list the sheet supplies would agree with the sheet no matter what the sheet
// said.
var readingScaleSteps = []string{
	"--fs-ed-17",
	"--fs-ed-19",
	"--fs-ed-20",
	"--fs-ed-24",
	"--fs-ed-30",
	"--fs-ed-38",
}

// The three roots the reader's size preference resolves against, and the rule
// that narrows the two largest steps on a phone.
const (
	baseRoot       = ":root {"
	largeRoot      = `:root[data-textsize='l'] {`
	extraLargeRoot = `:root[data-textsize='xl'] {`
	narrowQuery    = "@media (max-width: 720px)"
)

// readingSizes are the three roots a reader's size choice resolves against, in
// the order the page offers them.
var readingSizes = []struct {
	name   string
	opener string
}{
	{"the base size", baseRoot},
	{"大", largeRoot},
	{"特大", extraLargeRoot},
}

// TestReadingScaleIsDeclaredAtEveryTextSize holds the editorial scale's three
// blocks to one vocabulary. A reader's size choice is an attribute on the root,
// and every step of the scale has to be answered in all three blocks: a step
// declared in the base alone is frozen at its smallest value while the prose
// around it grows, which is a heading smaller than its own paragraphs at the two
// larger sizes and a stylesheet that looks complete at the one size a developer
// reads it in. Nothing else compares the three, so this does, by name.
func TestReadingScaleIsDeclaredAtEveryTextSize(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/tokens.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := string(source)

	narrowAt := strings.Index(css, narrowQuery)
	if narrowAt < 0 {
		t.Fatalf("%s has no %q rule, so a phone-width column still carries the full-width title", path, narrowQuery)
	}
	// The three size blocks are read from the text above the narrow rule. That
	// rule names the same roots, so a search of the whole file for an opener
	// could land in it and compare a block against itself.
	blocks := css[:narrowAt]

	declared := map[string][]string{}
	for _, size := range readingSizes {
		if n := strings.Count(blocks, size.opener); n != 1 {
			t.Fatalf("%s opens the rule %q %d times above the narrow-width block, want exactly 1 — the comparison below would read some other rule", path, size.opener, n)
		}
		names := readingScaleNames(t, ruleBody(t, blocks, size.opener))
		// Three blocks that parsed to nothing would compare equal and prove
		// nothing, so each one is held against the scale's own names first.
		for _, step := range readingScaleSteps {
			if !slices.Contains(names, step) {
				t.Fatalf("%s declares %v, and does not declare %s — a size the reading column uses is missing from the block this test would otherwise compare", size.name, names, step)
			}
		}
		declared[size.opener] = names
	}

	for _, size := range readingSizes[1:] {
		if diff := cmp.Diff(declared[baseRoot], declared[size.opener]); diff != "" {
			t.Errorf("the editorial scale at %s is not the base scale (-base +%s):\n%s\na step the base declares alone stays at its smallest value while the body grows past it", size.name, size.name, diff)
		}
	}

	// Each text-size root narrows its own pair inside the media query, rather
	// than one pair shared behind a combined selector list: the base size, 大
	// and 特大 need different clamped values to keep title above L1 above L2 at
	// every size, and only a rule of its own can give a root that. A media
	// query carries no specificity, so a rule keyed on one root alone ties the
	// block above it on specificity and wins only by sitting later in the
	// file — which is why each root gets a whole rule to itself here instead of
	// a shared one.
	narrow := css[narrowAt:]
	narrowedSteps := map[string][]string{}
	for _, size := range readingSizes {
		if n := strings.Count(narrow, size.opener); n != 1 {
			t.Fatalf("the narrow-width rule opens %s (%q) %d times, want exactly 1", size.name, size.opener, n)
		}
		body := ruleBody(t, narrow, size.opener)
		names := readingScaleNames(t, body)
		if len(names) == 0 {
			t.Errorf("the narrow-width rule for %s narrows no editorial size, so nothing about it can be checked", size.name)
		}
		for _, name := range names {
			if !slices.Contains(readingScaleSteps, name) {
				t.Errorf("the narrow-width rule for %s declares %s, which is not a step of the editorial scale — a name only this rule knows is one the three size blocks above cannot be compared on", size.name, name)
			}
		}
		narrowedSteps[size.opener] = names
	}
	for _, size := range readingSizes[1:] {
		if diff := cmp.Diff(narrowedSteps[baseRoot], narrowedSteps[size.opener]); diff != "" {
			t.Errorf("%s narrows a different set of steps than the base size (-base +%s):\n%s\nevery text-size root has to clamp the same steps, each to its own amount, or a reader at one size keeps a step the others no longer carry", size.name, size.name, diff)
		}
	}
}

// readingLadder names the chain a reader's size choice must never invert: the
// note title above the first heading level above the second, down to the
// fourth, which must stay at or above the body paragraphs it introduces. The
// fifth and sixth heading levels are left out on purpose — they share the
// body's own size (headinglook_test.go owns that choice, told apart by weight
// and letter-spacing rather than by a size a sixth heading would have to lose
// to the paragraph below it), so an equal pair there is not an inversion.
var readingLadder = []struct {
	level string
	token string
}{
	{"title", "--fs-ed-38"},
	{"L1", "--fs-ed-30"},
	{"L2", "--fs-ed-24"},
	{"L3", "--fs-ed-20"},
	{"L4", "--fs-ed-19"},
	{"body", "--fs-ed-17"},
}

// TestReadingScaleOrderNeverInverts holds the heading ladder to strict
// descending order in every text-size block, both at the wide layout and
// under the narrow-width query that clamps the title and the first heading
// level on a phone. A clamp chosen without checking what it lands next to is
// how a phone-width note ends up with its second heading drawn larger than
// its first, or the first and second tied — both measured on the fixture this
// test reads before the narrow-width rule was given one pair per text size.
func TestReadingScaleOrderNeverInverts(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/tokens.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := string(source)

	narrowAt := strings.Index(css, narrowQuery)
	if narrowAt < 0 {
		t.Fatalf("%s has no %q rule", path, narrowQuery)
	}
	wideCSS, narrowCSS := css[:narrowAt], css[narrowAt:]

	for _, size := range readingSizes {
		wide := cssDeclarations(t, ruleBody(t, wideCSS, size.opener))
		clamped := cssDeclarations(t, ruleBody(t, narrowCSS, size.opener))
		narrow := make(map[string]string, len(wide))
		maps.Copy(narrow, wide)
		maps.Copy(narrow, clamped)

		for _, layout := range []struct {
			name   string
			tokens map[string]string
		}{
			{"wide", wide},
			{"narrow (≤720px)", narrow},
		} {
			px := make([]float64, len(readingLadder))
			for i, rung := range readingLadder {
				raw, ok := layout.tokens[rung.token]
				if !ok {
					t.Fatalf("%s at %s declares no %s, so %s cannot be measured", size.name, layout.name, rung.token, rung.level)
				}
				px[i] = remPixels(t, raw)
			}
			for i := 1; i < len(readingLadder); i++ {
				prev, cur := readingLadder[i-1], readingLadder[i]
				if cur.level == "body" {
					if px[i] > px[i-1] {
						t.Errorf("%s at %s: body is %gpx, larger than %s's %gpx — the smallest heading has to stay at or above its own paragraphs", size.name, layout.name, px[i], prev.level, px[i-1])
					}
					continue
				}
				if px[i] >= px[i-1] {
					t.Errorf("%s at %s: %s is %gpx, not smaller than %s's %gpx — the heading ladder has to strictly descend", size.name, layout.name, cur.level, px[i], prev.level, px[i-1])
				}
			}
		}
	}
}

// remPixels converts an authored rem length to the pixels a reader's browser
// resolves it to. The editorial scale is authored entirely in rem and this
// stylesheet sets no root font-size of its own, so 1rem is the browser
// default of 16px.
func remPixels(t *testing.T, value string) float64 {
	t.Helper()
	trimmed, ok := strings.CutSuffix(strings.TrimSpace(value), "rem")
	if !ok {
		t.Fatalf("%q is not a rem length — the reading scale is authored in rem, and a step in another unit cannot be compared against the rest by a fixed multiplier", value)
	}
	n, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		t.Fatalf("%q does not parse as a number: %v", value, err)
	}
	return n * 16
}

// readingScaleNames returns the editorial-size names one rule body declares,
// sorted. The prefix is the whole scale and only the scale: --fs- would also
// take the chrome sizes, which the reader's preference deliberately does not
// move and which only the base block declares.
func readingScaleNames(t *testing.T, body string) []string {
	t.Helper()
	var names []string
	for property := range cssDeclarations(t, body) {
		if strings.HasPrefix(property, "--fs-ed-") {
			names = append(names, property)
		}
	}
	slices.Sort(names)
	return names
}
