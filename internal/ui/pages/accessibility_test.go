package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/ui/layouts"
)

func TestPageMainRegionsAreSkipTargets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		page templ.Component
	}{
		{name: "home", page: Home(HomeView{}, layouts.Chrome{})},
		{name: "search", page: Search(SearchView{}, layouts.Chrome{})},
		{name: "note", page: Note(NoteView{}, layouts.Chrome{})},
		{name: "file", page: File(FileView{}, layouts.Chrome{})},
		{name: "report", page: Report(ReportView{}, layouts.Chrome{})},
		{name: "syllabus", page: Syllabus(PathView{}, layouts.Chrome{})},
		{name: "status recovery", page: StatusRecovery(StatusRecoveryView{}, layouts.Chrome{})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := tt.page.Render(t.Context(), &buf); err != nil {
				t.Fatalf("render %s page: %v", tt.name, err)
			}
			const target = `<main id="main-content" tabindex="-1"`
			if html := buf.String(); !strings.Contains(html, target) {
				t.Errorf("%s page has no focusable skip target %q; html = %q", tt.name, target, html)
			}
		})
	}
}
