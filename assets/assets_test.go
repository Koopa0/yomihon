package assets

import (
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

func TestJavaScriptUsesTraditionalChineseSpeechLabels(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("js/yomihon.js")
	if err != nil {
		t.Fatalf("read JavaScript: %v", err)
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
