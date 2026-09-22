package note_test

import (
	"html"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/pages"
)

const readingLibraryRoot = "../../examples/vault"

// The dialect test protects individual constructs. This protects both courses:
// every chapter resolves, and the timeout detour stays off the Go main line.
func TestShippedReadingLibraryCourseOrder(t *testing.T) {
	t.Parallel()
	contract := readingLibraryContract(t)
	store, _ := newSnapshotStore(t, readingLibraryRoot, slog.New(slog.DiscardHandler), contract, contract.Governance())
	model := store.Current().Capture().Navigation()
	for _, tt := range []struct {
		path    string
		primary []string
		local   string
		rows    []string
	}{
		{
			path: "Notes/Books/Go 並行入門.md",
			primary: []string{
				"Lessons/go/G01 goroutine 與等待.md",
				"Lessons/go/G02 channel 的交接.md",
				"Lessons/go/G03 關閉 channel.md",
				"Lessons/go/G04 取消不再需要的工作.md",
				"Lessons/go/G05 固定數量的 worker.md",
			},
			local: "Lessons/go/G06 select 與逾時.md",
			rows: []string{
				"Lessons/go/G01 goroutine 與等待.md",
				"Lessons/go/G02 channel 的交接.md",
				"Lessons/go/G03 關閉 channel.md",
				"Lessons/go/G04 取消不再需要的工作.md",
				"Lessons/go/G06 select 與逾時.md",
				"Lessons/go/G05 固定數量的 worker.md",
			},
		},
		{
			path: "Notes/Books/在圖書館讀日文.md",
			primary: []string{
				"Lessons/japanese/J01 找到書的位置.md",
				"Lessons/japanese/J02 決定在哪裡讀.md",
			},
			rows: []string{
				"Lessons/japanese/J01 找到書的位置.md",
				"Lessons/japanese/J02 決定在哪裡讀.md",
			},
		},
	} {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			course := model.Path(tt.path)
			if course == nil {
				t.Fatal("the shipped course does not project")
			}
			if course.Planned != len(tt.primary) || len(course.Diagnostics) != 0 {
				t.Errorf("course has %d main-line chapters and diagnostics %v; want %d chapters and no diagnostics", course.Planned, course.Diagnostics, len(tt.primary))
			}
			if diff := cmp.Diff(tt.rows, libraryChapters(t, course.Groups)); diff != "" {
				t.Errorf("readable chapters (-want +got):\n%s", diff)
			}
			for i, rel := range tt.primary {
				var prev, next string
				if i > 0 {
					prev = tt.primary[i-1]
				}
				if i+1 < len(tt.primary) {
					next = tt.primary[i+1]
				}
				assertLibraryNeighbors(t, model, tt.path, rel, prev, next)
			}
			if tt.local != "" {
				assertLibraryNeighbors(t, model, tt.path, tt.local, "", "")
			}
		})
	}
}

func readingLibraryContract(t *testing.T) *schema.Contract {
	t.Helper()
	contract, err := schema.Load(readingLibraryRoot)
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

// Read the projected rows, not a second interpretation of the authored list.
func libraryChapters(t *testing.T, groups []*nav.PathGroup) []string {
	t.Helper()
	var chapters []string
	for _, group := range groups {
		for _, item := range group.Items {
			if group.Teaches(item.Entry) {
				if !item.Entry.Openable() {
					t.Errorf("chapter %q is not readable", item.Entry.Target)
				}
				chapters = append(chapters, item.Entry.RelPath)
			}
			if item.Group != nil {
				chapters = append(chapters, libraryChapters(t, []*nav.PathGroup{item.Group})...)
			}
		}
	}
	return chapters
}

func assertLibraryNeighbors(t *testing.T, model *nav.Model, course, rel, prev, next string) {
	t.Helper()
	steps := model.PathNeighbors(rel)
	at := slices.IndexFunc(steps, func(step nav.Neighbors) bool { return step.PathRelPath == course })
	if at < 0 {
		t.Fatalf("chapter %q is absent from %q", rel, course)
	}
	if got := steps[at]; got.Prev.RelPath != prev || got.Next.RelPath != next {
		t.Errorf("chapter %q has prev=%q next=%q; want prev=%q next=%q", rel, got.Prev.RelPath, got.Next.RelPath, prev, next)
	}
}

// A valid sidecar with the wrong slug silently disappears. Request the shipped
// lesson, so a parsed YAML file alone cannot stand in for an exercise a reader
// actually receives. The concept link must retain its no-script destination.
func TestShippedJapaneseLessonsCarryTheirPractice(t *testing.T) {
	t.Parallel()
	srv := newServerWithContract(t, readingLibraryRoot, readingLibraryContract(t))
	conceptHref := pages.VaultHref("/notes/", "Concepts/japanese/日文的地點與動作.md")
	for _, tt := range []struct {
		path     string
		sentence string
		gloss    string
	}{
		{"Lessons/japanese/J01 找到書的位置.md", "辞書は本棚にあります。", "辭典在書架。"},
		{"Lessons/japanese/J02 決定在哪裡讀.md", "図書館で雑誌を読みます。", "在圖書館讀雜誌。"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			code, page := get(t, srv.Client(), srv.URL+pages.VaultHref("/notes/", tt.path))
			if code != http.StatusOK {
				t.Fatalf("lesson returned %d", code)
			}
			if count := strings.Count(page, `class="y-slotcard"`); count != 1 {
				t.Fatalf("lesson has %d sentence cards, want 1", count)
			}
			output := libraryElement(t, page, `<p class="y-slotoutput"`, "</p>")
			// Ruby readings are alternate pronunciation, not extra words in the
			// sentence. Strip them before comparing the visible base text.
			output = regexp.MustCompile(`(?s)<rt>.*?</rt>|<[^>]*>`).ReplaceAllString(output, "")
			if got := strings.Join(strings.Fields(html.UnescapeString(output)), ""); got != tt.sentence {
				t.Errorf("initial practice sentence = %q, want %q", got, tt.sentence)
			}
			if gloss := libraryElement(t, page, `<p class="y-slotgloss"`, "</p>"); gloss != tt.gloss {
				t.Errorf("initial practice gloss = %q, want %q", gloss, tt.gloss)
			}
			trigger := regexp.MustCompile(`<a href="` + regexp.QuoteMeta(conceptHref) + `" class="wikilink concept-link" data-concept="([^"]+)"`).FindStringSubmatch(page)
			if len(trigger) != 2 {
				t.Fatal("lesson lost its navigable concept-sheet trigger")
			}
			sheet := libraryElement(t, page, `<template id="concept-`+trigger[1]+`"`, "</template>")
			if !strings.Contains(sheet, "句子是在安放一件物品，還是在說某個動作？") {
				t.Error("concept trigger has no matching explanation in its sheet")
			}
		})
	}
}

func libraryElement(t *testing.T, page, opening, closing string) string {
	t.Helper()
	_, tail, found := strings.Cut(page, opening)
	if !found {
		t.Fatalf("lesson has no %s", opening)
	}
	_, tail, found = strings.Cut(tail, ">")
	if !found {
		t.Fatalf("lesson has an unclosed %s", opening)
	}
	body, _, found := strings.Cut(tail, closing)
	if !found {
		t.Fatalf("lesson has no %s after %s", closing, opening)
	}
	return body
}
