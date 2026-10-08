package layouts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	projectassets "github.com/koopa0/yomihon/assets"
)

func TestBasePreloadsFirstPaintFonts(t *testing.T) {
	t.Parallel()
	source, err := os.ReadFile("../../../assets/css/fonts.css")
	if err != nil {
		t.Fatalf("read font declarations: %v", err)
	}
	want := firstPaintFontSources(t, string(source))
	if len(want) == 0 {
		t.Fatal("fonts.css declares no first-paint fonts")
	}
	for i, href := range want {
		if !strings.HasPrefix(href, "/static/fonts/") || filepath.Ext(href) != ".woff2" {
			t.Fatalf("font source = %q, want a local WOFF2 font", href)
		}
		body, readErr := projectassets.Files.ReadFile(strings.TrimPrefix(href, "/static/"))
		if readErr != nil {
			t.Fatalf("font source %q has no vendored resource: %v", href, readErr)
		}
		sum := sha256.Sum256(body)
		want[i] = href + "?v=" + hex.EncodeToString(sum[:])[:12]
	}
	var buf bytes.Buffer
	if renderErr := Base(Chrome{Title: "test"}).Render(t.Context(), &buf); renderErr != nil {
		t.Fatalf("render base: %v", renderErr)
	}
	doc, err := html.Parse(&buf)
	if err != nil {
		t.Fatalf("parse base document: %v", err)
	}
	var got []string
	seenStylesheet := false
	for node := range doc.Descendants() {
		if node.Type != html.ElementNode || node.Data != "link" {
			continue
		}
		attrs := make(map[string]string, len(node.Attr))
		for _, attr := range node.Attr {
			attrs[attr.Key] = attr.Val
		}
		relations := strings.Fields(attrs["rel"])
		if slices.Contains(relations, "stylesheet") {
			seenStylesheet = true
		}
		if !slices.Contains(relations, "preload") {
			continue
		}
		if attrs["as"] != "font" && !strings.HasPrefix(attrs["href"], "/static/fonts/") {
			continue
		}
		got = append(got, attrs["href"])
		if node.Parent == nil || node.Parent.Data != "head" || seenStylesheet {
			t.Errorf("preload %q must be declared in head before the stylesheets", attrs["href"])
		}
		_, crossOrigin := attrs["crossorigin"]
		properties := map[string]string{
			"as": attrs["as"], "type": attrs["type"], "crossorigin": attrs["crossorigin"],
		}
		if attrs["crossorigin"] == "anonymous" {
			properties["crossorigin"] = ""
		}
		if diff := cmp.Diff(map[string]string{"as": "font", "type": "font/woff2", "crossorigin": ""}, properties); diff != "" {
			t.Errorf("preload %q properties (-want +got):\n%s", attrs["href"], diff)
		}
		if !crossOrigin {
			t.Errorf("preload %q has no crossorigin attribute", attrs["href"])
		}
	}
	slices.Sort(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Base() first-paint font preloads (-fonts.css +head):\n%s", diff)
	}
}

// firstPaintFontSources reads every roman face whose subset includes basic
// Latin. A face without a style or a range uses the CSS defaults, so it is
// included too. The declarations, rather than filenames, decide membership.
func firstPaintFontSources(t *testing.T, source string) []string {
	t.Helper()
	source = cssComments.ReplaceAllString(source, "")
	faces := regexp.MustCompile(`(?i)@font-face\s*\{([^{}]*)\}`).FindAllStringSubmatch(source, -1)
	if len(faces) != len(regexp.MustCompile(`(?i)@font-face\b`).FindAllString(source, -1)) {
		t.Fatal("font-face declaration could not be read")
	}
	var sources []string
	for _, face := range faces {
		declarations := make(map[string]string)
		for declaration := range strings.SplitSeq(face[1], ";") {
			key, value, ok := strings.Cut(declaration, ":")
			if ok {
				declarations[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
			}
		}
		style := strings.ToLower(declarations["font-style"])
		if style != "" && style != "normal" {
			continue
		}
		if !fontRangeIncludesBasicLatin(t, declarations["unicode-range"]) {
			continue
		}
		urls := regexp.MustCompile(`(?i)url\(\s*['"]?([^'"\s)]+)['"]?\s*\)`).FindAllStringSubmatch(declarations["src"], -1)
		if len(urls) == 0 {
			t.Fatalf("first-paint font has no source URL: %q", face[0])
		}
		for _, match := range urls {
			sources = append(sources, match[1])
		}
	}
	slices.Sort(sources)
	return slices.Compact(sources)
}

func fontRangeIncludesBasicLatin(t *testing.T, value string) bool {
	t.Helper()
	if value == "" {
		return true
	}
	var covered [128]bool
	for part := range strings.SplitSeq(strings.ToUpper(value), ",") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "U+") {
			t.Fatalf("invalid font unicode-range: %q", part)
		}
		low, high, ranged := strings.Cut(strings.TrimPrefix(part, "U+"), "-")
		if !ranged {
			high = strings.ReplaceAll(low, "?", "F")
			low = strings.ReplaceAll(low, "?", "0")
		}
		start, err := strconv.ParseUint(low, 16, 32)
		if err != nil {
			t.Fatalf("invalid font unicode-range start %q: %v", low, err)
		}
		end, err := strconv.ParseUint(high, 16, 32)
		if err != nil || end < start || end > 0x10ffff {
			t.Fatalf("invalid font unicode-range end %q for %q: %v", high, low, err)
		}
		for code := start; code <= end && code < uint64(len(covered)); code++ {
			covered[code] = true
		}
	}
	return !slices.Contains(covered[:], false)
}

func TestFirstPaintFontSources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{name: "empty"},
		{name: "default style and range", source: `@font-face { src: url('/default.woff2'); }`, want: []string{"/default.woff2"}},
		{name: "variable roman", source: `@font-face { font-style: normal; font-weight: 300 700; src: url(/roman.woff2); }`, want: []string{"/roman.woff2"}},
		{name: "italic", source: `@font-face { font-style: italic; src: url('/italic.woff2'); }`},
		{name: "extended Latin", source: `@font-face { src: url('/extended.woff2'); unicode-range: U+0100-02BA, U+2020; }`},
		{name: "mixed ranges", source: `@font-face { src: url('/latin.woff2'); unicode-range: U+2000-206F, U+0000-007F; }`, want: []string{"/latin.woff2"}},
		{name: "split basic Latin", source: `@font-face { src: url('/split.woff2'); unicode-range: U+0000-003F, U+0040-007F; }`, want: []string{"/split.woff2"}},
		{name: "wildcard", source: `@font-face { src: url('/wildcard.woff2'); unicode-range: U+00??; }`, want: []string{"/wildcard.woff2"}},
		{name: "partial basic Latin", source: `@font-face { src: url('/partial.woff2'); unicode-range: U+0020-007F; }`},
		{name: "whole set", source: `/* @font-face { src: url('/comment.woff2'); } */ @font-face { src: url('/second.woff2'); } @font-face { font-style: normal; src: url('/first.woff2'); } @font-face { font-style: italic; src: url('/italic.woff2'); }`, want: []string{"/first.woff2", "/second.woff2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, firstPaintFontSources(t, tt.source)); diff != "" {
				t.Errorf("firstPaintFontSources() (-want +got):\n%s", diff)
			}
		})
	}
}
