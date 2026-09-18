package layouts

import (
	"os"
	"slices"
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
	largeRoot      = `:root[data-textsize="l"] {`
	extraLargeRoot = `:root[data-textsize="xl"] {`
	narrowQuery    = "@media (max-width: 720px)"
)

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

	sizes := []struct {
		name   string
		opener string
	}{
		{"the base size", baseRoot},
		{"大", largeRoot},
		{"特大", extraLargeRoot},
	}
	declared := map[string][]string{}
	for _, size := range sizes {
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

	for _, size := range sizes[1:] {
		if diff := cmp.Diff(declared[baseRoot], declared[size.opener]); diff != "" {
			t.Errorf("the editorial scale at %s is not the base scale (-base +%s):\n%s\na step the base declares alone stays at its smallest value while the body grows past it", size.name, size.name, diff)
		}
	}

	narrow := css[narrowAt:]
	// A media query carries no specificity of its own, so a rule inside this one
	// keyed on :root alone is one attribute weaker than the two blocks above and
	// would clamp nothing for the readers whose titles are largest. The rule has
	// to name all three roots to tie with them and win on order.
	open := strings.Index(narrow, "{")
	if open < 0 {
		t.Fatalf("the %q block is not opened", narrowQuery)
	}
	rule := narrow[open+1:]
	end := strings.Index(rule, "{")
	if end < 0 {
		t.Fatalf("the %q block holds no rule", narrowQuery)
	}
	var selectors []string
	for selector := range strings.SplitSeq(rule[:end], ",") {
		if trimmed := strings.TrimSpace(selector); trimmed != "" {
			selectors = append(selectors, trimmed)
		}
	}
	for _, want := range []string{":root", `:root[data-textsize="l"]`, `:root[data-textsize="xl"]`} {
		if !slices.Contains(selectors, want) {
			t.Errorf("the narrow-width rule names %v and not %s, so the readers who chose that size keep the wide title in a phone's column", selectors, want)
		}
	}

	narrowed := readingScaleNames(t, rule[end+1:strings.Index(rule, "}")])
	if len(narrowed) == 0 {
		t.Error("the narrow-width rule narrows no editorial size, so nothing about it can be checked")
	}
	for _, name := range narrowed {
		if !slices.Contains(readingScaleSteps, name) {
			t.Errorf("the narrow-width rule declares %s, which is not a step of the editorial scale — a name only this rule knows is one the three size blocks above cannot be compared on", name)
		}
	}
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
