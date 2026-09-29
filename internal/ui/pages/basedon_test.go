package pages

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

func TestDeclaredSourceAndTextCitationLabelsDiffer(t *testing.T) {
	t.Parallel()

	const (
		declared = `ui-side__label">聲明的來源`
		cited    = `ui-side__label">正文連到這篇`
		islands  = "沒有正文 [[…]] 連結連過來的筆記"
	)
	if declared == cited || declared == islands {
		t.Fatalf("declared-source %q must differ from cited-in-the-text labels %q and %q", declared, cited, islands)
	}

	var noteBuf bytes.Buffer
	noteView := NoteView{
		Title:         "Derived model",
		RelPath:       "Concepts/yomihon/Derived model.md",
		VaultHasLinks: true,
		BasedOn:       []snapshot.DeclaredSource{{Name: "Source model", RelPath: "Concepts/yomihon/Source model.md"}},
		CitedBy:       []nav.NoteRef{{Name: "Other", RelPath: "Concepts/yomihon/Other.md"}},
	}
	if err := Note(noteView, layouts.Chrome{}).Render(t.Context(), &noteBuf); err != nil {
		t.Fatalf("render note: %v", err)
	}
	noteHTML := noteBuf.String()
	if !strings.Contains(noteHTML, declared) {
		t.Errorf("the note page does not show the declared-source label %q", declared)
	}
	if !strings.Contains(noteHTML, cited) {
		t.Errorf("the note page does not show the text-citation label %q", cited)
	}
	if !strings.Contains(noteHTML, `class="y-basedon"`) {
		t.Error("the note page has no declared-source block")
	}
	if !strings.Contains(noteHTML, `href="/notes/Concepts/yomihon/Source`) {
		t.Error("a resolved source is not a link on the note page")
	}
	railStart := strings.Index(noteHTML, `<aside class="y-rail-right"`)
	if railStart < 0 {
		t.Fatal("the note page has no right rail")
	}
	if !strings.Contains(noteHTML[railStart:], `class="y-basedon"`) {
		t.Error("the wide rail has no declared-source block")
	}

	var healthBuf bytes.Buffer
	healthView := HealthView{
		Islands:     []HealthIslandGroup{{Dir: "Concepts/yomihon", Notes: []nav.NoteRef{{Name: "Source model", RelPath: "Concepts/yomihon/Source model.md"}}}},
		IslandCount: 1,
	}
	if err := Health(healthView, layouts.Chrome{}).Render(t.Context(), &healthBuf); err != nil {
		t.Fatalf("render health: %v", err)
	}
	healthHTML := healthBuf.String()
	if !strings.Contains(healthHTML, islands) {
		t.Errorf("the health list does not show the text-citation island label %q", islands)
	}
	if strings.Contains(healthHTML, "沒有人連過來的筆記") {
		t.Error("the health list still uses the old unscoped island heading")
	}
	if !strings.Contains(healthHTML, "一般 Markdown 連結和來源聲明都不計入") {
		t.Error("the health list does not exclude ordinary Markdown links and declared sources from its scope")
	}
}

func TestDeclaredSourceRendersAuthorTextWhenUnlinked(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	view := NoteView{
		BasedOn: []snapshot.DeclaredSource{{Name: "[[twin]]"}, {Name: "Book notes", RelPath: "Book notes.md"}},
	}
	if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "[[twin]]") {
		t.Errorf("the author's ambiguous value is missing: %q", html)
	}
	if strings.Contains(html, `href="/notes/A/twin.md"`) || strings.Contains(html, `href="/notes/B/twin.md"`) {
		t.Error("an unlinked declared source was guessed into a href")
	}
	if !strings.Contains(html, `href="/notes/Book`) {
		t.Error("the resolved bare name is not a link")
	}
}

func TestDeclaredSourcesRenderInDeclarationOrder(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	view := NoteView{
		BasedOn: []snapshot.DeclaredSource{
			{Name: "Zebra", RelPath: "Sources/Zebra.md"},
			{Name: "Apple", RelPath: "Sources/Apple.md"},
			{Name: "Middle", RelPath: "Sources/Middle.md"},
		},
	}
	if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	zebra := strings.Index(html, "Zebra")
	apple := strings.Index(html, "Apple")
	middle := strings.Index(html, "Middle")
	if zebra < 0 || apple < 0 || middle < 0 {
		t.Fatalf("a declared source is missing from the page")
	}
	if zebra >= apple || apple >= middle {
		t.Errorf("declared sources were reordered on the page")
	}
}

// The source-location card decides which rows to preview from what the page
// says about them: the block they sit in, the class a row whose place is
// missing carries, and the address. This holds that reading to the markup, so a
// change to the block cannot quietly widen or empty the set the card is offered
// on.
func TestDeclaredSourceRowsCarryWhatTheCardReads(t *testing.T) {
	t.Parallel()

	view := NoteView{
		Title:   "Claim",
		RelPath: "Notes/Claim.md",
		BasedOn: []snapshot.DeclaredSource{
			{
				Name:    "Study",
				RelPath: "Notes/Study.md",
				Locations: []render.SourceLocation{
					{Label: "Methods", Fragment: "methods"},
					{Label: "Absent", Fragment: "absent", Reason: "no such section"},
				},
			},
			{
				Name:      "Handbook",
				RelPath:   "Notes/Handbook.md",
				Locations: []render.SourceLocation{{Label: "Limits", Fragment: "limits"}},
			},
			{
				Name:      "Gone",
				RelPath:   "Notes/Gone.md",
				Locations: []render.SourceLocation{{Label: "Lost", Fragment: "lost", Reason: "no such section"}},
			},
			{Name: "Data", RelPath: "Notes/data.txt"},
			{Name: "[[twin]]"},
		},
		DeclaredBy: []DeclaringNoteView{{
			Note:      nav.NoteRef{Name: "Later", RelPath: "Notes/Later.md"},
			Locations: []DeclaredPlaceView{{Label: "Methods", Href: "/notes/Notes/Later.md#methods"}},
		}},
	}
	var buf bytes.Buffer
	if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()

	// The rail and the disclosure the narrow layout folds draw each list once.
	blocks := regexp.MustCompile(`(?s)<nav class="y-basedon"([^>]*)>(.*?)</nav>`).FindAllStringSubmatch(html, -1)
	anchor := regexp.MustCompile(`<a class="([^"]*)" href="([^"]*)"`)
	var own, declaring int
	for _, block := range blocks {
		if strings.Contains(block[1], "data-declared-by") {
			// The list of notes declaring this one shares the block's class and
			// is not this note's own source list, so the card is told apart from
			// it by the attribute alone.
			declaring++
			continue
		}
		own++
		var previewable, refused []string
		for _, match := range anchor.FindAllStringSubmatch(block[2], -1) {
			class, href := match[1], match[2]
			path, _, _ := strings.Cut(href, "#")
			if !strings.Contains(class, "wikilink-degraded") && strings.HasPrefix(href, "/notes/") && strings.HasSuffix(path, ".md") {
				previewable = append(previewable, href)
			} else {
				refused = append(refused, href)
			}
		}
		// A group with several locations is a link to the whole file and one
		// link per location; a lone location is one compact row and no group
		// link. A row whose place is missing, and a file that is not a note,
		// are refused.
		wantPreviewable := []string{
			"/notes/Notes/Study.md",
			"/notes/Notes/Study.md#methods",
			"/notes/Notes/Handbook.md#limits",
		}
		wantRefused := []string{
			"/notes/Notes/Study.md#absent",
			"/notes/Notes/Gone.md#lost",
			"/notes/Notes/data.txt",
		}
		if diff := cmp.Diff(wantPreviewable, previewable); diff != "" {
			t.Errorf("rows the card is offered on (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(wantRefused, refused); diff != "" {
			t.Errorf("rows the card must refuse (-want +got):\n%s", diff)
		}
	}
	if own != 2 {
		t.Errorf("the note draws %d source lists of its own, want 2 (rail and disclosure)", own)
	}
	if declaring != 2 {
		t.Errorf("the note draws %d lists of notes declaring it, want 2 (rail and disclosure)", declaring)
	}
}
