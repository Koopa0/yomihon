package pages

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/assets"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestRailFilterProjection(t *testing.T) {
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
			if strings.Contains(body, "addEventListener") || strings.Contains(body, "sessionStorage.setItem") {
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

func TestRailFilterDeclaration(t *testing.T) {
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
			got, err := railFilterDeclaration(tt.source)
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

type projectionErrorWriter struct{ err error }

func (w projectionErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRailProjectionReturnsWriteError(t *testing.T) {
	want := errors.New("write refused")
	if err := railFilterInitializer("probe-nonce").Render(t.Context(), projectionErrorWriter{want}); !errors.Is(err, want) {
		t.Fatalf("caught: rail projection lost its write error: %v", err)
	}
}

func TestRailFilterCallerSet(t *testing.T) {
	var callers []string
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".templ") {
			continue
		}
		data, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "@settleRail(") {
			callers = append(callers, entry.Name())
		}
	}
	if !slices.Equal(callers, []string{"sidebar.templ"}) {
		t.Fatalf("caught: settlement caller set changed: %v", callers)
	}
	c := layouts.Chrome{Nonce: "probe-nonce"}
	long := longFolderSidebar(t)
	input := regexp.MustCompile(`<input\b[^>]*\bdata-nav-filter\b`)
	for _, tt := range []struct {
		name      string
		component templ.Component
		filtered  bool
	}{
		{"library", sidebar(Sidebar{Mode: SidebarLibrary}, c), false},
		{"book", sidebar(Sidebar{Mode: SidebarCourse}, c), false},
		{"folder", sidebar(Sidebar{Mode: SidebarFolder}, c), false},
		{"index", sidebar(Sidebar{Mode: SidebarIndex}, c), false},
		{"a folder past the rows a list is narrowed at", sidebar(long, c), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := tt.component.Render(t.Context(), &b); err != nil {
				t.Fatal(err)
			}
			text := b.String()
			want := 0
			if tt.filtered {
				want = 1
			}
			if input.MatchString(text) != tt.filtered || strings.Count(text, "function initRailFilter(rail, input)") != want || strings.Count(text, "railFilterState?.restore();") != want {
				t.Fatal("caught: caller filter declaration and projection membership disagree")
			}
		})
	}
}

// TestEveryRailKeepsItsScrollPositionAcrossPages holds the pre-paint script that
// carries a rail's scroll position from one page to the next. A page is a new
// document and a new document's rail starts at its top, so without this a
// reader on a long course lost their place in the rail on every click. Every
// rail goes through the one boundary, so each of the four kinds is rendered and
// each must carry the script once, after the filter has been restored, because
// a filtered tree is a different height. It reads the storage key and the
// event that writes it, which are what the two pages agree on, and leaves the
// browser's own behavior to the probe that scrolls a real rail.
func TestEveryRailKeepsItsScrollPositionAcrossPages(t *testing.T) {
	t.Parallel()
	c := layouts.Chrome{Nonce: "probe-nonce"}
	long := longFolderSidebar(t)
	for _, tt := range []struct {
		name      string
		component templ.Component
		filtered  bool
	}{
		{"library", sidebar(Sidebar{Mode: SidebarLibrary}, c), false},
		{"book", sidebar(Sidebar{Mode: SidebarCourse}, c), false},
		{"folder", sidebar(Sidebar{Mode: SidebarFolder}, c), false},
		{"index", sidebar(Sidebar{Mode: SidebarIndex}, c), false},
		{"a folder past the rows a list is narrowed at", sidebar(long, c), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			if err := tt.component.Render(t.Context(), &out); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			if got := strings.Count(text, `"yomihon.railTop"`); got != 2 {
				t.Fatalf("the rail names its position key %d times, want 2 (read it, and write it when the page is left)", got)
			}
			if !strings.Contains(text, `addEventListener("pagehide"`) {
				t.Error("the rail never writes its position when the page is left, so the next page cannot put it back")
			}
			if tt.filtered {
				restore := strings.Index(text, "railFilterState?.restore()")
				keep := strings.Index(text, `"yomihon.railTop"`)
				if restore < 0 || keep < restore {
					t.Errorf("the position is put back at %d, before the filter is restored at %d, so a filtered tree would be scrolled for the wrong height", keep, restore)
				}
			}
		})
	}
}

// longFolderSidebar is the sidebar of a note in a folder of more files than a
// list is narrowed at, the one shape that draws the filter.
func longFolderSidebar(t *testing.T) Sidebar {
	t.Helper()
	root := t.TempDir()
	for i := range filterRows + 5 {
		full := filepath.Join(root, "Long", fmt.Sprintf("note-%02d.md", i))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("body\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return NewFolderSidebar(nav.Shell{Nav: modelOf(t, root)}, "Long/note-00.md")
}

// TestTheFilterComesOnlyPastTwentyRows holds the rule the head draws by: a list
// that fits in one look has no filter, and one longer than filterRows has one.
func TestTheFilterComesOnlyPastTwentyRows(t *testing.T) {
	t.Parallel()
	c := layouts.Chrome{Nonce: "n", Lang: wording.En}
	short := NewFolderSidebar(nav.Shell{Nav: buildModel(t)}, "Notes/alpha.md")
	if short.FilterShown(c.Lang) {
		t.Errorf("a folder of %d rows is offered a filter", short.rows(c.Lang))
	}
	long := longFolderSidebar(t)
	if !long.FilterShown(c.Lang) {
		t.Errorf("a folder of %d rows is offered no filter", long.rows(c.Lang))
	}
}
