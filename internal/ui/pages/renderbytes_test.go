package pages

import (
	"bytes"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/lesson"
	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/sequence"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// updateRenderBytes rewrites the recorded HTML instead of comparing against it.
// Regenerating is a decision about what the pages emit, so it is an explicit
// action rather than something a failing run does on its own.
var updateRenderBytes = flag.Bool("update-render-bytes", false, "rewrite the recorded page bytes")

func TestRecordedFilterKeys(t *testing.T) {
	t.Parallel()
	if *updateRenderBytes {
		t.Skip("recordings are being updated; verify them in a separate read-only run")
	}
	for _, name := range []string{"search-page", "search-page-unasked", "search-results-english"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join("testdata", "render", name+".html")) // #nosec G304 -- a recording name from this test's fixed local table
			if err != nil {
				t.Fatalf("ReadFile(%q) error = %v", name, err)
			}
			var got []string
			for _, match := range regexp.MustCompile(`<code>([^<]*):</code>`).FindAllSubmatch(data, -1) {
				got = append(got, string(match[1]))
			}
			slices.Sort(got)
			got = slices.Compact(got)
			if diff := cmp.Diff(lexical.FilterKeys(), got); diff != "" {
				t.Errorf("recorded filter keys (-grammar +recording):\n%s", diff)
			}
		})
	}
}

// TestRecordedNoteWikilinks keeps the recording's resolved and degraded links
// faithful to what a reader sees and hears, even when the bytes are re-recorded.
func TestRecordedNoteWikilinks(t *testing.T) {
	t.Parallel()
	if *updateRenderBytes {
		t.Skip("recordings are being updated; verify them in a separate read-only run")
	}
	recorded, err := os.ReadFile("testdata/render/note-page.html")
	if err != nil {
		t.Fatalf("ReadFile(note-page.html): %v", err)
	}
	links := regexp.MustCompile(`(?s)<a\b[^>]*\bclass="wikilink(?: [^"]*)?"[^>]*>.*?</a>`)
	want := []string{
		`<a href="/notes/Concepts/go/C01.md" class="wikilink">C01</a>`,
		`<a href="/notes/Concepts/go/C02.md#missing" class="wikilink wikilink-degraded" title="找不到「Missing」這個小節，連結會落在筆記最上方">C02<span class="y-offscreen">（找不到「Missing」這個小節，連結會落在筆記最上方）</span></a>`,
	}
	if diff := cmp.Diff(want, links.FindAllString(string(recorded), -1)); diff != "" {
		t.Errorf("recorded note wikilinks mismatch (-want +got):\n%s", diff)
	}
}

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
		{"sidebar-current-note", sidebar(NewSidebar(recordedShell(model), current), layouts.Chrome{Nonce: "response-nonce"})},
		{"sidebar-no-note", sidebar(NewSidebar(nav.Shell{Nav: model}, ""), layouts.Chrome{Nonce: "response-nonce"})},
		{"sidebar-english", sidebar(NewSidebar(recordedShell(model), current), layouts.Chrome{Nonce: "response-nonce", Lang: wording.En})},
		{"note-page", Note(recordedNoteView(t, model, current), recordedChrome())},
		// The head's dt/dd facts read in the other interface language too: the
		// terms are the interface's own words and only a second recording shows
		// neither language's spelling was left behind in the pair.
		{"note-page-english", Note(recordedNoteView(t, model, current), recordedEnglishChrome())},
		{"syllabus-page", Syllabus(recordedPathView(model), recordedChrome())},
		// The course in the other language it is read in. The rows are the
		// vault's own words either way; what changes is everything the page
		// says around them, and the page's shape must survive the longer
		// words rather than only the ones it was drawn with.
		{"syllabus-page-english", Syllabus(recordedPathView(model), recordedEnglishChrome())},
		// The same course as something to be listened to, in both languages.
		// Its paragraphs are the notes' own read-aloud elements, written out
		// here rather than rendered, so what these files pin is the page and
		// not a second copy of the renderer's bytes.
		{"listen-page", Listen(recordedListenView(), recordedChrome())},
		{"listen-page-english", Listen(recordedListenView(), recordedEnglishChrome())},
		// A course whose lessons mark nothing says so, and grows no bar.
		{"listen-page-silent", Listen(ListenView{Title: "朗讀《Go path》", PathHref: "/syllabus/Maps/Go%20path.md"}, recordedChrome())},
		{"home-page", Home(recordedHomeView(model), recordedChrome())},
		{"home-page-withheld", Home(recordedWithheldHomeView(model), recordedChrome())},
		{"health-page", Health(recordedHealthView(model), recordedChrome())},
		{"health-page-english", Health(recordedHealthView(model), recordedEnglishChrome())},
		{"health-page-unreadable", Health(recordedUnreadableHealthView(t, model), recordedChrome())},
		{"health-page-unreadable-english", Health(recordedUnreadableHealthView(t, model), recordedEnglishChrome())},
		{"file-page", File(recordedFileView(model), recordedChrome())},
		{"folder-page", Folder(recordedFolderView(model), recordedChrome())},
		{"notfound-page", NotFound(NotFoundView{Asked: "/notes/Nobody/wrote.md", Sidebar: NewSidebar(nav.Shell{Nav: model}, "")}, recordedChrome())},
		{"recovery-page", StatusRecovery(recordedRecoveryView(model), recordedChrome())},
		{"search-page", Search(recordedSearchView(model, recordedChrome().Lang), recordedChrome())},
		{"search-page-unasked", Search(SearchView{FilterKeys: lexical.FilterKeys()}, recordedChrome())},
		{"search-results-english", SearchResults(recordedSearchView(model, wording.En), wording.En)},
		{"report-page", Report(ReportView{Name: "2026-07-10.html", ReadingRail: NewReportReadingRail(recordedShell(model), "System/reports/daily-briefing/2026-07-10.html"), NeedsScript: true}, recordedChrome())},
		{"preferences-page", Preferences(recordedPreferencesView(wording.ZhHant), recordedChrome())},
		// The field legends are drawn in the label face now rather than sitting
		// inside a bordered box, and that face is where an untranslated legend
		// would be easiest to miss — recorded in English too so a fixture
		// carrying the wrong language's words shows up as a byte, not a guess.
		{"preferences-page-english", Preferences(recordedPreferencesView(wording.En), recordedEnglishChrome())},
		{"path-index-page", ListIndex(NewPathIndex(model.Paths(), schema.NavigationRoles{}, nav.Closure{}, ContractGoverning, recordedChrome().Lang, nil), recordedChrome())},
		{"map-index-page", ListIndex(NewMapIndex(model.Maps(), schema.NavigationRoles{}, nav.Closure{}, ContractGoverning, recordedChrome().Lang, nil), recordedChrome())},
		{"report-index-page", ListIndex(recordedReportIndexView(recordedChrome().Lang), recordedChrome())},
		// The same shelf in the other language it is read in. The day is the
		// vault's own either way; the two answers that are not a day — the
		// briefing kept current, and the report that wrote none — are the
		// interface's words, and only a recording in both languages shows
		// neither was left behind in one of them.
		{"report-index-page-english", ListIndex(recordedReportIndexView(recordedEnglishChrome().Lang), recordedEnglishChrome())},
		{"withheld-index-page", ListIndex(recordedWithheldIndexView(), recordedChrome())},
		{"withheld-index-page-silent", ListIndex(recordedSilentlyWithheldIndexView(), recordedChrome())},
		{"folder-index-fault-head", ListIndex(recordedFaultedModeIndexView(model), recordedChrome())},
		{"path-index-page-fault", ListIndex(recordedFaultedIndexView(), recordedChrome())},
		{"folder-index-page", FolderIndex(NewFolderIndex(model, ContractGoverning, recordedChrome().Lang, nil), RecentBlock{}, StatusDistribution{}, recordedChrome())},
		{"folder-index-shelf", FolderIndex(shelfIndex, shelfRecent, shelfStatuses, recordedChrome())},
		// The surfaces that have nothing to show, in both languages. Each of
		// them is a branch the recordings above never reach — the search fixture
		// finds notes, the health fixture has faults, and the desk fixture fills
		// its shelves — so without these the one notice they share is written
		// into a diff nobody can read back. Both languages, because the notice
		// is one component for either of them and a sentence that fits in only
		// one is a layout fault the Chinese recording alone cannot show.
		{"search-page-empty", Search(recordedNothingFoundView(model), recordedChrome())},
		{"search-page-empty-english", Search(recordedNothingFoundView(model), recordedEnglishChrome())},
		{"notfound-page-english", NotFound(NotFoundView{Asked: "/notes/Nobody/wrote.md", Sidebar: NewSidebar(nav.Shell{Nav: model}, "")}, recordedEnglishChrome())},
		{"notfound-page-unreadable", NotFound(NotFoundView{Asked: "/notes/Locked/away.md", Unreadable: true, Sidebar: NewSidebar(nav.Shell{Nav: model}, "")}, recordedChrome())},
		{"health-page-clear", Health(recordedClearHealthView(model), recordedChrome())},
		{"health-page-clear-english", Health(recordedClearHealthView(model), recordedEnglishChrome())},
		{"home-page-empty", Home(recordedNothingHomeView(wording.ZhHant), recordedChrome())},
		{"home-page-empty-english", Home(recordedNothingHomeView(wording.En), recordedEnglishChrome())},
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

// recordedNothingFoundView is a search that matched nothing. It carries the
// loosened searches and the lifecycle advice, which is everything the page has
// to offer at that moment and the part a narrower fixture leaves unwritten.
func recordedNothingFoundView(model *nav.Model) SearchView {
	return SearchView{
		Query:      "kafka",
		FilterKeys: lexical.FilterKeys(),
		StepBacks:  []SearchStepBack{{Query: "kaf", Count: 2}},
		Governed:   true,
		Sidebar:    NewSidebar(nav.Shell{Nav: model}, ""),
	}
}

// recordedClearHealthView is the folder with nothing left to report. Every list
// the page reads is empty, which is the one state the faulted recording beside
// it can never reach.
func recordedClearHealthView(model *nav.Model) HealthView {
	return HealthView{Sidebar: NewSidebar(nav.Shell{Nav: model}, "")}
}

// recordedNothingHomeView is the desk with nothing in the two shelves a
// declaration fills and both notices about a reading that came up short. The
// shelves are written out here rather than built from a model: what this
// recording holds is the markup an unfilled shelf turns into, and reaching
// through the builders would put their own arithmetic under the recording too.
//
// The two sentences differ on purpose. A shelf whose contract declares one type
// names it; a shelf with several names the list, and the two are separate
// sentences in both languages.
func recordedNothingHomeView(lang wording.Lang) HomeView {
	unfilled := func(mode, title, href, count, lede, empty string) DeskBlock {
		return DeskBlock{Mode: mode, Shelf: Shelf{Title: title, Href: href, Count: count, Lede: lede, Empty: empty}}
	}
	// The several-types sentence is given its list the way the page gives it
	// one: the types the contract declared, joined by the separator this
	// interface writes inside a sentence.
	declaredTypes := []string{"concept", "map"}
	return HomeView{
		PrivacyFault:   `never_egress_dirs = ["/"]`,
		Degraded:       fmt.Sprintf(wording.DegradedNoticeOne.In(lang), 1),
		DegradedDetail: "Sources/articles/Raw.md: permission denied",
		Blocks: []DeskBlock{
			unfilled(pathMode, wording.Paths.In(lang), "/paths",
				plural(0, wording.PathCountOne, wording.PathCountMany, lang),
				wording.DeskPathsLede.In(lang),
				fmt.Sprintf(wording.NoDeclaredTypeEmptyFmt.In(lang), "lesson")),
			unfilled(mapMode, wording.Maps.In(lang), "/maps",
				plural(0, wording.MapCountOne, wording.MapCountMany, lang),
				wording.DeskMapsLede.In(lang),
				fmt.Sprintf(wording.NoDeclaredTypesEmptyFmt.In(lang),
					strings.Join(declaredTypes, wording.ListSeparator.In(lang)))),
		},
		ReadmeMissing: true,
	}
}

// recordedPreferencesView is one page's worth of choices, written out here
// rather than built by the endpoint that assembles the real one. What this
// recording locks is the markup a set of choices turns into — the fieldset, the
// described note, the hidden return address, the visible submit — and reaching
// through the endpoint to get it would put the assembly under the recording too,
// where a moved label and a moved tag would look like one change.
//
// The legend and note text come from the wording package rather than sitting
// here as literals, so the English recording carries the English words a
// reader of that page actually sees — the uppercase mono legends are the part
// a language switch could silently leave in Chinese, the way an untranslated
// facet heading once did.
func recordedPreferencesView(lang wording.Lang) PreferencesView {
	return PreferencesView{
		ReturnTo: "/notes/Writing/lessons/go/L01.md",
		Settings: "/preferences?from=%2Fnotes%2FWriting%2Flessons%2Fgo%2FL01.md",
		Reading:  "/notes/Writing/lessons/go/L01.md",
		Fields: []PreferenceField{
			{
				Name:    "theme",
				Legend:  wording.PrefAppearance.In(lang),
				Note:    wording.PrefAppearanceNote.In(lang),
				Refused: wording.PrefSaveRefused.In(lang),
				// Following the system is the one option a cookie cannot
				// carry, so the recording keeps a choice whose marks differ
				// across its options — one that stores nothing beside two
				// that do.
				Options: []PreferenceOption{
					{Value: "system", Label: wording.PrefAppearanceSystem.In(lang), Checked: true, Unset: true},
					{Value: "light", Label: wording.PrefAppearanceLight.In(lang), Stores: true},
					{Value: "dark", Label: wording.PrefAppearanceDark.In(lang), Stores: true},
				},
			},
			{
				Name:    "ruby",
				Legend:  wording.PrefFurigana.In(lang),
				Note:    wording.PrefFuriganaNote.In(lang),
				Refused: wording.PrefSaveRefused.In(lang),
				Options: []PreferenceOption{
					{Value: "on", Label: wording.PrefOn.In(lang), Checked: true, Stores: true, Unset: true},
					{Value: "off", Label: wording.PrefOff.In(lang), Stores: true},
				},
			},
		},
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

// recordedShell is the fixture folder handed over the way a request receives
// it, carrying a stated vault so the recordings hold the foot of the rail with
// its three lines filled. The zero shell is recorded too — the rail with no
// current note keeps it — so both the stated and the unstated wording are
// pinned, and neither can go blank without a recording moving.
func recordedShell(model *nav.Model) nav.Shell {
	return nav.Shell{
		Nav:   model,
		Vault: nav.Vault{Name: "example-vault", Notes: 12, Findings: 3},
	}
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
		SingleKeyShortcutsEnabled: true,
		Lang:                      wording.ZhHant,
	}
}

// recordedEnglishChrome is the same chrome in the other interface language.
// The findings table names each of its columns twice — once in the header a
// reader clicks, once on every cell so a stacked row still says what it holds —
// and both are drawn from the interface's words, so only a recording in both
// languages can show that neither spelling was left behind in one of them.
func recordedEnglishChrome() layouts.Chrome {
	chrome := recordedChrome()
	chrome.Lang = wording.En
	return chrome
}

// recordedNoteView is a reading page carrying one of everything the page can
// draw — every aid, a diagnostic of each shape, a schema notice, a receipt, and
// a live write face — so the recording covers branches a narrower fixture would
// leave unwritten.
func recordedNoteView(t *testing.T, model *nav.Model, current string) NoteView {
	t.Helper()
	root := t.TempDir()
	const body = "body with [[C01]] and [[C02#Missing|C02]]\n"
	for rel, content := range map[string]string{
		current:              body,
		"Concepts/go/C01.md": "# C01\n",
		"Concepts/go/C02.md": "# C02\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", rel, err)
		}
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("reader.Close: %v", closeErr)
		}
	})
	store, err := snapshot.New(t.Context(), reader, slog.New(slog.DiscardHandler), nil, schema.Ungoverned())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	return NoteView{
		Title:           "L01",
		RelPath:         current,
		Language:        "ja",
		Type:            "lesson",
		Status:          "draft",
		ObsidianHref:    ObsidianHref("/vault", current),
		Updated:         "2026-07-10",
		UpdatedAt:       "2026-07-10",
		UpdatedFromFile: true,
		// Render both a resolved link and a missing section, so the recording
		// follows the renderer's address, tooltip, and offscreen explanation.
		BodyHTML:    store.Current().Render(current, body, recordedChrome().Lang).HTML,
		TitleAnchor: "l01",
		RenderDiagnostics: []render.Diagnostic{
			{Kind: render.DiagWikilinkBroken, Target: "Ghost", Section: "Part"},
			{Kind: render.DiagRenderFailed, Message: "boom"},
			{Kind: render.DiagnosticKind("kind-nobody-named")},
		},
		Prev:                nav.NoteRef{Name: "L00", RelPath: "Writing/lessons/go/L00.md"},
		Next:                nav.NoteRef{Name: "L02", RelPath: "Writing/lessons/go/L02.md"},
		StepsLabel:          "Go 從此步往下",
		StepsCourse:         true,
		VaultHasLinks:       true,
		CitedBy:             []nav.NoteRef{{Name: "C01", RelPath: "Concepts/go/C01.md"}},
		BasedOn:             []nav.NoteRef{{Name: "Book notes", RelPath: "Book notes.md"}, {Name: "[[twin]]"}},
		TOC:                 []render.TOCEntry{{ID: "h1", Level: 2, Text: "第一節"}},
		ReadingRail:         NewReadingRail(recordedShell(model), current, "golang"),
		Governed:            true,
		Transitions:         []Transition{{To: "ready"}, {To: "archived", NoReturn: true}},
		ContentIdentity:     "abc123",
		MarkAddress:         "/marks",
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
//
// The course is entered from a lesson, so the recording holds the mark for
// where the reader is standing. The fixture course lists that lesson twice, in
// two different parts, which is the case a course reached from one of them has
// to answer: both rows are that lesson and both are marked.
// recordedListenView is a course of two lessons, the second marking one
// paragraph and the first two, so the recording covers both a lesson boundary
// and a lesson carrying more than one paragraph. The paragraphs are the bytes
// render.InjectTTS produces, written out rather than produced here: this file
// records what the page does with them.
func recordedListenView() ListenView {
	reading := func(spoken, inner string) string {
		return `<div class="y-reading" lang="ja"><button class="y-tts" type="button" data-tts="` + spoken +
			`" lang="zh-Hant" aria-label="朗讀這段日文"><svg aria-hidden="true"></svg></button><p lang="ja">` + inner + `</p></div>`
	}
	return ListenView{
		Title:    "朗讀《Go path》",
		PathHref: "/syllabus/Maps/Go%20path.md",
		Lessons: []ListenLesson{
			{
				Title: "L01 わたしは学生です",
				Href:  "/notes/Writing/lessons/go/L01.md",
				Paragraphs: []string{
					reading("一つ目の段落です。", "一つ目の段落です。"),
					reading("二つ目の段落です。", "二つ目の段落です。"),
				},
			},
			{
				Title:      "L02 三つの段落",
				Href:       "/notes/Writing/lessons/go/L02.md",
				Paragraphs: []string{reading("三つ目の段落です。", "三つ目の段落です。")},
			},
		},
	}
}

func recordedPathView(model *nav.Model) PathView {
	current := model.Path("Maps/Go path.md")
	view := BuildPathView(current, model.Paths(), "Writing/lessons/go/L01.md")
	view.Vault = recordedShell(model).Vault
	return view
}

// recordedStatusStates names every state the write face can be in. The two
// faces draw the same set, which is the thing worth recording: a change that
// moves one of them and not the other shows up as a difference between two
// files here.
func recordedStatusStates() []struct {
	name string
	view NoteView
} {
	base := NoteView{RelPath: "Writing/lessons/go/L01.md", Governed: true, ContentIdentity: "abc123", MarkAddress: "/marks"}
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
		{"status-not-text", with(func(v *NoteView) { v.StatusNotText = true; v.ObsidianHref = "obsidian://open?path=x" })},
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
		Blocks:         NewDeskBlocks(model, schema.NavigationRoles{}, ContractGoverning, recordedChrome().Lang, nil),
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
	model = model.WithoutInstanceProjections(nav.Close(schema.Rejected("the contract could not be read")))
	view := recordedHomeView(model)
	view.Fault = "the contract could not be read"
	return view
}

// recordedShelfView is the folder index carrying the two shelf blocks, which
// only a governed vault with a readable distribution fills.
func recordedShelfView(model *nav.Model) (ListIndexView, RecentBlock, StatusDistribution) {
	recent := NewRecentBlock([]HomeNote{
		{Title: "L01", RelPath: "Writing/lessons/go/L01.md", Type: "lesson", Status: "draft", Modified: "2026-07-10", ModifiedAt: "2026-07-10"},
		{Title: "C01", RelPath: "Concepts/go/C01.md", Type: "concept", Status: "seed", Modified: "2026-07-09", ModifiedAt: "2026-07-09"},
	}, true, true, recordedChrome().Lang)
	return NewFolderIndex(model, ContractGoverning, recordedChrome().Lang, nil), recent, NewStatusDistribution(
		[]LifecycleItem{
			{Name: "draft", Count: 2, Href: statusHref("draft")},
			{Name: "ready", Count: 1, Sealed: true, Href: statusHref("ready")},
		},
		[]LifecycleItem{{Count: 1, Unknown: true, Label: "沒有寫狀態"}},
		model.KnowledgeScoped(), recordedChrome().Lang,
	)
}

// recordedReportIndexView carries every answer a report row can give in the
// column a reader scans, none of which the shared fixture vault has: a written
// report with a day of its own and the line it opens with, a briefing named
// for the day it covers, the briefing the vault keeps current, and a report
// that wrote no day at all. They are already in the order the shelf puts them,
// newest first, so the recording shows the row and not the sort.
func recordedReportIndexView(lang wording.Lang) ListIndexView {
	return NewReportIndex([]nav.Report{
		{Name: "latest.html", RelPath: "System/reports/daily-briefing/latest.html", Briefing: true, Latest: true},
		{
			Name:    "Vault audit",
			RelPath: "System/reports/2026-07-10 vault audit.md",
			Date:    "2026-07-10",
			Opening: "Four notes went from draft to ready. Nothing was archived.",
		},
		{
			Name:     "2026-07-02 briefing.html",
			RelPath:  "System/reports/daily-briefing/2026-07-02 briefing.html",
			Briefing: true,
			Date:     "2026-07-02",
		},
		{Name: "Notes on the scan", RelPath: "System/reports/notes on the scan.md"},
	}, lang, nil)
}

// recordedWithheldIndexView is a mode index whose declaration could not be
// read: no rows, and the reason in place of the sentence about a vault that
// declared nothing.
// recordedFaultedIndexView is a course whose structure could not be read: it
// lists, it says how much it plans, and it says that figure is not an answer.
// Nothing in the shared fixture draws a row reporting a hole in its own shelf,
// so the mark that says so had no recorded bytes and was free to lose its
// colour to the ordinary measure beside it.
func recordedFaultedIndexView() ListIndexView {
	return NewPathIndex([]nav.Path{{
		Title:       "Unread Course",
		RelPath:     "Maps/unread.md",
		Diagnostics: []sequence.Diagnostic{{Rule: "path.nesting_too_deep", Line: 4, Message: "nested past one level"}},
	}}, schema.NavigationRoles{}, nav.Closure{}, ContractGoverning, recordedChrome().Lang, nil)
}

// recordedSilentlyWithheldIndexView is the state a page can reach without a
// sentence to show for it: the declaration is closed, so nothing may be listed,
// and the closure carried no words. The page says neither how much it holds nor
// that it holds none — the same silence the desk keeps — and nothing recorded
// that until this.
func recordedSilentlyWithheldIndexView() ListIndexView {
	return NewMapIndex(nil, schema.NavigationRoles{}, nav.Close(schema.Rejected("")), ContractGoverning, recordedChrome().Lang, nil)
}

func recordedWithheldIndexView() ListIndexView {
	return NewMapIndex(nil, schema.NavigationRoles{}, nav.Close(schema.Rejected("the contract could not be read")), ContractGoverning, recordedChrome().Lang, nil)
}

// recordedFaultedModeIndexView is a mode index whose fault is stated below an
// intact head — kicker, title, and count still show. internal/note/index.go
// reaches this by setting view.Fault after NewFolderIndex without calling
// withholdListing; no ListIndex constructor produces it on its own.
func recordedFaultedModeIndexView(model *nav.Model) ListIndexView {
	view := NewFolderIndex(model, ContractGoverning, recordedChrome().Lang, nil)
	view.Fault = "artifact unavailable"
	return view
}

// recordedHealthView holds one of every kind of finding, and two of one kind
// about one file: the table folds those into a single counted row, so a
// recording carrying only singletons would never show what that row looks like.
func recordedHealthView(model *nav.Model) HealthView {
	ref := nav.NoteRef{Name: "L01", RelPath: "Writing/lessons/go/L01.md"}
	return HealthView{
		Unwritten: []snapshot.HealthLink{
			{From: ref, Target: "Ghost"},
			{From: ref, Target: "Phantom"},
		},
		TitleOnly:             []snapshot.HealthTitleLink{{From: ref, Target: "L02", Note: nav.NoteRef{Name: "L02", RelPath: "Writing/lessons/go/L02.md"}}},
		Islands:               []HealthIslandGroup{{Dir: "Concepts/go", Name: "Concepts/go", Notes: []nav.NoteRef{{Name: "C02", RelPath: "Concepts/go/C02.md"}}}},
		IslandCount:           1,
		Collisions:            []HealthCollision{{Name: "Repeat", Candidates: []nav.NoteRef{{Name: "A/Repeat.md", RelPath: "A/Repeat.md"}, {Name: "B/Repeat.md", RelPath: "B/Repeat.md"}}}},
		Blocked:               []HealthBlockedSource{{Path: "Sources/articles/Raw.md", Reason: "permission denied"}},
		Skipped:               []HealthSkippedSource{{Path: "Notes/Linked note.md", Reason: "symbolic link"}},
		StatusOutsideEnum:     []HealthStatusNote{{Note: ref, Type: "lesson", Status: "seed"}},
		StatusUnreachable:     []HealthStatusNote{{Note: ref, Type: "concept", Status: "published"}},
		FrontmatterUnreadable: []HealthNoteFindings{{Note: ref, Severity: judge.SeverityError, Count: 1}},
		SchemaFaults:          []HealthNoteFindings{{Note: ref, Severity: judge.SeverityError, Count: 3}},
		// The ordering a request that named none resolves to, which is what a
		// reader arriving at the page is holding.
		Sort:    HealthByFinding,
		Sidebar: NewSidebar(nav.Shell{Nav: model}, ""),
	}
}

// recordedUnreadableHealthView is the page a reading that could not open a file
// draws. The blocked list is not typed out here: the fixture vault is copied,
// one of its notes has every permission taken from it, and what the reading
// then reports about that note is what the recording holds — the path it saw
// and the machine's own words for why it could not be opened. A row invented
// beside the reading would go on looking right the day the reading stopped
// reporting such a file at all.
//
// Nothing else is set. The same sealed folder also has citations that land
// nowhere and notes nothing cites, and gathering those would mean assembling a
// view here the way the request handler assembles one — a second builder,
// whose output is nobody's page. One of every kind is recorded beside this.
func recordedUnreadableHealthView(t *testing.T, model *nav.Model) HealthView {
	t.Helper()
	return HealthView{
		Blocked: sealedVaultBlocked(t),
		Sort:    HealthByFinding,
		Sidebar: NewSidebar(nav.Shell{Nav: model}, ""),
	}
}

// sealedVaultBlocked copies the judging face's own sealed-file fixture, takes
// every permission from the note that fixture exists for, and returns what a
// reading of the copy could not open.
//
// Both ways this can quietly produce nothing are fatal rather than skipped. A
// filesystem that ignores the permission — a container running as root, a
// volume that carries none — leaves a vault with no hole in it, and a recording
// made there would freeze a page about a file that was readable all along. A
// seal the reading never noticed is the same recording by another route, so an
// empty list is refused as well.
func sealedVaultBlocked(t *testing.T) []HealthBlockedSource {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "judge", "testdata", "vault-unreadable"))); err != nil {
		t.Fatalf("copy the sealed-file fixture: %v", err)
	}
	sealed := filepath.Join(root, filepath.FromSlash("Concepts/golang/Sealed.md"))
	if err := os.Chmod(sealed, 0); err != nil {
		t.Fatalf("Chmod(%q, 0): %v", sealed, err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(sealed, 0o600); err != nil && !os.IsNotExist(err) {
			t.Errorf("restore %q: %v", sealed, err)
		}
	})
	if _, err := os.ReadFile(sealed); err == nil { // #nosec G304 -- this test's own copy of a fixture note
		t.Fatal("this process can still read a file it took every permission from, so no reading here has a hole in it to report")
	}

	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("reader.Close: %v", closeErr)
		}
	})
	store, err := snapshot.New(t.Context(), reader, slog.New(slog.DiscardHandler), nil, schema.Ungoverned())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	blocked := store.Current().Freshness().Blocked
	if len(blocked) == 0 {
		t.Fatal("the reading opened every file of a vault with a sealed note, so there is no finding here to record")
	}
	out := make([]HealthBlockedSource, 0, len(blocked))
	for _, source := range blocked {
		out = append(out, HealthBlockedSource{Path: source.Path, Reason: source.Reason})
	}
	return out
}

func recordedFileView(model *nav.Model) FileView {
	return FileView{
		Kind:        FileInfo,
		Title:       "notes.csv",
		RelPath:     "Sources/notes.csv",
		Size:        1234567,
		ContentType: "text/csv",
		Sidebar:     NewSidebar(nav.Shell{Nav: model}, ""),
	}
}

// recordedFolderView is one level with both kinds of row: a folder carrying
// what is under it, and a file that opens onto itself.
func recordedFolderView(*nav.Model) FolderView {
	return NewFolderLevel("Writing/lessons", "lessons",
		[]nav.NoteRef{{Name: "L01", RelPath: "Writing/lessons/L01.md"}},
		[]nav.Folder{{
			Name:    "go",
			RelPath: "Writing/lessons/go",
			Notes: []nav.NoteRef{
				{Name: "L02", RelPath: "Writing/lessons/go/L02.md"},
				{Name: "L03", RelPath: "Writing/lessons/go/L03.md"},
			},
		}},
		recordedChrome().Lang, nil)
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
		Sidebar:         NewSidebar(nav.Shell{Nav: model}, "Writing/lessons/go/L01.md"),
	}
}

// recordedSearchView takes the language because the column's headings and
// its not-stated cell are written before the view leaves the handler, the
// way a result's own status is: everything else on this page is translated
// inside the template, so without this the English recording would show a
// Traditional Chinese column.
func recordedSearchView(model *nav.Model, lang wording.Lang) SearchView {
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
		FilterKeys:        lexical.FilterKeys(),
		StepBacks:         []SearchStepBack{{Query: "kafka", Count: 2}},
		Facets:            recordedSearchFacets(lang),
		Governed:          true,
		Sidebar:           NewSidebar(nav.Shell{Nav: model}, ""),
	}
}

// recordedSearchFacets is one of each row the column can draw: a plain value,
// a value already in the query whose row leads back out of it, a value no
// carrier declares, a value the grammar cannot spell and so cannot offer, and
// the cell standing for the hits that declared nothing. A recording that held
// only the first would leave the other four unrecorded and free to change.
func recordedSearchFacets(lang wording.Lang) []SearchFacet {
	return []SearchFacet{{
		Key:     "status",
		Heading: wording.FacetHeading("status", lang),
		Rows: []SearchFacetRow{
			{Label: "draft", Count: 2, Query: "kafka", Active: true},
			{Label: "ready", Count: 1, Query: "kafka status:ready"},
			{Label: "seedling", Count: 1, Query: "kafka status:seedling", OutsideEnum: true},
		},
	}, {
		Key:     "type",
		Heading: wording.FacetHeading("type", lang),
		Rows: []SearchFacetRow{
			{Label: "lesson", Count: 2, Query: "kafka type:lesson"},
			{Label: wording.FacetUnstated.In(lang), Count: 1, Unstated: true},
		},
	}, {
		Key:     "domain",
		Heading: wording.FacetHeading("domain", lang),
		Rows: []SearchFacetRow{
			// A value the grammar cannot write back: its quote closes a group
			// early, so the query would read as a shorter constraint and a
			// stray word. The row keeps its count and offers no link.
			{Label: `a" b`, Count: 1},
		},
	}}
}
