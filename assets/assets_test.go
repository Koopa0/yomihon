package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"
)

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
