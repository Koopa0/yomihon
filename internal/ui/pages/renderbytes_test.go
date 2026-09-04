package pages

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// updateRenderBytes rewrites the recorded HTML instead of comparing against it.
// Regenerating is a decision about what the pages emit, so it is an explicit
// action rather than something a failing run does on its own.
var updateRenderBytes = flag.Bool("update-render-bytes", false, "rewrite the recorded page bytes")

// TestRenderedBytesAreUnchanged records what each surface writes, so a change
// that is meant to move code and not output has something that can tell it did.
// Moving a component between files, folding two switches into one, or lifting a
// helper out of a template all leave the compiler and every behavioural test
// green while a dropped space, a reordered attribute, or a lost branch goes
// unseen — the reader is the only instrument that would ever have noticed.
//
// A recorded file states what the pages emit today, not what they owe anybody:
// where a change is meant to alter what a page says, the recording is rewritten
// in the same commit and the reason belongs in that commit's message.
func TestRenderedBytesAreUnchanged(t *testing.T) {
	t.Parallel()

	model := buildModel(t)
	current := "Writing/lessons/go/L01.md"
	shelfIndex, shelfRecent, shelfStatuses := recordedShelfView(model)

	type surface struct {
		name      string
		component templ.Component
	}
	cases := []surface{
		{"sidebar-current-note", sidebar(NewSidebar(model, current), layouts.Chrome{Nonce: "response-nonce"})},
		{"sidebar-no-note", sidebar(NewSidebar(model, ""), layouts.Chrome{Nonce: "response-nonce"})},
		{"sidebar-english", sidebar(NewSidebar(model, current), layouts.Chrome{Nonce: "response-nonce", Lang: wording.En})},
		{"note-page", Note(recordedNoteView(model, current), recordedChrome())},
		{"syllabus-page", Syllabus(recordedPathView(model), recordedChrome())},
		{"home-page", Home(recordedHomeView(model), recordedChrome())},
		{"home-page-withheld", Home(recordedWithheldHomeView(model), recordedChrome())},
		{"health-page", Health(recordedHealthView(model), recordedChrome())},
		{"file-page", File(recordedFileView(model), recordedChrome())},
		{"folder-page", Folder(recordedFolderView(model), recordedChrome())},
		{"notfound-page", NotFound(NotFoundView{Asked: "/notes/Nobody/wrote.md", Sidebar: NewSidebar(model, "")}, recordedChrome())},
		{"recovery-page", StatusRecovery(recordedRecoveryView(model), recordedChrome())},
		{"search-page", Search(recordedSearchView(model), recordedChrome())},
		{"search-results-english", SearchResults(recordedSearchView(model), wording.En)},
		{"report-page", Report(ReportView{Name: "2026-07-10.html", Sidebar: NewSidebar(model, ""), NeedsScript: true}, recordedChrome())},
		{"path-index-page", ListIndex(NewPathIndex(model.Paths(), recordedChrome().Lang), recordedChrome())},
		{"map-index-page", ListIndex(NewMapIndex(model.Maps(), recordedChrome().Lang), recordedChrome())},
		{"report-index-page", ListIndex(recordedReportIndexView(), recordedChrome())},
		{"withheld-index-page", ListIndex(recordedWithheldIndexView(), recordedChrome())},
		{"folder-index-page", FolderIndex(NewFolderIndex(model, recordedChrome().Lang), RecentBlock{}, StatusDistribution{}, recordedChrome())},
		{"folder-index-shelf", FolderIndex(shelfIndex, shelfRecent, shelfStatuses, recordedChrome())},
	}
	for _, state := range recordedStatusStates() {
		cases = append(cases,
			surface{"statuspanel-" + state.name, statusPanel(state.view, wording.ZhHant)},
			surface{"statusbar-" + state.name, statusBar(state.view, wording.ZhHant)},
		)
	}
	if len(cases) < 30 {
		t.Fatalf("only %d surfaces are recorded, so this test locks almost nothing", len(cases))
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := tt.component.Render(t.Context(), &buf); err != nil {
				t.Fatalf("render %s: %v", tt.name, err)
			}
			path := filepath.Join("testdata", "render", tt.name+".html")
			if *updateRenderBytes {
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
					t.Fatalf("WriteFile(%s): %v", path, err)
				}
				return
			}
			want, err := os.ReadFile(path) // #nosec G304 -- a recording name from this test's own table
			if err != nil {
				t.Fatalf("ReadFile(%s): %v — rerun with -update-render-bytes to record it", path, err)
			}
			// A blank recording still catches a surface that starts drawing, but
			// it catches nothing about one that stopped, so the two surfaces
			// that are meant to draw nothing are named and every other blank is
			// a fixture that quietly reached no markup at all.
			if len(want) == 0 && !drawsNothing[tt.name] {
				t.Fatalf("%s recorded no bytes; its fixture reaches no markup, so the file locks nothing", tt.name)
			}
			if diff := cmp.Diff(string(want), buf.String()); diff != "" {
				t.Errorf("%s bytes moved (-recorded +rendered):\n%s", tt.name, diff)
			}
		})
	}
}

// drawsNothing names the recordings that are meant to be empty. Both status
// faces stay away from an ungoverned folder, which has no lifecycle to control,
// and from a note whose frontmatter could not be read, where no status was
// parsed to act on.
//
// The rail panel's two entries used to hold a full panel each, byte for byte
// the same as one another, because the condition that keeps it away lived in
// its caller: the recording drew a component the page never draws in those
// states, so it locked nothing and could not tell the two apart. The condition
// is the panel's own now, as it always was the bar's.
var drawsNothing = map[string]bool{
	"statusbar-ungoverned":               true,
	"statusbar-frontmatter-diagnostic":   true,
	"statuspanel-ungoverned":             true,
	"statuspanel-frontmatter-diagnostic": true,
}

// recordedChrome is one fixed request's chrome, so the recording says nothing
// about the machine it was made on.
func recordedChrome() layouts.Chrome {
	return layouts.Chrome{
		Title:                     "L01",
		Nonce:                     "response-nonce",
		Theme:                     "light",
		Ruby:                      "on",
		TextSize:                  "m",
		HasRuby:                   true,
		SingleKeyShortcutsEnabled: true,
		Lang:                      wording.ZhHant,
	}
}

// recordedNoteView is a reading page carrying one of everything the page can
// draw — every aid, a diagnostic of each shape, a schema notice, a receipt, and
// a live write face — so the recording covers branches a narrower fixture would
// leave unwritten.
func recordedNoteView(model *nav.Model, current string) NoteView {
	return NoteView{
		Title:           "L01",
		RelPath:         current,
		Language:        "ja",
		Type:            "lesson",
		Status:          "draft",
		ObsidianHref:    ObsidianHref("/vault", current),
		Unsearchable:    true,
		Updated:         "2026-07-10",
		UpdatedAt:       "2026-07-10",
		UpdatedFromFile: true,
		// The prose carries both shapes a wikilink can take, because the page
		// treats them differently: the resolved one is a link a hover card can
		// be opened on, and the degraded one is not, so a recording holding
		// only one of them would lock only half the decision.
		BodyHTML: `<p>body with <a href="/notes/Concepts/go/C01.md" class="wikilink">C01</a>` +
			` and <a href="/notes/Concepts/go/C02.md" class="wikilink wikilink-degraded" title="no such section">C02</a></p>`,
		TitleAnchor: "l01",
		RenderDiagnostics: []render.Diagnostic{
			{Kind: render.DiagWikilinkBroken, Target: "Ghost", Section: "Part"},
			{Kind: render.DiagRenderFailed, Message: "boom"},
			{Kind: render.DiagnosticKind("kind-nobody-named")},
		},
		Prev:                nav.NoteRef{Name: "L00", RelPath: "Writing/lessons/go/L00.md"},
		Next:                nav.NoteRef{Name: "L02", RelPath: "Writing/lessons/go/L02.md"},
		StepsLabel:          "Go 課程順序",
		StepsCourse:         true,
		VaultHasLinks:       true,
		CitedBy:             []nav.NoteRef{{Name: "C01", RelPath: "Concepts/go/C01.md"}},
		TOC:                 []render.TOCEntry{{ID: "h1", Level: 2, Text: "第一節"}},
		Sidebar:             NewSidebar(model, current),
		Governed:            true,
		Transitions:         []Transition{{To: "ready"}, {To: "archived", NoReturn: true}},
		ContentIdentity:     "abc123",
		TranscludedIdentity: "def456",
		FlippedFrom:         "seed",
		FlipNoReturn:        true,
		SchemaNotices: [][]wording.SchemaPart{
			wording.SchemaSentence(wording.ZhHant, "schema.enum", "status", "seed", "Writing"),
		},
		Concepts: []lesson.ConceptDoc{{ID: "c1", Title: "助詞", HTML: "<p>concept</p>"}},
	}
}

// recordedPathView reads the fixture folder's own course through the same
// builder the page uses, so the recording is of an interpretation rather than
// of a tree typed out beside it.
func recordedPathView(model *nav.Model) PathView {
	current := model.Path("Maps/Go path.md")
	return BuildPathView(current, model.Paths())
}

// recordedStatusStates names every state the write face can be in. The two
// faces draw the same set, which is the thing worth recording: a change that
// moves one of them and not the other shows up as a difference between two
// files here.
func recordedStatusStates() []struct {
	name string
	view NoteView
} {
	base := NoteView{RelPath: "Writing/lessons/go/L01.md", Governed: true, ContentIdentity: "abc123"}
	with := func(mutate func(v *NoteView)) NoteView {
		v := base
		mutate(&v)
		return v
	}
	return []struct {
		name string
		view NoteView
	}{
		{"non-instance", with(func(v *NoteView) { v.NonInstance = true })},
		{"write-diagnostic", with(func(v *NoteView) { v.WriteDiagnostic = "contract unreadable"; v.Status = "draft" })},
		{"no-frontmatter", with(func(v *NoteView) { v.NoFrontmatter = true })},
		{"status-unknown", with(func(v *NoteView) { v.Status = "seed"; v.StatusUnknown = true })},
		{"status-unreadable", with(func(v *NoteView) { v.ObsidianHref = "obsidian://open?path=x" })},
		{"no-transitions", with(func(v *NoteView) { v.Status = "published" })},
		{"transitions", with(func(v *NoteView) {
			v.Status = "draft"
			v.Transitions = []Transition{{To: "ready"}, {To: "archived", NoReturn: true}}
		})},
		{"ungoverned", with(func(v *NoteView) { v.Governed = false; v.Status = "draft" })},
		{"frontmatter-diagnostic", with(func(v *NoteView) { v.Diagnostic = "yaml: line 2"; v.Status = "draft" })},
		{"schema-notices", with(func(v *NoteView) {
			v.Status = "draft"
			v.Transitions = []Transition{{To: "ready"}}
			v.SchemaNotices = [][]wording.SchemaPart{
				wording.SchemaSentence(wording.ZhHant, "schema.required", "slug", "", ""),
			}
		})},
	}
}

// The remaining page entry points, each carrying enough to reach the blocks it
// draws conditionally. They are recorded for the same reason the reading page
// is: their own helpers moved out of their templates.

func recordedHomeView(model *nav.Model) HomeView {
	return HomeView{
		Fault:          "",
		PrivacyFault:   "the contract declares no privacy scope",
		Degraded:       "有檔案讀不進來",
		DegradedDetail: "permission denied",
		Blocks:         NewDeskBlocks(model, recordedChrome().Lang),
		ReadmeMissing:  true,
	}
}

// recordedWithheldHomeView is the desk over a vault whose contract could not be
// read. The builder's own lock is the withheld check in the command's route
// tests, which drives the real site over a real broken contract; this records
// what the markup then looks like.
//
// The two contract-derived blocks are still drawn, because they are the
// only route to their pages, and they say neither how much they hold nor that
// they hold nothing; the reason sits below the seam. Recording it is what makes
// a block that starts speaking for a declaration nobody could read visible in a
// diff — that it stays quiet is asserted against the running site elsewhere.
func recordedWithheldHomeView(model *nav.Model) HomeView {
	view := recordedHomeView(model)
	view.Fault = "the contract could not be read"
	view.Blocks = NewDeskBlocks(model, recordedChrome().Lang)
	for i := range view.Blocks {
		if view.Blocks[i].Mode == pathMode || view.Blocks[i].Mode == mapMode {
			withhold(&view.Blocks[i].Shelf)
			view.Blocks[i].Shelf.Rows = nil
		}
	}
	return view
}

// recordedShelfView is the folder index carrying the two shelf blocks, which
// only a governed vault with a readable distribution fills.
func recordedShelfView(model *nav.Model) (ListIndexView, RecentBlock, StatusDistribution) {
	recent := NewRecentBlock([]HomeNote{
		{Title: "L01", RelPath: "Writing/lessons/go/L01.md", Type: "lesson", Status: "draft", Modified: "2026-07-10", ModifiedAt: "2026-07-10"},
		{Title: "C01", RelPath: "Concepts/go/C01.md", Type: "concept", Status: "seed", Modified: "2026-07-09", ModifiedAt: "2026-07-09"},
	}, true, true, recordedChrome().Lang)
	return NewFolderIndex(model, recordedChrome().Lang), recent, StatusDistribution{
		Statuses: []LifecycleItem{
			{Name: "draft", Count: 2, Href: statusHref("draft")},
			{Name: "ready", Count: 1, Sealed: true, Href: statusHref("ready")},
		},
		Unstated: []LifecycleItem{{Count: 1, Unknown: true, Label: "沒有寫狀態"}},
	}
}

// recordedReportIndexView carries both kinds of report the vault holds and the
// newest mark, none of which the shared fixture vault has.
func recordedReportIndexView() ListIndexView {
	return NewReportIndex([]nav.Report{
		{Name: "2026-07-10 vault audit.md", RelPath: "System/reports/2026-07-10 vault audit.md"},
		{Name: "notes on the scan.md", RelPath: "System/reports/notes on the scan.md"},
		{Name: "latest.html", RelPath: "System/reports/daily-briefing/latest.html", Briefing: true, Latest: true},
	}, recordedChrome().Lang)
}

// recordedWithheldIndexView is a mode index whose declaration could not be
// read: no rows, and the reason in place of the sentence about a vault that
// declared nothing.
func recordedWithheldIndexView() ListIndexView {
	view := NewMapIndex(nil, recordedChrome().Lang)
	view.Fault = "the contract could not be read"
	return view
}

func recordedHealthView(model *nav.Model) HealthView {
	ref := nav.NoteRef{Name: "L01", RelPath: "Writing/lessons/go/L01.md"}
	return HealthView{
		Unwritten:             []HealthLink{{From: ref, Target: "Ghost"}},
		TitleOnly:             []HealthTitleLink{{From: ref, Target: "L02"}},
		Islands:               []HealthIslandGroup{{Dir: "Concepts/go", Notes: []nav.NoteRef{{Name: "C02", RelPath: "Concepts/go/C02.md"}}}},
		IslandCount:           1,
		Collisions:            []HealthCollision{{Name: "Repeat", Candidates: []nav.NoteRef{{Name: "Repeat", RelPath: "A/Repeat.md"}, {Name: "Repeat", RelPath: "B/Repeat.md"}}}},
		Blocked:               []HealthBlockedSource{{Path: "Sources/articles/Raw.md", Reason: "permission denied"}},
		StatusOutsideEnum:     []HealthStatusNote{{Note: ref, Type: "lesson", Status: "seed"}},
		FrontmatterUnreadable: []nav.NoteRef{ref},
		SchemaFaults:          []nav.NoteRef{ref},
		Sidebar:               NewSidebar(model, ""),
	}
}

func recordedFileView(model *nav.Model) FileView {
	return FileView{
		Kind:        FileInfo,
		Title:       "notes.csv",
		RelPath:     "Sources/notes.csv",
		Size:        1234567,
		ContentType: "text/csv",
		Sidebar:     NewSidebar(model, ""),
	}
}

func recordedFolderView(model *nav.Model) FolderView {
	return FolderView{
		Dir:        "Writing/lessons",
		Name:       "lessons",
		Crumbs:     Breadcrumb("Writing/lessons/go/L01.md"),
		Subfolders: []nav.NoteRef{{Name: "go", RelPath: "Writing/lessons/go"}},
		Notes:      []nav.NoteRef{{Name: "L01", RelPath: "Writing/lessons/go/L01.md"}},
		Sidebar:    NewSidebar(model, "Writing/lessons/go/L01.md"),
	}
}

func recordedRecoveryView(model *nav.Model) StatusRecoveryView {
	return StatusRecoveryView{
		Changed:         true,
		Summary:         "摘要",
		NextAction:      "下一步",
		TechnicalDetail: "rename: file exists",
		NotePath:        "Writing/lessons/go/L01.md",
		NoteIdentity:    "abc123",
		ObsidianHref:    ObsidianHref("/vault", "Writing/lessons/go/L01.md"),
		Sidebar:         NewSidebar(model, "Writing/lessons/go/L01.md"),
	}
}

func recordedSearchView(model *nav.Model) SearchView {
	return SearchView{
		Query: "kafka",
		Results: []SearchResult{{
			RelPath:     "Writing/lessons/go/L01.md",
			Title:       "L01",
			Status:      "draft",
			SnippetRuns: []SnippetRun{{Text: "before "}, {Text: "kafka", Hit: true}, {Text: " after"}},
			PathRuns:    []SnippetRun{{Text: "Writing/"}, {Text: "lessons", Hit: true}},
			AliasRuns:   []SnippetRun{{Text: "another name"}},
		}, {
			// A match with words on either side of it: the row that shows what
			// the marked stretches do to the string they were cut from, which a
			// match at the end of a path cannot show.
			RelPath:     "Writing/lessons/go/Reading kafka from source.md",
			Title:       "Reading kafka from source",
			Status:      "ready",
			SnippetRuns: []SnippetRun{{Text: "the "}, {Text: "kafka", Hit: true}, {Text: " reader keeps its offsets"}},
			PathRuns:    []SnippetRun{{Text: "Writing/lessons/go/Reading "}, {Text: "kafka", Hit: true}, {Text: " from source.md"}},
		}},
		Total:             3,
		UnknownFilterKeys: []string{"tag", "kind"},
		FilterKeys:        []string{"status", "type", "path"},
		StepBacks:         []SearchStepBack{{Query: "kafka", Count: 2}},
		Governed:          true,
		Sidebar:           NewSidebar(model, ""),
	}
}
