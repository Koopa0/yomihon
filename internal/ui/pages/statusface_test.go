package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestBothStatusFacesDrawEveryWriteFaceState holds the one thing that made the
// two faces worth folding into a single decision: the panel in the rail and the
// bar along the foot must offer the same state for the same note. Only one of
// them is present at a given width, so a state the other face never learned to
// draw is a reader with the write face's answer nowhere on the page.
//
// The table walks the whole declared set rather than a list typed out beside
// it, so a state added later has no way to arrive uncovered.
func TestBothStatusFacesDrawEveryWriteFaceState(t *testing.T) {
	t.Parallel()

	governed := NoteView{RelPath: "Writing/L01.md", Governed: true, ContentIdentity: "abc123"}
	with := func(mutate func(v *NoteView)) NoteView {
		v := governed
		mutate(&v)
		return v
	}
	tests := map[faceState]struct {
		view  NoteView
		token string
		// mark is text only this state's face writes.
		mark string
	}{
		faceNonInstance: {
			view:  with(func(v *NoteView) { v.NonInstance = true }),
			token: "non-instance",
			mark:  wording.NonInstanceReason.In(wording.ZhHant),
		},
		faceWriteUnavailable: {
			view:  with(func(v *NoteView) { v.Status = "draft"; v.WriteDiagnostic = "contract unreadable" }),
			token: "unavailable",
			mark:  "contract unreadable",
		},
		faceNoFrontmatter: {
			view:  with(func(v *NoteView) { v.NoFrontmatter = true }),
			token: "instance",
			mark:  wording.NoFrontmatter.In(wording.ZhHant),
		},
		faceStatusUnknown: {
			view:  with(func(v *NoteView) { v.Status = "seed"; v.StatusUnknown = true }),
			token: "instance",
			mark:  wording.StatusOutsideList.In(wording.ZhHant),
		},
		faceStatusNotText: {
			view:  with(func(v *NoteView) { v.StatusNotText = true }),
			token: "instance",
			mark:  wording.StatusNotText.In(wording.ZhHant),
		},
		faceStatusUnreadable: {
			view:  with(func(v *NoteView) {}),
			token: "instance",
			mark:  wording.StatusUnreadable.In(wording.ZhHant),
		},
		faceOutsideScope: {
			view: with(func(v *NoteView) {
				v.RelPath = "System/agent-guides/L05.md"
				v.Status = "draft"
				v.OutsideKnowledgeScope = true
			}),
			token: "instance",
			mark:  wording.OutsideKnowledgeScope.In(wording.ZhHant),
		},
		faceNoTransitions: {
			view:  with(func(v *NoteView) { v.Status = "published" }),
			token: "instance",
			mark:  wording.NoLegalTransitions.In(wording.ZhHant),
		},
		faceTransitions: {
			view:  with(func(v *NoteView) { v.Status = "draft"; v.Transitions = []Transition{{To: "ready"}} }),
			token: "instance",
			mark:  `<form class="y-statusform" method="post" action="/status">`,
		},
	}

	for state := faceNonInstance; state <= faceTransitions; state++ {
		tt, covered := tests[state]
		if !covered {
			t.Fatalf("write-face state %d has no fixture, so nothing says either face can draw it", state)
		}
		view := tt.view
		if got := statusFace(&view); got != state {
			t.Errorf("statusFace() = %d for the fixture written for state %d", got, state)
			continue
		}
		if got := state.token(); got != tt.token {
			t.Errorf("state %d stamps data-status-state=%q, want %q", state, got, tt.token)
		}
		for faceName, component := range map[string]templ.Component{
			"the rail panel": statusPanel(view, wording.ZhHant),
			"the foot bar":   statusBar(view, wording.ZhHant),
		} {
			var buf bytes.Buffer
			if err := component.Render(t.Context(), &buf); err != nil {
				t.Fatalf("render %s: %v", faceName, err)
			}
			html := buf.String()
			if !strings.Contains(html, `data-status-state="`+tt.token+`"`) {
				t.Errorf("%s does not stamp data-status-state=%q for state %d; html = %q", faceName, tt.token, state, html)
			}
			if !strings.Contains(html, tt.mark) {
				t.Errorf("%s does not draw state %d — %q is missing; html = %q", faceName, state, tt.mark, html)
			}
		}
	}
	if len(tests) != int(faceTransitions)+1 {
		t.Errorf("the table holds %d fixtures for %d declared states", len(tests), int(faceTransitions)+1)
	}
}

// TestTheWriteFaceStatesOverrideInOneOrder pins which fact wins when several
// are true of the same note. Each fixture below is true of every state after
// the one it names, so a reordering of the decision shows up here rather than
// as a face quietly answering the wrong question.
//
// One pair is deliberately absent: a status outside the contract's list and a
// status nothing could be read from cannot both hold, because an unknown value
// is a value that was read. Their order relative to each other decides nothing
// and is not asserted.
func TestTheWriteFaceStatesOverrideInOneOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		view NoteView
		want faceState
	}{
		{
			name: "a note the folder does not govern says so before anything else",
			view: NoteView{Governed: true, NonInstance: true, WriteDiagnostic: "unreadable", NoFrontmatter: true, OutsideKnowledgeScope: true},
			want: faceNonInstance,
		},
		{
			name: "a write face that could not open outranks what the frontmatter says",
			view: NoteView{Governed: true, WriteDiagnostic: "unreadable", NoFrontmatter: true, Status: "seed", StatusUnknown: true, OutsideKnowledgeScope: true},
			want: faceWriteUnavailable,
		},
		{
			name: "no frontmatter at all outranks the status read out of it",
			view: NoteView{Governed: true, NoFrontmatter: true, OutsideKnowledgeScope: true},
			want: faceNoFrontmatter,
		},
		{
			name: "a status outside the list outranks having no move to offer",
			view: NoteView{Governed: true, Status: "seed", StatusUnknown: true, OutsideKnowledgeScope: true},
			want: faceStatusUnknown,
		},
		{
			name: "a status nothing could be read from outranks having no move to offer",
			view: NoteView{Governed: true, OutsideKnowledgeScope: true},
			want: faceStatusUnreadable,
		},
		{
			name: "the layer that withheld the moves is named before the schema is blamed for them",
			view: NoteView{Governed: true, Status: "draft", OutsideKnowledgeScope: true},
			want: faceOutsideScope,
		},
		{
			name: "a readable status with nowhere to go is its own state",
			view: NoteView{Governed: true, Status: "published"},
			want: faceNoTransitions,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := statusFace(&tt.view); got != tt.want {
				t.Errorf("statusFace() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestTheUngovernedNoticesSpeakTheReadersLanguage holds the two sentences that
// say the lifecycle does not reach this note. One of them used to be resolved
// once at start-up, in the default language, for every reader. Both status
// faces show either, and both are handed the language the page is written in.
func TestTheUngovernedNoticesSpeakTheReadersLanguage(t *testing.T) {
	t.Parallel()

	notices := []struct {
		name   string
		view   NoteView
		phrase wording.Phrase
	}{
		{
			name:   "the folder holds this note outside its lifecycle",
			view:   NoteView{Governed: true, NonInstance: true, RelPath: "System/templates/T.md"},
			phrase: wording.NonInstanceReason,
		},
		{
			name:   "the declared knowledge layer withheld the moves",
			view:   NoteView{Governed: true, Status: "draft", OutsideKnowledgeScope: true, RelPath: "System/agent-guides/L05.md"},
			phrase: wording.OutsideKnowledgeScope,
		},
	}
	for _, notice := range notices {
		if notice.phrase.In(wording.ZhHant) == notice.phrase.In(wording.En) {
			t.Fatalf("%s reads the same in both languages, so nothing below can tell them apart", notice.name)
		}
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			t.Run(notice.name+"/"+string(lang), func(t *testing.T) {
				t.Parallel()
				for faceName, component := range map[string]templ.Component{
					"the rail panel": statusPanel(notice.view, lang),
					"the foot bar":   statusBar(notice.view, lang),
				} {
					var buf bytes.Buffer
					if err := component.Render(t.Context(), &buf); err != nil {
						t.Fatalf("render %s: %v", faceName, err)
					}
					if got := buf.String(); !strings.Contains(got, notice.phrase.In(lang)) {
						t.Errorf("%s does not say why in %q; html = %q", faceName, lang, got)
					}
				}
			})
		}
	}
}
