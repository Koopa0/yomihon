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
	"strings"
	"testing"
)

func TestBrandMarkBytesAreCanonical(t *testing.T) {
	t.Parallel()

	data, err := Files.ReadFile("brand/yomihon-mark.svg")
	if err != nil {
		t.Fatalf("read brand mark: %v", err)
	}
	got := sha256.Sum256(data)
	const want = "4580605b5d69ce8475c1c69103844ffb74b7ce95a1a35b695a6c0f620aa0b6b2"
	if gotHash := hex.EncodeToString(got[:]); gotHash != want {
		t.Errorf("brand mark SHA-256 = %s, want %s", gotHash, want)
	}
}

func TestBrandMarkUsesPassiveSVGGrammar(t *testing.T) {
	t.Parallel()

	data, err := Files.ReadFile("brand/yomihon-mark.svg")
	if err != nil {
		t.Fatalf("read brand mark: %v", err)
	}
	if err := validateBrandMarkSVG(data); err != nil {
		t.Errorf("brand mark violates the passive SVG grammar: %v", err)
	}
}

func TestBrandMarkREADMEProjectionIsCanonical(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatalf("read repository README: %v", err)
	}
	readme := string(data)
	if firstLine, _, _ := strings.Cut(readme, "\n"); firstLine != "# yomihon" {
		t.Errorf("README first line = %q, want exact product heading %q", firstLine, "# yomihon")
	}
	const projection = `<img src="assets/brand/yomihon-mark.svg" width="32" height="32" alt="" aria-hidden="true">`
	if got := strings.Count(readme, projection); got != 1 {
		t.Errorf("README canonical decorative mark projections = %d, want 1 exact %q", got, projection)
	}
}

func TestBrandMarkValidatorRejectsForbiddenContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		svg  string
	}{
		{name: "script", svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path d="M0 0Z"/><script/></svg>`},
		{name: "external use", svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path d="M0 0Z"/><use href="https://example.com/mark.svg#shape"/></svg>`},
		{name: "raster image", svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path d="M0 0Z"/><image href="data:image/png;base64,AA=="/></svg>`},
		{name: "gradient", svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><linearGradient id="g"/><path d="M0 0Z"/></svg>`},
		{name: "filter", svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><filter id="f"/><path d="M0 0Z"/></svg>`},
		{name: "text", svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><text>y</text><path d="M0 0Z"/></svg>`},
		{name: "event handler", svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path d="M0 0Z" onload="alert(1)"/></svg>`},
		{name: "style", svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path d="M0 0Z" style="fill:url(https://example.com/x)"/></svg>`},
		{name: "generator metadata", svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><metadata>generator</metadata><path d="M0 0Z"/></svg>`},
		{name: "comment", svg: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><!-- generator --><path d="M0 0Z"/></svg>`},
		{name: "processing instruction", svg: `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path d="M0 0Z"/></svg>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := validateBrandMarkSVG([]byte(tt.svg)); err == nil {
				t.Errorf("validateBrandMarkSVG() accepted forbidden %s content", tt.name)
			}
		})
	}
}

func validateBrandMarkSVG(data []byte) error {
	const namespace = "http://www.w3.org/2000/svg"

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
				paths++
				if paths != 1 {
					return fmt.Errorf("path count = %d, want 1", paths)
				}
				if len(value.Attr) != 1 || value.Attr[0].Name.Space != "" || value.Attr[0].Name.Local != "d" || strings.TrimSpace(value.Attr[0].Value) == "" {
					return fmt.Errorf("path attributes = %v, want one non-empty d attribute", value.Attr)
				}
			default:
				return fmt.Errorf("element at depth %d = {%s}%s, want svg > path only", depth, value.Name.Space, value.Name.Local)
			}
		case xml.EndElement:
			depth--
			if depth < 0 {
				return fmt.Errorf("unexpected closing element {%s}%s", value.Name.Space, value.Name.Local)
			}
		case xml.CharData:
			if strings.TrimSpace(string(value)) != "" {
				return errors.New("non-whitespace character data is forbidden")
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
	if roots != 1 || paths != 1 {
		return fmt.Errorf("svg/path count = %d/%d, want 1/1", roots, paths)
	}
	return nil
}

func validateBrandSVGRootAttributes(attrs []xml.Attr) error {
	if len(attrs) != 2 {
		return fmt.Errorf("svg attribute count = %d, want xmlns and viewBox", len(attrs))
	}
	seenNamespace := false
	seenViewBox := false
	for _, attr := range attrs {
		switch {
		case attr.Name.Space == "" && attr.Name.Local == "xmlns" && attr.Value == "http://www.w3.org/2000/svg":
			seenNamespace = true
		case attr.Name.Space == "" && attr.Name.Local == "viewBox" && attr.Value == "0 0 32 32":
			seenViewBox = true
		default:
			return fmt.Errorf("svg attribute {%s}%s=%q is forbidden", attr.Name.Space, attr.Name.Local, attr.Value)
		}
	}
	if !seenNamespace || !seenViewBox {
		return errors.New("svg attributes missing exact xmlns or viewBox")
	}
	return nil
}

// TestCSSCarriesTheMotionGuarantees locks, as stylesheet text, two
// guarantees only the stylesheet carries; until a screenshot pipeline can
// assert them from computed style, a textual assertion is the lock that
// can actually go red. First: the reduced-motion blanket kill must keep
// exempting both essential state displays — the seal hold-fill and the
// reading-position hairline — or a reduced-motion reader loses the
// press-and-hold progress and the scroll-position display. Second:
// reduced motion must turn cross-document navigation transitions off
// entirely.
func TestCSSCarriesTheMotionGuarantees(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("css/components.css")
	if err != nil {
		t.Fatalf("read stylesheet: %v", err)
	}
	css := string(b)

	// The blanket kill is the reduced-motion rule that crushes animation and
	// transition durations; its element selector must carry both exemptions.
	kill := regexp.MustCompile(`prefers-reduced-motion: reduce\) \{ ([^{]+)\{[^}]*animation-duration: 0\.001ms !important`)
	m := kill.FindStringSubmatch(css)
	if m == nil {
		t.Fatal("the reduced-motion blanket kill rule is missing from css/components.css")
	}
	for _, exempt := range []string{":not(.y-sealfill)", ":not(.y-readline)"} {
		if !strings.Contains(m[1], exempt) {
			t.Errorf("the blanket kill selector %q is missing the %s exemption", strings.TrimSpace(m[1]), exempt)
		}
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
