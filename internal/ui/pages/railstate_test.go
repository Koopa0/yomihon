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
	source, err := assets.Files.ReadFile("js/rail-filter.js")
	if err != nil {
		t.Fatalf("read canonical filter: %v", err)
	}
	declaration := strings.TrimSuffix(strings.TrimSpace(string(source)), "\nexport { initRailFilter };")
	sidebar, err := assets.Files.ReadFile("js/sidebar.js")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(sidebar), "import { initRailFilter } from './rail-filter.js';") != 1 {
		t.Fatal("caught: deferred sidebar does not import the canonical filter")
	}
	scripts := regexp.MustCompile(`<script\b([^>]*)>([\s\S]*?)</script>`)
	nonceAttribute := regexp.MustCompile(`\bnonce="([^"]*)"`)
	strict := regexp.MustCompile(`^\s*\(\(\)\s*=>\s*\{\s*["']use strict["'];`)
	invoke := regexp.MustCompile(`\nconst rail = document\.querySelector\('\.y-rail-left'\);[\s\S]*if \(input\) initRailFilter\(rail, input\);\s*\}\)\(\);\s*$`)
	for _, nonce := range []string{"probe-nonce", `probe"><script>bad</script>'&`} {
		t.Run(nonce, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := settleRail(layouts.Chrome{Nonce: nonce}, true).Render(t.Context(), &output); err != nil {
				t.Fatalf("render rail boundary: %v", err)
			}
			var projections [][]string
			for _, script := range scripts.FindAllStringSubmatch(output.String(), -1) {
				if strings.Contains(script[2], "function initRailFilter(") {
					projections = append(projections, script)
				}
				attribute := nonceAttribute.FindStringSubmatch(script[1])
				if len(attribute) != 2 || html.UnescapeString(attribute[1]) != nonce || strings.Contains(script[1], "type=") {
					t.Fatal("caught: rail phase is not a nonce-bearing classic script")
				}
			}
			if len(projections) != 1 {
				t.Fatalf("caught: rail has %d canonical filtering projections, want 1", len(projections))
			}
			projection := projections[0]
			prefix := strict.FindStringIndex(projection[2])
			suffix := invoke.FindStringIndex(projection[2])
			if prefix == nil || suffix == nil || prefix[1] > suffix[0] {
				t.Fatal("caught: canonical filter must be captured once in a private strict classic closure")
			}
			body := strings.TrimSpace(projection[2][prefix[1]:suffix[0]])
			if diff := cmp.Diff(strings.TrimSpace(declaration), body); diff != "" {
				t.Errorf("caught: whole filtering projection mismatch (-want +got):\n%s", diff)
			}
			if strings.Contains(body, "addEventListener") || strings.Contains(body, "sidebarController") || strings.Contains(body, "sessionStorage.setItem") {
				t.Error("caught: deferred interaction was copied into the prepaint filter")
			}
			text := output.String()
			capture := strings.Index(text, "if (input) initRailFilter(rail, input);")
			common := strings.Index(text, `d.open = want;`)
			restore := strings.Index(text, `railFilterState?.restore();`)
			if capture < 0 || common <= capture || restore <= common {
				t.Error("caught: original state capture, common settlement and filter restoration are out of order")
			}
		})
	}
}

func TestSidebarDeclaration(t *testing.T) {
	t.Parallel()
	body := "function initRailFilter(rail, input) {\n}\n"
	export := "export { initRailFilter };\n"
	for _, tt := range []struct {
		name    string
		source  string
		want    string
		wantErr string
	}{
		{name: "factory", source: body + export, want: strings.TrimSpace(body)},
		{name: "line header", source: "// import and export are module words\n" + body + export, want: "// import and export are module words\n" + strings.TrimSpace(body)},
		{name: "block header", source: "/*\nexport is a word in a header\n*/\n" + body + export, want: "/*\nexport is a word in a header\n*/\n" + strings.TrimSpace(body)},
		{name: "export text", source: "function initRailFilter(rail, input) {\n// export retained\nconst word = 'export';\n}\n" + export, want: "function initRailFilter(rail, input) {\n// export retained\nconst word = 'export';\n}"},
		{name: "missing export", source: body, wantErr: "rail filter source must end with its single factory export"},
		{name: "different factory", source: "function other() {}\n" + export, wantErr: "rail filter source must contain only its plain import-free factory"},
		{name: "import", source: "import { x } from './other.js';\n" + body + export, wantErr: "rail filter source must contain only its plain import-free factory"},
		{name: "second export", source: body + "export { other };\n" + export, wantErr: "rail filter source must contain only its plain import-free factory"},
		{name: "duplicate export", source: body + export + export, wantErr: "rail filter source must contain only its plain import-free factory"},
		{name: "closing script", source: strings.Replace(body, "{", "{\n// </script>", 1) + export, wantErr: "rail filter source contains an HTML closing script token"},
		{name: "mixed case closing script", source: strings.Replace(body, "{", "{\n// </ScRiPt >", 1) + export, wantErr: "rail filter source contains an HTML closing script token"},
		{name: "unclosed header", source: "/* unfinished\n" + body + export, wantErr: "rail filter source must contain only its plain import-free factory"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := sidebarDeclaration(tt.source)
			var message string
			if err != nil {
				message = err.Error()
			}
			if diff := cmp.Diff(tt.wantErr, message); diff != "" {
				t.Errorf("caught: declaration error mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("caught: declaration body mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
