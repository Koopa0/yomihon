package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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

// TestCSSCarriesTheMotionGuarantees locks, as stylesheet text, two
// guarantees only the stylesheet carries; until a screenshot pipeline can
// assert them from computed style, a textual assertion is the lock that
// can actually go red. First: the reduced-motion blanket kill must keep
// exempting the one essential state display — the reading-position
// hairline — or a reduced-motion reader loses the scroll-position
// display. Second: reduced motion must turn cross-document navigation
// transitions off entirely.
func TestCSSCarriesTheMotionGuarantees(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("css/components.css")
	if err != nil {
		t.Fatalf("read stylesheet: %v", err)
	}
	css := string(b)

	// The blanket kill is the reduced-motion rule that crushes animation and
	// transition durations; its element selector must carry the exemption.
	kill := regexp.MustCompile(`prefers-reduced-motion: reduce\) \{ ([^{]+)\{[^}]*animation-duration: 0\.001ms !important`)
	m := kill.FindStringSubmatch(css)
	if m == nil {
		t.Fatal("the reduced-motion blanket kill rule is missing from css/components.css")
	}
	if exempt := ":not(.y-readline)"; !strings.Contains(m[1], exempt) {
		t.Errorf("the blanket kill selector %q is missing the %s exemption", strings.TrimSpace(m[1]), exempt)
	}

	off := regexp.MustCompile(`@media \(prefers-reduced-motion: reduce\) \{\s*@view-transition \{\s*navigation: none;`)
	if !off.MatchString(css) {
		t.Error("no reduced-motion view-transition rule sets navigation: none")
	}
}

func TestLessonJavaScriptUsesTraditionalChineseSpeechLabels(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("js/lesson.js")
	if err != nil {
		t.Fatalf("read lesson JavaScript: %v", err)
	}
	js := string(b)
	for _, label := range []string{
		"'朗讀這段日文'",
		"'停止朗讀'",
		"'日文朗讀控制'",
	} {
		if !strings.Contains(js, label) {
			t.Errorf("speech controls do not carry Traditional Chinese label %s", label)
		}
	}
}

func TestSearchJavaScriptUsesTraditionalChineseStatusMessages(t *testing.T) {
	t.Parallel()

	b, err := Files.ReadFile("js/search.js")
	if err != nil {
		t.Fatalf("read search JavaScript: %v", err)
	}
	js := string(b)
	for _, want := range []string{
		"有 ${count} 筆結果",
		"即時結果目前無法使用；按 Enter 執行完整搜尋",
	} {
		if !strings.Contains(js, want) {
			t.Errorf("search status messages do not carry Traditional Chinese text %q", want)
		}
	}
	for _, legacy := range []string{"result' : 'results", "Live results unavailable"} {
		if strings.Contains(js, legacy) {
			t.Errorf("search JavaScript retains legacy English status copy %q", legacy)
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
		"fonts/Geist-Variable.woff2":             "c46b00cf667277d22cc237e58149520daec19542edc3f05e7daff4581dc23d2a",
		"fonts/GeistMono-Variable.woff2":         "78b4deef94de1cc4b63ba58ba86fe9e64b7f41aa8c6a7e2eb534e281834e94dd",
		"fonts/Newsreader-Italic-Variable.woff2": "d8c263970d52e0b94b3d5d4250d5962fe39f8f3b6fa9ad13b406d73ff3f4b036",
		"fonts/Newsreader-Variable.woff2":        "ac6fa9ed533278f4c8fd3ae44a1fc78c7df736040237ab86fc1160d020af0af2",
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

	notices, err := os.ReadFile("../THIRD_PARTY_NOTICES.md")
	if err != nil {
		t.Fatalf("read third-party notices: %v", err)
	}
	for _, component := range []string{"Mermaid 11.15.0", "Geist and Geist Mono 1.500", "Newsreader 1.003"} {
		if !bytes.Contains(notices, []byte(component)) {
			t.Errorf("third-party notices do not name %s", component)
		}
	}
}

// The passage's language belongs to the server, which stamps it from the
// author's read-aloud marker. The runtime reads it from there rather than
// carrying a second copy — and it looks above the button, because the button
// carries its own lang for its Chinese label and asking it would speak
// Japanese in a Chinese voice.
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
	if !strings.Contains(js, "trigger?.parentElement?.closest?.('[lang]')") {
		t.Error("speech language does not start its search above the button, so the button's own label language can win")
	}
}
