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

	type surface struct {
		name      string
		component templ.Component
	}
	cases := []surface{
		{"sidebar-current-note", sidebar(NewSidebar(model, current, wording.ZhHant), "response-nonce")},
		{"sidebar-no-note", sidebar(NewSidebar(model, "", wording.ZhHant), "response-nonce")},
		{"sidebar-english", sidebar(NewSidebar(model, current, wording.En), "response-nonce")},
		{"note-page", Note(recordedNoteView(model, current), recordedChrome())},
		{"syllabus-page", Syllabus(recordedPathView(model), recordedChrome())},
	}
	for _, state := range recordedStatusStates() {
		cases = append(cases,
			surface{"statuspanel-" + state.name, statusPanel(state.view)},
			surface{"statusbar-" + state.name, statusBar(state.view)},
		)
	}
	if len(cases) < 15 {
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

// drawsNothing names the recordings that are meant to be empty. The bottom bar
// stays away from an ungoverned folder, which has no lifecycle to control, and
// from a note whose frontmatter could not be read, where no status was parsed
// to act on.
var drawsNothing = map[string]bool{
	"statusbar-ungoverned":             true,
	"statusbar-frontmatter-diagnostic": true,
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
		Lang:            wording.ZhHant,
		Language:        "ja",
		Type:            "lesson",
		Status:          "draft",
		ObsidianHref:    ObsidianHref("/vault", current),
		Unsearchable:    true,
		Updated:         "2026-07-10",
		UpdatedAt:       "2026-07-10",
		UpdatedFromFile: true,
		BodyHTML:        "<p>body</p>",
		TitleAnchor:     "l01",
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
		Sidebar:             NewSidebar(model, current, wording.ZhHant),
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
	base := NoteView{Lang: wording.ZhHant, RelPath: "Writing/lessons/go/L01.md", Governed: true, ContentIdentity: "abc123"}
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
