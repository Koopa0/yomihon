package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestReadingRailBookShowsOnePath(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	current := "Writing/lessons/go/L01.md"

	var buf bytes.Buffer
	if err := readingRail(NewReadingRail(nav.Shell{Nav: model}, current, "golang"), layouts.Chrome{Nonce: "n", Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-reading-rail="book"`,
		`data-book-path="Maps/Go path.md"`,
		`data-book-branch=`,
		`class="y-lessonsteps"`,
		`aria-current="page"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("book rail missing %q; html = %q", want, html)
		}
	}
	for _, ban := range []string{
		`data-sidebar-group="paths"`,
		`data-sidebar-group="maps"`,
		`data-sidebar-group="journal"`,
		`data-sidebar-group="reports"`,
	} {
		if strings.Contains(html, ban) {
			t.Errorf("book rail still contains %q", ban)
		}
	}
	// One book replaces the vault drawers, not the reader's manual choices.
	// The browser probe owns restoration behavior; this render check ensures
	// that this rail emits exactly one nonce-bound initializer to read them.
	if got := strings.Count(html, `sessionStorage.getItem("yomihon.nav")`); got != 1 {
		t.Errorf("book rail has %d disclosure-state readers, want one", got)
	}
	if !strings.Contains(html, `</aside><script nonce="n">`) {
		t.Error("book rail does not initialize its state after its markup with the response nonce")
	}
}

func TestReadingRailFolderShowsSiblings(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	current := "Concepts/go/C01.md"

	var buf bytes.Buffer
	if err := readingRail(NewReadingRail(nav.Shell{Nav: model}, current, "golang"), layouts.Chrome{Nonce: "n", Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-reading-rail="folder"`,
		`class="y-here"`,
		`href="/notes/Concepts/go/C01.md"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("folder rail missing %q", want)
		}
	}
	if strings.Contains(html, `data-sidebar-group=`) {
		t.Error("folder rail still carries vault drawer groups")
	}
}

func TestReadingRailReportsListsReports(t *testing.T) {
	t.Parallel()
	model := buildModel(t)
	current := "System/reports/2026-07-10 vault audit.md"

	var buf bytes.Buffer
	if err := readingRail(NewReadingRail(nav.Shell{Nav: model}, current, ""), layouts.Chrome{Nonce: "n", Lang: wording.ZhHant}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-reading-rail="reports"`,
		`data-reading-reports`,
		`Vault audit`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("reports rail missing %q", want)
		}
	}
}
