package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const validBrandSVGFixture = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path data-brand-part="cover" fill="#0F0F0F" d="M0 0Z"/><path data-brand-part="pages" fill="#F5F1E6" d="M1 1Z"/><path data-brand-part="obi" fill="#D62A0F" d="M2 2Z"/></svg>`

func TestBrandDirectoryContainsExactlyCanonicalSVG(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir("brand")
	if err != nil {
		t.Fatalf("read brand asset directory: %v", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name()+"/")
			continue
		}
		names = append(names, entry.Name())
	}
	if want := []string{"yomihon-mark.svg"}; !slices.Equal(names, want) {
		t.Errorf("brand asset files = %q, want exact canonical set %q", names, want)
	}
}

func TestBrandMarkIsEmbeddedAndUsesRestrictedSVGGrammar(t *testing.T) {
	t.Parallel()

	data, err := Files.ReadFile("brand/yomihon-mark.svg")
	if err != nil {
		t.Fatalf("read embedded brand mark: %v", err)
	}
	if validationErr := validateBrandMarkSVG(data); validationErr != nil {
		t.Errorf("brand mark violates the restricted passive SVG grammar: %v", validationErr)
	}
	disk, err := os.ReadFile("brand/yomihon-mark.svg")
	if err != nil {
		t.Fatalf("read tracked brand mark: %v", err)
	}
	if !bytes.Equal(data, disk) {
		t.Error("embedded brand mark differs from the tracked canonical SVG")
	}
}

func TestReadmeStartsWithCanonicalBrandHeading(t *testing.T) {
	t.Parallel()

	readme, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatalf("read repository README: %v", err)
	}
	const heading = `<h1><img src="assets/brand/yomihon-mark.svg" width="36" height="36" alt="" aria-hidden="true"> yomihon</h1>`
	firstLine, _, _ := strings.Cut(string(readme), "\n")
	if firstLine != heading {
		t.Errorf("README first line = %q, want rendered canonical brand heading %q", firstLine, heading)
	}
	const source = `src="assets/brand/yomihon-mark.svg"`
	if got := strings.Count(string(readme), source); got != 1 {
		t.Errorf("README canonical brand projections = %d, want one exact %q", got, source)
	}
}

func TestBrandMarkValidatorRejectsNonCanonicalSVG(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		svg  string
	}{
		{name: "script", svg: strings.Replace(validBrandSVGFixture, `</svg>`, `<script/></svg>`, 1)},
		{name: "external use", svg: strings.Replace(validBrandSVGFixture, `</svg>`, `<use href="https://example.com/mark.svg#shape"/></svg>`, 1)},
		{name: "raster image", svg: strings.Replace(validBrandSVGFixture, `</svg>`, `<image href="data:image/png;base64,AA=="/></svg>`, 1)},
		{name: "style element", svg: strings.Replace(validBrandSVGFixture, `</svg>`, `<style>path{fill:red}</style></svg>`, 1)},
		{name: "mask", svg: strings.Replace(validBrandSVGFixture, `</svg>`, `<mask id="m"/></svg>`, 1)},
		{name: "text", svg: strings.Replace(validBrandSVGFixture, `</svg>`, `<text>yomihon</text></svg>`, 1)},
		{name: "metadata", svg: strings.Replace(validBrandSVGFixture, `</svg>`, `<metadata>generator</metadata></svg>`, 1)},
		{name: "nested group", svg: strings.Replace(validBrandSVGFixture, `<path data-brand-part="cover" fill="#0F0F0F" d="M0 0Z"/>`, `<g><path data-brand-part="cover" fill="#0F0F0F" d="M0 0Z"/></g>`, 1)},
		{name: "fourth path", svg: strings.Replace(validBrandSVGFixture, `</svg>`, `<path data-brand-part="extra" fill="#0F0F0F" d="M3 3Z"/></svg>`, 1)},
		{name: "wrong part order", svg: strings.Replace(validBrandSVGFixture, `data-brand-part="cover"`, `data-brand-part="pages"`, 1)},
		{name: "wrong cover color", svg: strings.Replace(validBrandSVGFixture, `#0F0F0F`, `#101010`, 1)},
		{name: "wrong pages color", svg: strings.Replace(validBrandSVGFixture, `#F5F1E6`, `#FFFFFF`, 1)},
		{name: "wrong obi color", svg: strings.Replace(validBrandSVGFixture, `#D62A0F`, `#FF0000`, 1)},
		{name: "event handler", svg: strings.Replace(validBrandSVGFixture, `d="M0 0Z"`, `d="M0 0Z" onload="alert(1)"`, 1)},
		{name: "style attribute", svg: strings.Replace(validBrandSVGFixture, `d="M0 0Z"`, `d="M0 0Z" style="opacity:.5"`, 1)},
		{name: "href attribute", svg: strings.Replace(validBrandSVGFixture, `d="M0 0Z"`, `d="M0 0Z" href="https://example.com"`, 1)},
		{name: "attribute order", svg: strings.Replace(validBrandSVGFixture, `data-brand-part="cover" fill="#0F0F0F" d="M0 0Z"`, `fill="#0F0F0F" data-brand-part="cover" d="M0 0Z"`, 1)},
		{name: "empty path", svg: strings.Replace(validBrandSVGFixture, `d="M0 0Z"`, `d=""`, 1)},
		{name: "comment", svg: strings.Replace(validBrandSVGFixture, `</svg>`, `<!-- generated --></svg>`, 1)},
		{name: "processing instruction", svg: `<?xml version="1.0"?>` + validBrandSVGFixture},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateBrandMarkSVG([]byte(tt.svg)); err == nil {
				t.Errorf("validateBrandMarkSVG() accepted non-canonical %s content", tt.name)
			}
		})
	}
}

func TestBrandMarkValidatorAcceptsValidStructure(t *testing.T) {
	t.Parallel()

	if err := validateBrandMarkSVG([]byte(validBrandSVGFixture)); err != nil {
		t.Fatalf("validateBrandMarkSVG() rejected the canonical structure: %v", err)
	}
}

func validateBrandMarkSVG(data []byte) error {
	const namespace = "http://www.w3.org/2000/svg"
	wantPaths := []struct {
		part string
		fill string
	}{
		{part: "cover", fill: "#0F0F0F"},
		{part: "pages", fill: "#F5F1E6"},
		{part: "obi", fill: "#D62A0F"},
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	roots := 0
	paths := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("decode XML: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			depth++
			switch {
			case depth == 1 && value.Name.Space == namespace && value.Name.Local == "svg":
				roots++
				if roots != 1 {
					return fmt.Errorf("root count = %d, want 1", roots)
				}
				if err := validateBrandSVGRootAttributes(value.Attr); err != nil {
					return err
				}
			case depth == 2 && value.Name.Space == namespace && value.Name.Local == "path":
				if paths >= len(wantPaths) {
					return fmt.Errorf("path count exceeds %d", len(wantPaths))
				}
				want := wantPaths[paths]
				if err := validateBrandSVGPathAttributes(value.Attr, want.part, want.fill); err != nil {
					return fmt.Errorf("%s path: %w", want.part, err)
				}
				paths++
			default:
				return fmt.Errorf("element at depth %d = {%s}%s, want svg with three direct path children only", depth, value.Name.Space, value.Name.Local)
			}
		case xml.EndElement:
			depth--
			if depth < 0 {
				return fmt.Errorf("unexpected closing element {%s}%s", value.Name.Space, value.Name.Local)
			}
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return errors.New("character data is forbidden")
			}
		case xml.Comment:
			return errors.New("comments are forbidden")
		case xml.Directive:
			return errors.New("directives are forbidden")
		case xml.ProcInst:
			return errors.New("processing instructions are forbidden")
		default:
			return fmt.Errorf("unsupported XML token %T", token)
		}
	}
	if depth != 0 {
		return fmt.Errorf("final XML depth = %d, want 0", depth)
	}
	if roots != 1 || paths != len(wantPaths) {
		return fmt.Errorf("svg/path count = %d/%d, want 1/%d", roots, paths, len(wantPaths))
	}
	return nil
}

func validateBrandSVGRootAttributes(attrs []xml.Attr) error {
	if len(attrs) != 2 {
		return fmt.Errorf("svg attributes = %v, want exact xmlns and viewBox", attrs)
	}
	if attr := attrs[0]; attr.Name.Space != "" || attr.Name.Local != "xmlns" || attr.Value != "http://www.w3.org/2000/svg" {
		return fmt.Errorf("first svg attribute = {%s}%s=%q, want xmlns=%q", attr.Name.Space, attr.Name.Local, attr.Value, "http://www.w3.org/2000/svg")
	}
	if attr := attrs[1]; attr.Name.Space != "" || attr.Name.Local != "viewBox" || attr.Value != "0 0 32 32" {
		return fmt.Errorf("second svg attribute = {%s}%s=%q, want viewBox=%q", attr.Name.Space, attr.Name.Local, attr.Value, "0 0 32 32")
	}
	return nil
}

func validateBrandSVGPathAttributes(attrs []xml.Attr, wantPart, wantFill string) error {
	if len(attrs) != 3 {
		return fmt.Errorf("attributes = %v, want exact data-brand-part, fill, and d", attrs)
	}
	want := []struct {
		name  string
		value string
	}{
		{name: "data-brand-part", value: wantPart},
		{name: "fill", value: wantFill},
	}
	for i, expected := range want {
		attr := attrs[i]
		if attr.Name.Space != "" || attr.Name.Local != expected.name || attr.Value != expected.value {
			return fmt.Errorf("attribute %d = {%s}%s=%q, want %s=%q", i, attr.Name.Space, attr.Name.Local, attr.Value, expected.name, expected.value)
		}
	}
	d := attrs[2]
	if d.Name.Space != "" || d.Name.Local != "d" || strings.TrimSpace(d.Value) == "" {
		return fmt.Errorf("third attribute = {%s}%s=%q, want non-empty d", d.Name.Space, d.Name.Local, d.Value)
	}
	return nil
}

// TestCSSCarriesTheMotionGuarantees locks, as stylesheet text, a guarantee
// only the stylesheet carries; until a screenshot pipeline can assert it from
// computed style, a textual assertion is the lock that can actually go red:
// the reduced-motion blanket kill must keep exempting the one essential state
// display — the reading-position hairline — or a reduced-motion reader loses
// the scroll-position display. The other motion guarantee, that a page change
// is animated only for a reader who allows motion, is
// TestCSSChangesPagesOnlyForReadersWhoAllowMotion.
func TestCSSCarriesTheMotionGuarantees(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("css/components.css")
	if err != nil {
		t.Fatalf("read stylesheet: %v", err)
	}
	css := string(b)

	// The blanket kill is the reduced-motion rule that crushes animation and
	// transition durations; its element selector must carry the exemption.
	// Whitespace between tokens is not the property under test — the guide's
	// reformat may put the selector list and the opening braces on their own
	// lines — so it matches any run of whitespace, not a literal single space.
	kill := regexp.MustCompile(`(?s)prefers-reduced-motion:\s*reduce\)\s*\{\s*([^{]+)\{[^}]*animation-duration:\s*0\.001ms\s*!important`)
	m := kill.FindStringSubmatch(css)
	if m == nil {
		t.Fatal("the reduced-motion blanket kill rule is missing from css/components.css")
	}
	if exempt := ":not(.y-readline)"; !strings.Contains(m[1], exempt) {
		t.Errorf("the blanket kill selector %q is missing the %s exemption", strings.TrimSpace(m[1]), exempt)
	}
}

// atRuleBodies returns the text inside every @media block of css whose prelude
// starts with prefix. The match is by braces rather than by pattern, because
// the blocks it is asked for hold rules of their own.
func atRuleBodies(css, prefix string) []string {
	var bodies []string
	for from := 0; ; {
		at := strings.Index(css[from:], prefix)
		if at < 0 {
			return bodies
		}
		at += from
		open := strings.Index(css[at:], "{")
		if open < 0 {
			return bodies
		}
		open += at
		depth, end := 0, -1
		for i := open; i < len(css) && end < 0; i++ {
			switch css[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = i
				}
			}
		}
		if end < 0 {
			return bodies
		}
		bodies = append(bodies, css[open+1:end])
		from = end
	}
}

// TestCSSChangesPagesOnlyForReadersWhoAllowMotion locks the declaration of the
// page change. A browser decides whether a transition runs, and it can decline
// for reasons the page does not control, so what the product owns and what is
// asserted here is the stylesheet: the opt-in exists once and only for readers
// who allow motion; the only regions with a name of their own are the header
// and the left rail, which are cut rather than faded; and the page itself
// carries the motion, from the tokens.
func TestCSSChangesPagesOnlyForReadersWhoAllowMotion(t *testing.T) {
	t.Parallel()

	components, err := os.ReadFile("css/components.css")
	if err != nil {
		t.Fatalf("read stylesheet: %v", err)
	}
	css := string(components)
	tokenFile, err := os.ReadFile("css/tokens.css")
	if err != nil {
		t.Fatalf("read tokens: %v", err)
	}
	tokens := string(tokenFile)

	const gate = "@media (prefers-reduced-motion: no-preference)"
	gated := strings.Join(atRuleBodies(css, gate), "\n")
	if gated == "" {
		t.Fatalf("css/components.css has no %q block, so nothing in it can be gated on motion", gate)
	}

	// The opt-in is the one rule that makes a link animate, so it is declared
	// exactly once and the one declaration sits under the gate. A second,
	// ungated copy would animate for a reader who asked for no motion, and the
	// reduced-motion blanket does not reach the pseudo-elements it creates.
	if got := strings.Count(css, "@view-transition"); got != 1 {
		t.Errorf("css/components.css declares @view-transition %d times, want exactly 1", got)
	}
	if !regexp.MustCompile(`@view-transition\s*\{\s*navigation:\s*auto;?\s*\}`).MatchString(gated) {
		t.Errorf("css/components.css does not opt into navigation transitions under %q", gate)
	}

	// Every named region lives under the same gate and is one of the two that
	// hold still. A third name would be a region that crossfades into a double
	// exposure, and a name outside the gate would exist for a reader who gets
	// no transition at all.
	railGate := gate + " and (min-width: 901px)"
	rail := strings.Join(atRuleBodies(css, railGate), "\n")
	nameDecl := regexp.MustCompile(`view-transition-name:\s*([\w-]+)`)
	var named []string
	for _, m := range nameDecl.FindAllStringSubmatch(css, -1) {
		named = append(named, m[1])
	}
	slices.Sort(named)
	if want := []string{"y-header", "y-rail"}; !slices.Equal(named, want) {
		t.Errorf("css/components.css names the regions %v, want exactly %v", named, want)
	}
	if got := len(nameDecl.FindAllString(gated, -1)); got != len(named) {
		t.Errorf("%d of %d view-transition-name declarations sit under %q", got, len(named), gate)
	}
	if !regexp.MustCompile(`\.y-header\s*\{[^}]*view-transition-name:\s*y-header`).MatchString(gated) {
		t.Error("the header does not carry the name y-header under the motion gate")
	}
	if !regexp.MustCompile(`\.y-rail-left\s*\{[^}]*view-transition-name:\s*y-rail\b`).MatchString(rail) {
		t.Errorf("the left rail does not carry the name y-rail under %q; it is named only where it is in flow, because a drawer waits off-screen", railGate)
	}

	// A rule's selector list and its declarations, for every rule under the gate.
	ruleOf := regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`)
	bodyFor := func(selector string) (string, bool) {
		for _, m := range ruleOf.FindAllStringSubmatch(gated, -1) {
			if strings.Contains(m[1], selector) {
				return m[2], true
			}
		}
		return "", false
	}
	for _, name := range []string{"y-header", "y-rail"} {
		for part, want := range map[string]string{
			"group": "animation-duration: 0s",
			"old":   "animation: none",
			"new":   "animation: none",
		} {
			selector := "::view-transition-" + part + "(" + name + ")"
			body, ok := bodyFor(selector)
			if !ok || !strings.Contains(body, want) {
				t.Errorf("%s is not cut with %q under %q; its body is %q", selector, want, gate, body)
			}
		}
		old := "::view-transition-old(" + name + ")"
		hidden := false
		for _, m := range ruleOf.FindAllStringSubmatch(gated, -1) {
			if strings.Contains(m[1], old) && strings.Contains(m[2], "display: none") {
				hidden = true
			}
		}
		if !hidden {
			t.Errorf("%s is never hidden, so the old and new snapshots of %s would double-expose", old, name)
		}
	}

	// The page carries the motion: the old one leaves, the new one rises after
	// a gap, each on its own token.
	for selector, want := range map[string]string{
		"::view-transition-old(root)": "y-page-leave var(--dur-page-out) var(--ease-in)",
		"::view-transition-new(root)": "y-page-rise var(--dur-page-in) var(--ease-out) var(--dur-page-gap)",
	} {
		body, ok := bodyFor(selector)
		if !ok || !strings.Contains(body, want) {
			t.Errorf("%s does not run %q under %q; its body is %q", selector, want, gate, body)
		}
	}
	// Under a partly transparent old and new page the ground that shows through
	// is the page's own paper, not the document canvas, and the root group's
	// own morph is not what decides how long the change lasts.
	if body, ok := bodyFor("::view-transition-group(root)"); !ok ||
		!strings.Contains(body, "background: var(--bg)") || !strings.Contains(body, "animation-duration: 0s") {
		t.Errorf("::view-transition-group(root) does not stand on the page's paper colour with no morph of its own under %q; its body is %q", gate, body)
	}

	// The generated tree hangs off the root element, outside the container the
	// reduced-motion blanket selects, so the blanket has a rule of its own for
	// it. The opt-in is withheld from these readers already; this holds if a
	// change is ever started anyway.
	blanket := strings.Join(atRuleBodies(css, "@media (prefers-reduced-motion: reduce)"), "\n")
	generated := regexp.MustCompile(`::view-transition-group\(\*\),\s*::view-transition-old\(\*\),\s*::view-transition-new\(\*\)\s*\{[^}]*animation:\s*none\s*!important`)
	if !generated.MatchString(blanket) {
		t.Error("the reduced-motion blanket does not switch off the animations of the page-change tree (::view-transition-group/old/new(*))")
	}
	for _, keyframes := range []string{"@keyframes y-page-leave", "@keyframes y-page-rise"} {
		if !strings.Contains(css, keyframes) {
			t.Errorf("css/components.css has no %q", keyframes)
		}
	}
	for _, token := range []string{"--dur-page-out", "--dur-page-in", "--dur-page-gap", "--ease-in", "--ease-out"} {
		if !regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(token) + `:\s*\S`).MatchString(tokens) {
			t.Errorf("css/tokens.css does not define %s; the pseudo-element tree reads it from the document root", token)
		}
	}
}

func TestThirdPartyAssetProvenance(t *testing.T) {
	t.Parallel()

	fontReadme, err := Files.ReadFile("fonts/README.md")
	if err != nil {
		t.Fatalf("read font provenance: %v", err)
	}
	want := map[string]string{
		"fonts/Geist-Variable.woff2":                      "c46b00cf667277d22cc237e58149520daec19542edc3f05e7daff4581dc23d2a",
		"fonts/GeistMono-Variable.woff2":                  "78b4deef94de1cc4b63ba58ba86fe9e64b7f41aa8c6a7e2eb534e281834e94dd",
		"fonts/Newsreader-Latin-Italic-Variable.woff2":    "48bc8861b9b2ca9300747cad4fd6a3b4ac3028d364df00bd1b72097baa75e509",
		"fonts/Newsreader-Latin-Variable.woff2":           "62981321d9a3cc7a61a73792729043703fd6112da86e8ec848bb57f088578757",
		"fonts/Newsreader-LatinExt-Italic-Variable.woff2": "d8c263970d52e0b94b3d5d4250d5962fe39f8f3b6fa9ad13b406d73ff3f4b036",
		"fonts/Newsreader-LatinExt-Variable.woff2":        "ac6fa9ed533278f4c8fd3ae44a1fc78c7df736040237ab86fc1160d020af0af2",
	}
	for name, wantHash := range want {
		data, readErr := Files.ReadFile(name)
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		got := sha256.Sum256(data)
		if gotHash := hex.EncodeToString(got[:]); gotHash != wantHash {
			t.Errorf("%s SHA-256 = %s, want %s; update the font provenance with any intentional replacement", name, gotHash, wantHash)
		}
		if !bytes.Contains(fontReadme, []byte(wantHash)) {
			t.Errorf("font provenance does not record %s for %s", wantHash, name)
		}
	}

	// The tracked inventory file is the redistribution claim the notices point
	// at. The hashes above are proved against the embedded bytes, so requiring
	// the inventory to match them exactly means a drifted or hand-edited
	// SHA256SUMS line fails here rather than silently shipping.
	sums, err := Files.ReadFile("fonts/SHA256SUMS")
	if err != nil {
		t.Fatalf("read font hash inventory: %v", err)
	}
	inventory := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSuffix(string(sums), "\n"), "\n") {
		hash, file, ok := strings.Cut(line, "  ")
		if !ok || len(hash) != 64 {
			t.Fatalf("font hash inventory line %q is not \"<sha256>  <file>\"", line)
		}
		inventory["fonts/"+file] = hash
	}
	for name, wantHash := range want {
		gotHash, ok := inventory[name]
		if !ok {
			t.Errorf("font hash inventory does not list %s", name)
			continue
		}
		if gotHash != wantHash {
			t.Errorf("font hash inventory records %s for %s, want %s", gotHash, name, wantHash)
		}
	}
	for name := range inventory {
		if _, ok := want[name]; !ok {
			t.Errorf("font hash inventory lists %s, which is not a verified font", name)
		}
	}

	for _, name := range []string{"fonts/LICENSE.txt", "js/mermaid/LICENSE"} {
		data, readErr := Files.ReadFile(name)
		if readErr != nil {
			t.Fatalf("read notice %s: %v", name, readErr)
		}
		if len(data) == 0 {
			t.Errorf("notice %s is empty", name)
		}
	}

	// The licence obligation is met by the embedded texts above, which ship
	// inside the binary. The summary notice is kept on the maintainer's machine
	// with the rest of the governance prose rather than in history, so a clean
	// clone has none to read: it is checked where present and skipped where
	// not, as a subtest so the embedded checks above keep reporting their own
	// result.
	t.Run("summary notice names each component", func(t *testing.T) {
		t.Parallel()

		notices, err := os.ReadFile("../THIRD_PARTY_NOTICES.md")
		if errors.Is(err, fs.ErrNotExist) {
			t.Skip("THIRD_PARTY_NOTICES.md is not in this checkout; it is kept on the maintainer's machine")
		}
		if err != nil {
			t.Fatalf("read third-party notices: %v", err)
		}
		for _, component := range []string{"Mermaid 11.15.0", "Geist and Geist Mono 1.500", "Newsreader 1.003"} {
			if !bytes.Contains(notices, []byte(component)) {
				t.Errorf("third-party notices do not name %s", component)
			}
		}
	})
}

// passageIsTheTrigger matches a passage that is the button itself, whichever
// way it is written. The property below is about where the language is read
// from, not about which expression reaches the element, so the button is named
// and the way up out of it is left free to change.
var passageIsTheTrigger = regexp.MustCompile(`passage\s*=\s*trigger\s*[,)]`)

// The passage's language belongs to the server, which stamps it from the
// author's read-aloud marker. The runtime reads it from there rather than
// carrying a second copy, and never from the button: a button carries its own
// lang for its Chinese label, so asking the button would speak Japanese in a
// Chinese voice. A paragraph's passage is what encloses its button; a speaker
// that sits beside its sentence instead hands that sentence over, which is why
// the language is resolved from a passage and not from the trigger.
//
// Reading a note through makes one more thing breakable. A note may carry
// paragraphs in more than one language, so the voice has to be decided again
// for each of them; decided once for the reading, the second paragraph would be
// spoken in the first one's voice. That is the single-resolution-site check.
func TestSpeechLanguageComesFromTheMarkedPassage(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("js/lesson.js")
	if err != nil {
		t.Fatalf("read lesson JavaScript: %v", err)
	}
	js := string(b)
	if strings.Contains(js, "utterance.lang = 'ja-JP'") {
		t.Error("speech language is hardcoded at the utterance rather than read from the passage")
	}
	// Named positively as well, because the three checks below all pass for an
	// utterance whose language is decided somewhere else entirely.
	if !strings.Contains(js, "utterance.lang = speechLanguage(passage)") {
		t.Error("the utterance's language does not come from the passage the server marked")
	}
	if passageIsTheTrigger.MatchString(js) {
		t.Error("a paragraph's passage is the button itself, so the button's own label language can win")
	}
	if strings.Contains(js, "speechLanguage(trigger)") {
		t.Error("speech language is resolved from the button, whose lang belongs to its label rather than to the words it speaks")
	}
	// Two: the function and the one call inside the utterance's own
	// construction. A third is a second place deciding the voice — and the one
	// a reading through invites is a language resolved once above the walk and
	// handed to every paragraph in it.
	if got := strings.Count(js, "speechLanguage("); got != 2 {
		t.Errorf("the runtime names speechLanguage( %d times, want 2: the function and the one call that makes an utterance, so a note read through resolves the voice once per paragraph", got)
	}
}
