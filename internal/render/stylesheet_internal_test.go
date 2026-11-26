package render

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/assets"
)

// The renderer decides, as it writes a block, which reading measures the block
// is too wide for, and it counts that from numbers the stylesheet owns: the
// measures, the code face's size and padding, the table's sizes, tracking and
// gap, a code span's size and padding, and the mat's inset. The tests below
// hold each Go copy to the declaration it copies, in the stylesheets the page
// is served (the embedded ones), so a change on either side fails a plain go
// test with both sides named, rather than setting blocks at the wrong width.
//
// They hold the inputs, in the rules that set them, not the layout: a later
// rule that overrides one, or a count that disagrees with what a browser draws,
// is the browser probe wide-blocks.mjs's to catch. The faces' own advances
// (Geist's, Geist Mono's three fifths, the semibold's extra width) belong to the
// font files rather than the sheets, so what is held for them is that the faces
// they were measured from are still the ones the sheets name first. A rem is
// counted at the browser's 16px, since no sheet sets a root size of its own.

// textSizeRoots are, for each text size, the rule tokens.css sets that size's
// tokens in (rule) and the root the wide rule in components.css names it by
// (wide). An
// attribute root declares only what it changes and takes the rest from the
// base root, as the cascade does.
var textSizeRoots = [...]struct {
	name string
	rule string
	wide string
}{
	textM:  {name: "m", rule: ":root,\n.y-papersample[data-paper='stone']", wide: ":root:not([data-textsize='l'], [data-textsize='xl'])"},
	textL:  {name: "l", rule: ":root[data-textsize='l']", wide: ":root[data-textsize='l']"},
	textXL: {name: "xl", rule: ":root[data-textsize='xl']", wide: ":root[data-textsize='xl']"},
}

const (
	// latinArticle is the rule components.css gives a Latin article its
	// measure in.
	latinArticle = ".y-article[lang]:not(:lang(zh)):not(:lang(ja)):not(:lang(ko))"
	// wideLatin and wideFull are the two articles the wide rule tells apart,
	// spelled as it spells them: a Latin one, and one that is Chinese,
	// Japanese or Korean or declares no language.
	wideLatin = ".y-main > .y-article[lang]:not(:lang(zh), :lang(ja), :lang(ko)) > .y-prose > "
	wideFull  = ".y-main > .y-article:not([lang]:not(:lang(zh), :lang(ja), :lang(ko))) > .y-prose > "
)

var (
	cssComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	cssVar     = regexp.MustCompile(`^var\((--[a-z0-9-]+)\)$`)
	// latinCalc is the Latin measure as components.css writes it: a step of
	// the editorial scale times a number of them.
	latinCalc          = regexp.MustCompile(`^calc\(var\((--fs-ed-\d+)\) \* (\d+)\)$`)
	wideToken          = regexp.MustCompile(`^\[data-wide~='([a-z-]+)'\]$`)
	tabSizeDeclaration = regexp.MustCompile(`(?:^|[;{])\s*tab-size\s*:([^;}]*)`)
)

// TestTheReadingMeasuresAreTheStylesheets holds readingMeasures to the sheets.
// Every token in it is one the wide rule sets wide, at the text size the table
// reads it at, and its width is that size's --measure-read for the article the
// rule names: tokens.css's for a Chinese, Japanese or Korean article or one
// that declares no language, and components.css's count of reading ems for a
// Latin one. Every token the rule sets wide is in the table, so a measure the
// sheet adds is one the renderer has to count.
func TestTheReadingMeasuresAreTheStylesheets(t *testing.T) {
	t.Parallel()
	sheets := embeddedStylesheets(t)

	latin := sheets.components.rule(t, latinArticle)
	latinMeasure := latin.value(t, "--measure-read")
	parts := latinCalc.FindStringSubmatch(latinMeasure)
	if parts == nil {
		t.Fatalf("%s --measure-read = %q, want calc(var(--fs-ed-N) * N): readingMeasures (widemeasure.go) counts the Latin measure as a number of reading ems, and a measure written otherwise is one this test has to be taught to read", latin.where, latinMeasure)
	}
	readingEms, err := strconv.Atoi(parts[2])
	if err != nil {
		t.Fatalf("%s --measure-read = %q: %v", latin.where, latinMeasure, err)
	}

	type cssMeasure struct {
		size   textSize
		width  float64
		source string
	}
	want := map[string]cssMeasure{}
	for _, selector := range sheets.wideSelectors(t) {
		token, size, isLatin := parseWideSelector(t, selector)
		if _, twice := want[token]; twice {
			t.Fatalf("components.css sets [data-wide~=%q] wide twice in the wide rule", token)
		}
		if isLatin {
			step, trail := sheets.sizedPixels(t, parts[1], size)
			want[token] = cssMeasure{size: size, width: float64(readingEms) * step, source: fmt.Sprintf("%s --measure-read is %s and %s, so %gpx", latin.where, latinMeasure, trail, float64(readingEms)*step)}
			continue
		}
		width, trail := sheets.sizedPixels(t, "--measure-read", size)
		want[token] = cssMeasure{size: size, width: width, source: trail}
	}

	named := map[string]bool{}
	for _, m := range readingMeasures {
		named[m.token] = true
		css, ok := want[m.token]
		if !ok {
			t.Errorf("readingMeasures (widemeasure.go) names %q, which no selector of the wide rule in components.css sets wide, so a block that names it is never widened", m.token)
			continue
		}
		if css.size != m.size {
			t.Errorf("readingMeasures (widemeasure.go) reads %q at text size %s, but components.css sets it wide at %s", m.token, textSizeRoots[m.size].name, textSizeRoots[css.size].name)
		}
		if float64(m.width) != css.width {
			t.Errorf("readingMeasures (widemeasure.go) gives %q a width of %dpx, but %s", m.token, m.width, css.source)
		}
	}
	for token := range want {
		if !named[token] {
			t.Errorf("components.css sets [data-wide~=%q] wide, and readingMeasures (widemeasure.go) does not name it, so no block ever carries it", token)
		}
	}
}

// TestTheCodeBlockCountIsTheStylesheets holds what codeFits counts a line
// against to the code block's own rules: the face it is set in, its size at
// each text size, the padding either side the measure loses, the code element
// inside it adding neither a size nor padding of its own, and the tab stop.
func TestTheCodeBlockCountIsTheStylesheets(t *testing.T) {
	t.Parallel()
	sheets := embeddedStylesheets(t)
	pre := sheets.components.rule(t, ".y-prose pre")

	sheets.firstFace(t, pre, "Geist Mono", "codeNarrowAdvance (codefigure.go)")

	token := varName(t, pre, "font-size")
	for size, root := range textSizeRoots {
		px, trail := sheets.sizedPixels(t, token, textSize(size))
		if float64(codeSize[size]) != px {
			t.Errorf("codeSize[%s] = %dpx (codefigure.go), but %s font-size is var(%s) and %s", root.name, codeSize[size], pre.where, token, trail)
		}
	}

	start, end := inlinePadding(t, pre)
	padding := pixels(t, pre.where+" padding", start) + pixels(t, pre.where+" padding", end)
	if float64(codePadding) != padding {
		t.Errorf("codePadding = %dpx (codefigure.go), the room a measure loses to the box, but %s padding is %s and %s either side, so %gpx", codePadding, pre.where, start, end, padding)
	}

	inner := sheets.components.rule(t, ".y-prose pre code")
	if got := inner.value(t, "font-size"); got != "inherit" {
		t.Errorf("%s font-size = %q, want inherit: codeSize (codefigure.go) counts a line at the block's own size", inner.where, got)
	}
	if start, end := inlinePadding(t, inner); !zeroLength(start) || !zeroLength(end) {
		t.Errorf("%s padding is %s and %s either side, want 0: codeFits (codefigure.go) counts a line from the box's own padding alone", inner.where, start, end)
	}

	stop := sheets.tabSize(t)
	if want := stop * codeNarrowAdvance; codeTabStop != want {
		t.Errorf("codeTabStop = %d fifths of an em (codefigure.go), but the stylesheets set tab-size: %d, which is %d of Geist Mono's %d-fifth characters, so %d", codeTabStop, stop, stop, codeNarrowAdvance, want)
	}
}

// TestTheTableCountIsTheStylesheets holds what tableWide counts a table's
// columns with to the table's rules: the body's and the header's sizes at each
// text size, the header's tracking and weight, the faces, the space after every
// column but the last and none before the first, and a code span's size and
// padding.
func TestTheTableCountIsTheStylesheets(t *testing.T) {
	t.Parallel()
	sheets := embeddedStylesheets(t)
	table := sheets.components.rule(t, ".y-prose table")
	th := sheets.components.rule(t, ".y-prose th")
	td := sheets.components.rule(t, ".y-prose td")

	for _, sized := range []struct {
		goSide string
		sizes  [3]float64
		rule   cssRule
	}{
		{goSide: "tableBodySize", sizes: tableBodySize, rule: table},
		{goSide: "tableHeaderSize", sizes: tableHeaderSize, rule: th},
	} {
		token := varName(t, sized.rule, "font-size")
		for size, root := range textSizeRoots {
			px, trail := sheets.sizedPixels(t, token, textSize(size))
			if sized.sizes[size] != px {
				t.Errorf("%s[%s] = %gpx (tablewidth.go), but %s font-size is var(%s) and %s", sized.goSide, root.name, sized.sizes[size], sized.rule.where, token, trail)
			}
		}
	}
	if got := td.value(t, "font-size"); got != "inherit" {
		t.Errorf("%s font-size = %q, want inherit: tableBodySize (tablewidth.go) counts a cell at the size %s sets", td.where, got, table.where)
	}
	if got := td.value(t, "font-variant-numeric"); got != "tabular-nums" {
		t.Errorf("%s font-variant-numeric = %q, want tabular-nums: geistAdvance (tablewidth.go) counts a cell's figures tabular", td.where, got)
	}

	if got := ems(t, th.where+" letter-spacing", th.value(t, "letter-spacing")); got != tableHeaderTracking {
		t.Errorf("tableHeaderTracking = %gem (tablewidth.go), but %s letter-spacing is %gem", tableHeaderTracking, th.where, got)
	}
	weight := varName(t, th, "font-weight")
	base := sheets.tokens.rule(t, textSizeRoots[textM].rule)
	if got := base.value(t, weight); got != "600" {
		t.Errorf("%s font-weight is var(%s), which %s sets to %s, but tableHeaderWeight (tablewidth.go) is how much wider Geist's 600 draws than its 400", th.where, weight, base.where, got)
	}

	for _, cell := range []cssRule{th, td} {
		sheets.firstFace(t, cell, "Geist", "sansAdvance (tablewidth.go)")
		start, end := inlinePadding(t, cell)
		if got := pixels(t, cell.where+" padding", start); got != 0 {
			t.Errorf("%s padding starts a cell %gpx in, but tableWide (tablewidth.go) counts no space before a column", cell.where, got)
		}
		if got := pixels(t, cell.where+" padding", end); got != tableColumnGap {
			t.Errorf("tableColumnGap = %dpx (tablewidth.go), but %s padding ends a cell %gpx after its text", tableColumnGap, cell.where, got)
		}
	}
	last := sheets.components.rule(t, ".y-prose :is(th, td):last-child")
	if got := pixels(t, last.where+" padding-inline-end", last.value(t, "padding-inline-end")); got != 0 {
		t.Errorf("%s padding-inline-end = %gpx, want 0: tableWide (tablewidth.go) counts tableColumnGap after every column but the last", last.where, got)
	}

	code := sheets.components.rule(t, ".y-prose code")
	sheets.firstFace(t, code, "Geist Mono", "tableCodeAdvance (tablewidth.go)")
	size := ems(t, code.where+" font-size", code.value(t, "font-size"))
	if tableCodeSize != size {
		t.Errorf("tableCodeSize = %gem (tablewidth.go), but %s font-size is %gem", tableCodeSize, code.where, size)
	}
	if want := codeNarrowAdvance / 5.0 * size; !closeTo(tableCodeAdvance, want) {
		t.Errorf("tableCodeAdvance = %gem (tablewidth.go), but Geist Mono's %d fifths at %s font-size %gem are %.6gem", tableCodeAdvance, codeNarrowAdvance, code.where, size, want)
	}
	start, end := inlinePadding(t, code)
	if want := (ems(t, code.where+" padding", start) + ems(t, code.where+" padding", end)) * size; !closeTo(tableCodePadding, want) {
		t.Errorf("tableCodePadding = %gem of the cell (tablewidth.go), but %s padding is %s and %s either side of a span set at %gem, so %.6gem", tableCodePadding, code.where, start, end, size, want)
	}
}

// TestThePlateInsetIsTheStylesheets holds plateInset, which a picture's room at
// each measure is counted without, to the mat's padding either side.
func TestThePlateInsetIsTheStylesheets(t *testing.T) {
	t.Parallel()
	sheets := embeddedStylesheets(t)
	mat := sheets.components.rule(t, ".y-prose .mat")
	start, end := inlinePadding(t, mat)
	side := func(value string) float64 {
		if name := cssVar.FindStringSubmatch(value); name != nil {
			value = mat.value(t, name[1])
		}
		return pixels(t, mat.where+" padding", value)
	}
	if want := side(start) + side(end); float64(plateInset) != want {
		t.Errorf("plateInset = %dpx (plate.go), but %s padding is %s and %s either side, so %gpx", plateInset, mat.where, start, end, want)
	}
}

// stylesheets are the embedded sheets the page is served, each with its
// comments removed, so a declaration or a brace in a comment is never read.
type stylesheets struct {
	tokens, components cssSheet
	all                []cssSheet
}

type cssSheet struct {
	name string
	css  string
}

func embeddedStylesheets(t *testing.T) *stylesheets {
	t.Helper()
	entries, err := assets.Files.ReadDir("css")
	if err != nil {
		t.Fatalf("assets.Files.ReadDir(%q) error = %v", "css", err)
	}
	s := &stylesheets{}
	for _, entry := range entries {
		source, err := assets.Files.ReadFile("css/" + entry.Name())
		if err != nil {
			t.Fatalf("assets.Files.ReadFile(%q) error = %v", "css/"+entry.Name(), err)
		}
		sheet := cssSheet{name: entry.Name(), css: cssComment.ReplaceAllString(string(source), "")}
		s.all = append(s.all, sheet)
		switch sheet.name {
		case "tokens.css":
			s.tokens = sheet
		case "components.css":
			s.components = sheet
		}
	}
	if s.tokens.css == "" || s.components.css == "" {
		t.Fatalf("the embedded stylesheets hold no tokens.css or no components.css, so there is nothing to hold the renderer to")
	}
	return s
}

// cssRule is one rule's body, and the sheet and selector it was found by.
type cssRule struct {
	where string
	body  string
}

// rule is the one rule at the top level of the sheet, outside any at-rule,
// whose whole selector is selector: not the end of a longer selector, nor one
// entry of a list.
func (s cssSheet) rule(t *testing.T, selector string) cssRule {
	t.Helper()
	opener := selector + " {"
	var bodies []string
	for at := 0; ; {
		i := strings.Index(s.css[at:], opener)
		if i < 0 {
			break
		}
		i += at
		at = i + len(opener)
		before := s.css[:i]
		if strings.Count(before, "{") != strings.Count(before, "}") {
			continue
		}
		if boundary := strings.LastIndexAny(before, "};"); strings.TrimSpace(before[boundary+1:]) != "" {
			continue
		}
		body, _, closed := strings.Cut(s.css[at:], "}")
		if !closed {
			t.Fatalf("%s %q is never closed", s.name, selector)
		}
		bodies = append(bodies, body)
	}
	if len(bodies) != 1 {
		t.Fatalf("%s has %d top-level rules %q, want exactly 1: the renderer copies its numbers from that rule", s.name, len(bodies), selector)
	}
	if strings.Contains(bodies[0], "{") {
		t.Fatalf("%s %q nests another rule, which this test does not read", s.name, selector)
	}
	return cssRule{where: fmt.Sprintf("%s %q", s.name, selector), body: bodies[0]}
}

// lookup is the value the rule declares property as, with its spaces folded,
// and whether it declares it at all. A property declared twice in one rule is
// refused rather than resolved.
func (r cssRule) lookup(t *testing.T, property string) (string, bool) {
	t.Helper()
	pattern := regexp.MustCompile(`(?:^|;)\s*` + regexp.QuoteMeta(property) + `\s*:([^;]*)`)
	matches := pattern.FindAllStringSubmatch(r.body, -1)
	switch len(matches) {
	case 0:
		return "", false
	case 1:
		return strings.Join(strings.Fields(matches[0][1]), " "), true
	default:
		t.Fatalf("%s declares %s %d times, so which one the browser draws is not this test's to guess", r.where, property, len(matches))
		return "", false
	}
}

func (r cssRule) value(t *testing.T, property string) string {
	t.Helper()
	value, ok := r.lookup(t, property)
	if !ok {
		t.Fatalf("%s declares no %s, which the renderer copies from it", r.where, property)
	}
	return value
}

// inlinePadding is the padding the rule's shorthand sets at the start and the
// end of a line, in a left-to-right line. A longhand beside it would change a
// side of it, and is refused rather than folded in.
func inlinePadding(t *testing.T, r cssRule) (start, end string) {
	t.Helper()
	for _, longhand := range []string{"padding-left", "padding-right", "padding-inline", "padding-inline-start", "padding-inline-end"} {
		if _, ok := r.lookup(t, longhand); ok {
			t.Fatalf("%s sets %s beside its padding, and this test reads the shorthand alone", r.where, longhand)
		}
	}
	values := strings.Fields(r.value(t, "padding"))
	switch len(values) {
	case 1:
		return values[0], values[0]
	case 2, 3:
		return values[1], values[1]
	case 4:
		return values[3], values[1]
	default:
		t.Fatalf("%s padding = %q is not a padding shorthand", r.where, strings.Join(values, " "))
		return "", ""
	}
}

// sizedPixels is what token comes to at a text size, in pixels, and the way it
// got there, for a message.
func (s *stylesheets) sizedPixels(t *testing.T, token string, size textSize) (px float64, trail string) {
	t.Helper()
	root := s.tokens.rule(t, textSizeRoots[size].rule)
	value, ok := root.lookup(t, token)
	if !ok && size != textM {
		root = s.tokens.rule(t, textSizeRoots[textM].rule)
		value, ok = root.lookup(t, token)
	}
	if !ok {
		t.Fatalf("tokens.css declares no %s at text size %s", token, textSizeRoots[size].name)
	}
	px = pixels(t, root.where+" "+token, value)
	if strings.HasSuffix(value, "rem") {
		value += fmt.Sprintf(" (%gpx)", px)
	}
	return px, fmt.Sprintf("%s sets %s to %s", root.where, token, value)
}

// wideSelectors are the selectors of the rule that sets a block at the wide
// measure, one a line. Every [data-wide~=…] in components.css has to be in that
// one rule, or a token set wide somewhere else is one nothing here ties to a
// measure.
func (s *stylesheets) wideSelectors(t *testing.T) []string {
	t.Helper()
	const attribute = "[data-wide~="
	css := s.components.css
	var rules []string
	for at, last := 0, -1; ; {
		i := strings.Index(css[at:], attribute)
		if i < 0 {
			break
		}
		i += at
		open := strings.IndexByte(css[i:], '{')
		if open < 0 {
			t.Fatalf("components.css names %s…] in a selector that never opens a rule", attribute)
		}
		at = i + open
		if start := strings.LastIndexAny(css[:i], "{};") + 1; start != last {
			last = start
			rules = append(rules, strings.TrimSpace(css[start:at]))
		}
	}
	if len(rules) != 1 {
		t.Fatalf("components.css names %s…] in %d rules, want exactly 1, the wide rule: a token set wide anywhere else is one this test cannot tie to a measure:\n%s", attribute, len(rules), strings.Join(rules, "\n\n"))
	}
	return strings.Split(rules[0], ",\n")
}

// parseWideSelector reads one selector of the wide rule: the token it sets
// wide, at which text size, and whether in a Latin article.
func parseWideSelector(t *testing.T, selector string) (token string, size textSize, latin bool) {
	t.Helper()
	selector = strings.TrimSpace(selector)
	rest, found := "", false
	for i, root := range textSizeRoots {
		if after, ok := strings.CutPrefix(selector, root.wide+" "); ok {
			rest, size, found = after, textSize(i), true
		}
	}
	if !found {
		t.Fatalf("the wide rule's selector %q starts at none of the three text sizes' roots", selector)
	}
	switch {
	case strings.HasPrefix(rest, wideLatin):
		rest, latin = strings.TrimPrefix(rest, wideLatin), true
	case strings.HasPrefix(rest, wideFull):
		rest = strings.TrimPrefix(rest, wideFull)
	default:
		t.Fatalf("the wide rule's selector %q names neither a Latin article (%q) nor any other (%q)", selector, wideLatin, wideFull)
	}
	match := wideToken.FindStringSubmatch(rest)
	if match == nil {
		t.Fatalf("the wide rule's selector %q does not end in one [data-wide~='…']", selector)
	}
	return match[1], size, latin
}

// firstFace holds that rule sets its text in a family whose first face is the
// one the renderer's advances were measured from.
func (s *stylesheets) firstFace(t *testing.T, r cssRule, face, goSide string) {
	t.Helper()
	token := varName(t, r, "font-family")
	base := s.tokens.rule(t, textSizeRoots[textM].rule)
	first, _, _ := strings.Cut(base.value(t, token), ",")
	if got := strings.Trim(strings.TrimSpace(first), `'"`); got != face {
		t.Errorf("%s font-family is var(%s), whose first face in %s is %q, but %s counts %q's advances", r.where, token, base.where, got, goSide, face)
	}
}

// tabSize is the one tab-size the sheets declare, wherever it is written: a
// second one, on the code block or anywhere above it, would move where a tab
// in code runs to.
func (s *stylesheets) tabSize(t *testing.T) int {
	t.Helper()
	var declared []string
	value := ""
	for _, sheet := range s.all {
		for _, match := range tabSizeDeclaration.FindAllStringSubmatch(sheet.css, -1) {
			value = strings.TrimSpace(match[1])
			declared = append(declared, sheet.name+": "+value)
		}
	}
	if len(declared) != 1 {
		t.Fatalf("the stylesheets declare tab-size %d times %q, want exactly 1: codeTabStop (codefigure.go) counts one tab stop for every code block", len(declared), declared)
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("tab-size is %q, want a number of characters: %v", declared[0], err)
	}
	return n
}

// varName is the custom property a declaration takes its value from.
func varName(t *testing.T, r cssRule, property string) string {
	t.Helper()
	value := r.value(t, property)
	match := cssVar.FindStringSubmatch(value)
	if match == nil {
		t.Fatalf("%s %s = %q, want var(--…): the renderer copies the token it names", r.where, property, value)
	}
	return match[1]
}

// pixels is a px length, or a rem one at the browser's 16px.
func pixels(t *testing.T, where, value string) float64 {
	t.Helper()
	if value == "0" {
		return 0
	}
	if n, ok := strings.CutSuffix(value, "rem"); ok {
		return 16 * cssNumber(t, where, value, n)
	}
	if n, ok := strings.CutSuffix(value, "px"); ok {
		return cssNumber(t, where, value, n)
	}
	t.Fatalf("%s = %q, want a px or rem length", where, value)
	return 0
}

// ems is an em length.
func ems(t *testing.T, where, value string) float64 {
	t.Helper()
	if value == "0" {
		return 0
	}
	n, ok := strings.CutSuffix(value, "em")
	if !ok || strings.HasSuffix(value, "rem") {
		t.Fatalf("%s = %q, want an em length", where, value)
	}
	return cssNumber(t, where, value, n)
}

func cssNumber(t *testing.T, where, value, number string) float64 {
	t.Helper()
	n, err := strconv.ParseFloat(number, 64)
	if err != nil {
		t.Fatalf("%s = %q: %v", where, value, err)
	}
	return n
}

// zeroLength reports whether a length is nothing, in whatever unit it is
// written.
func zeroLength(value string) bool {
	n, err := strconv.ParseFloat(strings.TrimRight(value, "abcdefghijklmnopqrstuvwxyz%"), 64)
	return err == nil && n == 0
}

func closeTo(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
