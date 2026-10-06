package pages

import (
	"bytes"
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/ui/layouts"
)

type projectionErrorWriter struct{ err error }

func (w projectionErrorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRailProjectionReturnsWriteError(t *testing.T) {
	want := errors.New("write refused")
	if err := sidebarInitializer("probe-nonce").Render(t.Context(), projectionErrorWriter{want}); !errors.Is(err, want) {
		t.Fatalf("caught: rail projection lost its write error: %v", err)
	}
}

func TestRailFilterAcceptsCommentedPlainFactory(t *testing.T) {
	source := "// Ordinary module header.\nfunction initRailFilter(rail, input) { return { applyFilter() {} }; }\nexport { initRailFilter };\n"
	got, err := sidebarDeclaration(source)
	if err != nil {
		t.Fatalf("caught: ordinary shared factory rejected: %v", err)
	}
	if !strings.Contains(got, "// Ordinary module header.") || strings.Contains(got, "export { initRailFilter }") {
		t.Fatal("caught: projection lost the header or retained its module export")
	}
}

func TestUnfilteredRailHasNoFilteringProjection(t *testing.T) {
	var output bytes.Buffer
	if err := syllabusRail(PathView{}, layouts.Chrome{Nonce: "probe-nonce"}).Render(t.Context(), &output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "function initSidebar()") || strings.Contains(output.String(), "function initRailFilter(") {
		t.Fatal("caught: an unfiltered syllabus carries filtering source")
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
	if !slices.Equal(callers, []string{"readingrail.templ", "sidebar.templ", "syllabus.templ"}) {
		t.Fatalf("caught: settlement caller set changed: %v", callers)
	}
	c := layouts.Chrome{Nonce: "probe-nonce"}
	input := regexp.MustCompile(`<input\b[^>]*\bdata-nav-filter\b`)
	for _, tt := range []struct {
		name      string
		component templ.Component
		filtered  bool
	}{
		{"sidebar", sidebar(Sidebar{}, c), true},
		{"book", readingRail(ReadingRail{Kind: ReadingRailBook}, c), true},
		{"folder", readingRail(ReadingRail{Kind: ReadingRailFolder}, c), true},
		{"reports", readingRail(ReadingRail{Kind: ReadingRailReports}, c), true},
		{"syllabus", syllabusRail(PathView{}, c), false},
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
