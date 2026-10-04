package pages

import (
	"bytes"
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/assets"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

func TestRailSidebarProjection(t *testing.T) {
	t.Parallel()
	source, err := assets.Files.ReadFile("js/sidebar.js")
	if err != nil {
		t.Fatalf("read canonical sidebar: %v", err)
	}
	declaration := strings.TrimPrefix(string(source), "export ")
	scripts := regexp.MustCompile(`<script\b([^>]*)>([\s\S]*?)</script>`)
	nonceAttribute := regexp.MustCompile(`\bnonce="([^"]*)"`)
	strict := regexp.MustCompile(`^\s*\(\(\)\s*=>\s*\{\s*["']use strict["'];`)
	invoke := regexp.MustCompile(`\binitSidebar\(\);\s*\}\)\(\);\s*$`)
	for _, nonce := range []string{"probe-nonce", `probe"><script>bad</script>'&`} {
		t.Run(nonce, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := settleRail(layouts.Chrome{Nonce: nonce}).Render(t.Context(), &output); err != nil {
				t.Fatalf("render rail boundary: %v", err)
			}
			var projections [][]string
			for _, script := range scripts.FindAllStringSubmatch(output.String(), -1) {
				if strings.Contains(script[2], "function initSidebar() {") {
					projections = append(projections, script)
				}
			}
			if len(projections) != 1 {
				t.Fatalf("rail boundary has %d canonical sidebar projections, want 1 before deferred initialization", len(projections))
			}
			projection := projections[0]
			prefix := strict.FindStringIndex(projection[2])
			suffix := invoke.FindStringIndex(projection[2])
			if prefix == nil || suffix == nil || prefix[1] > suffix[0] {
				t.Fatal("canonical sidebar must run once in a private strict classic-script closure at the rail boundary")
			}
			body := strings.TrimSpace(projection[2][prefix[1]:suffix[0]])
			if diff := cmp.Diff(strings.TrimSpace(declaration), body); diff != "" {
				t.Errorf("complete rail projection source mismatch (-want +got):\n%s", diff)
			}
			attribute := nonceAttribute.FindStringSubmatch(projection[1])
			if len(attribute) != 2 {
				t.Fatal("rail projection has no single quoted nonce attribute")
			}
			if diff := cmp.Diff(nonce, html.UnescapeString(attribute[1])); diff != "" {
				t.Errorf("rail projection nonce mismatch (-want +got):\n%s", diff)
			}
			if strings.Count(projection[2], "initSidebar();") != 1 {
				t.Error("rail projection must invoke its canonical initializer exactly once")
			}
		})
	}
}

func TestSidebarDeclaration(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		source  string
		want    string
		wantErr string
	}{
		{name: "initializer", source: "export function initSidebar() {\n}\n", want: "function initSidebar() {\n}\n"},
		{name: "export text stays intact", source: "export function initSidebar() {\n// export retained\n}\n", want: "function initSidebar() {\n// export retained\n}\n"},
		{name: "missing export", source: "function initSidebar() {}", wantErr: "sidebar source must begin with its exported initializer"},
		{name: "leading comment", source: "// comment\nexport function initSidebar() {}", wantErr: "sidebar source must begin with its exported initializer"},
		{name: "different initializer", source: "export function other() {}", wantErr: "sidebar source must begin with its exported initializer"},
		{name: "closing script", source: "export function initSidebar() {\n// </script>\n}", wantErr: "sidebar source contains an HTML closing script token"},
		{name: "mixed case closing script", source: "export function initSidebar() {\n// </ScRiPt >\n}", wantErr: "sidebar source contains an HTML closing script token"},
		{name: "opening script", source: "export function initSidebar() {\n// <script>\n}", want: "function initSidebar() {\n// <script>\n}"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := sidebarDeclaration(tt.source)
			var message string
			if err != nil {
				message = err.Error()
			}
			if diff := cmp.Diff(tt.wantErr, message); diff != "" {
				t.Errorf("sidebarDeclaration error mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("sidebarDeclaration source mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
