package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestNoteArticleLanguageComesOnlyFromAuthority holds what the article element
// is allowed to say about the language a note is written in. A tag that
// reached the view is rendered as the note declared it; a view carrying none
// leaves the element with no language of its own, so the language the page is
// written in stands for the article too.
//
// Each expectation is the whole opening tag rather than the attribute alone.
// The absent case differs from the present ones by the space before the
// attribute as much as by the attribute, so an expectation that named only
// `lang="…"` would have no way to state the absent case at all, and one that
// named only the absence would be satisfied by an article that had lost its
// class as readily as by the right one.
func TestNoteArticleLanguageComesOnlyFromAuthority(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "no authority states no language", want: `<article class="y-article">`},
		{name: "Japanese", lang: "ja", want: `<article class="y-article" lang="ja">`},
		{name: "Traditional Chinese", lang: "zh-Hant", want: `<article class="y-article" lang="zh-Hant">`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Note(NoteView{Language: tt.lang}, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if html := buf.String(); !strings.Contains(html, tt.want) {
				t.Errorf("article opening tag = missing %q in %q", tt.want, html)
			}
		})
	}
}

func TestInlineReadingAidsUseChromeLanguage(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	view := NoteView{
		Language:   "ja",
		Diagnostic: "invalid frontmatter",
	}
	if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	if html := buf.String(); !strings.Contains(html, `<div class="y-inlineaids" lang="zh-Hant">`) {
		t.Errorf("inline reading aids do not restore the chrome language inside a Japanese article: %q", html)
	}
}

// TestWriteFaceReachableInEveryLayoutState is the write-face safety lock over
// the layout matrix. The right rail exists to hold reading aids; a note with
// none drops the rail's column, and narrow viewports hide the rail outright —
// so the fixed-bottom status bar must carry the status face, including the
// fail-closed notice, whenever the rail is absent. The status panel is the
// one surface the vault is written through: no combination of aids and
// contract state may leave its state with nowhere to appear.
//
// Each case constructs the view directly and asserts the collapse predicate
// plus the rendered page. The closed-contract, no-aid note is the pinned
// worst case: before the status bar carried the notices, that combination
// rendered the write face's state nowhere at all.
func TestWriteFaceReachableInEveryLayoutState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		view        NoteView
		wantAids    bool
		wantPresent []string
		wantAbsent  []string
		wantCounts  map[string]int
	}{
		{
			name:     "no aids, open contract: the status bar carries the transition forms",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft", Transitions: []Transition{{To: schema.SealStatus}}},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-sealbar",
				`action="/status"`,
			},
		},
		{
			name:     "no aids, closed contract: the status bar carries the fail-closed notice",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft", Transitions: []Transition{{To: schema.SealStatus}, {To: "archived"}}, WriteDiagnostic: "contract unavailable"},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-sealbar",
				"ui-status--draft",
				"生命週期寫入目前無法使用",
			},
			wantAbsent: []string{`action="/status"`},
			wantCounts: map[string]int{
				`data-status-state="unavailable"`: 2,
				"生命週期寫入目前無法使用":                    2,
			},
		},
		{
			name: "malformed non-instance suppresses every status form even if transitions leak into the view",
			view: NoteView{
				Governed:    true,
				Title:       "Template",
				RelPath:     "System/templates/T.md",
				Status:      "draft",
				Transitions: []Transition{{To: schema.SealStatus}, {To: "archived"}},
				NonInstance: true,
				Diagnostic:  "bad yaml",
			},
			wantAids: true,
			wantPresent: []string{
				"y-statuspanel",
				"y-sealbar",
				"筆記狀況",
			},
			wantAbsent: []string{`action="/status"`, "ui-status--draft"},
			wantCounts: map[string]int{
				`data-status-state="non-instance"`:           2,
				wording.NonInstanceReason.In(wording.ZhHant): 2,
			},
		},
		{
			name:     "no aids, no frontmatter: the status bar says so",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", NoFrontmatter: true},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-sealbar",
				wording.NoFrontmatter.In(wording.ZhHant),
			},
		},
		{
			name:     "headings keep the rail and add the inline disclosure",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft", Transitions: []Transition{{To: schema.SealStatus}}, TOC: []render.TOCEntry{{Level: 2, Text: "H", ID: "h"}}},
			wantAids: true,
			wantPresent: []string{
				"y-statuspanel",
				"y-inlineaids",
				"y-toc-inline",
				"本頁內容",
			},
			wantAbsent: []string{"y-shell--rail-empty"},
		},
		{
			// The one wording for an empty transition set: the schema defines
			// nothing onward. Owner lists gate nothing, so there is no second
			// owner-boundary sentence for the panel to reach for.
			name:     "empty transition set states the schema fact",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft"},
			wantAids: false,
			wantPresent: []string{
				"y-statuspanel",
				"y-sealbar",
				wording.NoLegalTransitions.In(wording.ZhHant),
			},
			wantAbsent: []string{"接下來的狀態轉換由其他 owner 持有"},
			// Counted, not merely present: the two faces carry the same
			// sentence, so a Contains satisfied by the panel alone would leave
			// the bar's branch — the only status face at the widths that drop
			// the rail — locked by nothing.
			wantCounts: map[string]int{wording.NoLegalTransitions.In(wording.ZhHant): 2},
		},
		{
			// The other reading of an empty set: the note sits outside the
			// knowledge layer, so the layer is named on both faces and the
			// schema sentence — false here, since the contract does define
			// onward moves — is not reached for.
			name:        "empty transition set outside the knowledge layer names the layer",
			view:        NoteView{Governed: true, Title: "T", RelPath: "System/agent-guides/a.md", Status: "draft", OutsideKnowledgeScope: true},
			wantAids:    false,
			wantPresent: []string{"y-statuspanel", "y-sealbar"},
			wantAbsent:  []string{wording.NoLegalTransitions.In(wording.ZhHant), `action="/status"`},
			wantCounts:  map[string]int{wording.OutsideKnowledgeScope.In(wording.ZhHant): 2},
		},
		{
			// The corner the invariant missed. The frontmatter parses, so the
			// diagnostics face stays away; its status is a shape no single
			// value can be read out of, so nothing reaches the chip; and with
			// no transitions the bar printed a spacer and nothing else. At
			// every width that drops the rail, that empty bar was the whole
			// write face. Saying "no legal transition" here would be the wrong
			// sentence too: the schema was never consulted, because there was
			// no status to consult it about.
			name:     "unreadable status: both faces say the status could not be read",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md"},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-sealbar",
				"y-statuspanel",
				"讀不出 status 值",
			},
			wantAbsent: []string{wording.NoLegalTransitions.In(wording.ZhHant), `action="/status"`},
			wantCounts: map[string]int{
				"讀不出 status 值": 2,
			},
		},
		{
			name:     "broken frontmatter: the diagnostics face owns the page, no status bar",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Diagnostic: "unterminated string"},
			wantAids: true,
			wantPresent: []string{
				"筆記狀況",
				"y-inlineaids",
			},
			wantAbsent: []string{"y-shell--rail-empty", "y-sealbar", "y-statuspanel"},
		},
		{
			// The gate proving it can close. A folder nothing governs has no
			// lifecycle at all, so neither status face belongs on the page —
			// not even to apologise for a contract that was never promised.
			name:     "ungoverned folder: neither status face appears",
			view:     NoteView{Title: "T", RelPath: "a.md", Status: "draft"},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-title",
			},
			wantAbsent: []string{
				"y-statuspanel",
				"y-sealbar",
				"目前沒有合法的狀態轉換",
				"生命週期寫入目前無法使用",
				`action="/status"`,
			},
		},
		{
			name:     "render diagnostics alone keep the rail",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft", Transitions: []Transition{{To: schema.SealStatus}}, RenderDiagnostics: []render.Diagnostic{{Kind: render.DiagWikilinkBroken, Target: "X", Message: "broken"}}},
			wantAids: true,
			wantPresent: []string{
				"筆記狀況",
				"y-inlineaids",
				"y-statuspanel",
			},
			wantAbsent: []string{"y-shell--rail-empty"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.view.hasAids(); got != tt.wantAids {
				t.Errorf("hasAids() = %v, want %v", got, tt.wantAids)
			}

			var buf bytes.Buffer
			if err := Note(tt.view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			for _, want := range tt.wantPresent {
				if !strings.Contains(html, want) {
					t.Errorf("rendered page is missing %q", want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(html, absent) {
					t.Errorf("rendered page unexpectedly contains %q", absent)
				}
			}
			for marker, want := range tt.wantCounts {
				if got := strings.Count(html, marker); got != want {
					t.Errorf("rendered page count for %q = %d, want %d", marker, got, want)
				}
			}
		})
	}
}

// TestStatusBarMirrorsTheStatusPanelGuard records the status bar's render
// condition as the invariant it is: the bar renders exactly when the status
// panel does. A frontmatter diagnostic ordinarily suppresses both because no
// status was parsed; a path-classified non-instance is the exception and keeps
// both quiet notices visible beside the frontmatter diagnostic.
func TestStatusBarMirrorsTheStatusPanelGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		view          NoteView
		wantStatusBar bool
	}{
		{name: "open contract", view: NoteView{Governed: true, Status: "draft", Transitions: []Transition{{To: schema.SealStatus}}}, wantStatusBar: true},
		{name: "closed contract", view: NoteView{Governed: true, Status: "draft", WriteDiagnostic: "contract unavailable"}, wantStatusBar: true},
		{name: "no frontmatter", view: NoteView{Governed: true, NoFrontmatter: true}, wantStatusBar: true},
		{name: "frontmatter diagnostic", view: NoteView{Governed: true, Diagnostic: "bad yaml"}, wantStatusBar: false},
		{name: "non-instance remains named beside frontmatter diagnostic", view: NoteView{Governed: true, Diagnostic: "bad yaml", NonInstance: true}, wantStatusBar: true},
		// The same views on a folder nothing governs: the bar has no lifecycle
		// to mirror, so it is absent in every one of them.
		{name: "ungoverned open-looking view", view: NoteView{Status: "draft", Transitions: []Transition{{To: schema.SealStatus}}}, wantStatusBar: false},
		{name: "ungoverned no frontmatter", view: NoteView{NoFrontmatter: true}, wantStatusBar: false},
		{name: "ungoverned non-instance", view: NoteView{Diagnostic: "bad yaml", NonInstance: true}, wantStatusBar: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := Note(tt.view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := strings.Contains(buf.String(), "y-sealbar"); got != tt.wantStatusBar {
				t.Errorf("status bar rendered = %v, want %v", got, tt.wantStatusBar)
			}
		})
	}

	t.Run("note carries every legal transition as a plain form", func(t *testing.T) {
		t.Parallel()

		v := NoteView{
			Governed:    true,
			RelPath:     "a.md",
			Status:      "draft",
			Transitions: []Transition{{To: schema.SealStatus}, {To: "archived"}},
		}
		var buf bytes.Buffer
		if err := statusBar(v).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render status bar: %v", err)
		}
		html := buf.String()
		for _, want := range []string{
			`name="to" value="ready"`,
			`name="to" value="archived"`,
			"ui-status--draft",
		} {
			if !strings.Contains(html, want) {
				t.Errorf("status bar is missing %q", want)
			}
		}
		if got, want := strings.Count(html, `name="to"`), 2; got != want {
			t.Errorf("status bar transition form count = %d, want %d", got, want)
		}
	})
}

// TestStatusBarFlagsAStatusOutsideTheSchema locks the narrow-width status
// face to the same answer the rail panel gives an undeclared status value:
// the flag states the fault and no transition form renders, even when the
// view carries onward targets. The bar is the only status face where the
// right rail is absent, so a bar that fell through to plain forms would
// contradict the panel on exactly the viewports that cannot see the panel.
func TestStatusBarFlagsAStatusOutsideTheSchema(t *testing.T) {
	t.Parallel()

	v := NoteView{
		Governed:      true,
		RelPath:       "a.md",
		Status:        "這是草稿",
		StatusUnknown: true,
		Transitions:   []Transition{{To: "archived"}},
	}
	var buf bytes.Buffer
	if err := statusBar(v).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render status bar: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "y-statusflag") {
		t.Errorf("status bar does not carry the out-of-schema flag:\n%s", html)
	}
	if got := strings.Count(html, `name="to"`); got != 0 {
		t.Errorf("status bar renders %d transition forms for an undeclared status, want 0:\n%s", got, html)
	}
}

// TestTransitionButtonsAreDescribedByTheSchemaNotices holds both states of the
// wiring between the transition controls and the schema's findings about this
// note's frontmatter. The amber notices sit beside the buttons, which reaches
// a sighted reader and says nothing to one whose reader announces a focused
// control on its own — so on a page carrying findings, every transition submit
// names the notices block as its description and the block carries the id
// being named; on a page carrying none, the reference is absent rather than
// dangling. The form count is the yardstick for "every": each transition form
// holds exactly one submit, on both status faces, so the number of references
// must equal the number of forms — fewer leaves a button unexplained, more
// means the description leaked onto a control it does not govern.
func TestTransitionButtonsAreDescribedByTheSchemaNotices(t *testing.T) {
	t.Parallel()

	base := NoteView{
		Governed:    true,
		RelPath:     "a.md",
		Status:      "draft",
		Transitions: []Transition{{To: schema.SealStatus}, {To: "archived", NoReturn: true}},
	}

	t.Run("a page with findings describes every submit", func(t *testing.T) {
		t.Parallel()

		v := base
		v.SchemaNotices = [][]wording.SchemaPart{{{Code: "slug"}, {Text: " is written as no slug"}}}
		var buf bytes.Buffer
		if err := Note(v, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render: %v", err)
		}
		html := buf.String()
		if got := strings.Count(html, ` id="schema-notices"`); got != 1 {
			t.Fatalf("the schema-notices id appears %d times, want exactly 1 block for the buttons to point at", got)
		}
		forms := strings.Count(html, `name="to"`)
		if forms == 0 {
			t.Fatal("no transition form rendered, so nothing below checks the description wiring")
		}
		if got := strings.Count(html, `aria-describedby="schema-notices"`); got != forms {
			t.Errorf("%d of %d transition submits name the notices block as their description", got, forms)
		}
	})

	t.Run("a page without findings leaves the controls bare", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := Note(base, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render: %v", err)
		}
		html := buf.String()
		if strings.Contains(html, "schema-notices") {
			t.Errorf("a page with no findings still carries a schema-notices reference or id:\n%s", html)
		}
	})
}

// TestInlineDiagnosticsFoldAboveTheProse pins the placement, not the contents.
// At widths where the right rail is gone this block sits between the title and
// the first sentence, so an open list of findings was the last thing the page
// said before the prose it was about — worst on the notes that have the most of
// them. It folds like the contents above it, so the count is still stated and
// nothing is hidden from a reader who wants it.
func TestInlineDiagnosticsFoldAboveTheProse(t *testing.T) {
	t.Parallel()

	view := NoteView{
		Title:   "DDIA",
		RelPath: "Sources/DDIA.md",
		RenderDiagnostics: []render.Diagnostic{
			{Kind: render.DiagWikilinkBroken, Target: "Relational Model", Message: "does not resolve"},
			{Kind: render.DiagWikilinkBroken, Target: "Leader Election", Message: "does not resolve"},
		},
	}
	var buf bytes.Buffer
	if err := inlineAids(view).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render inlineAids: %v", err)
	}
	got := buf.String()

	if !strings.Contains(got, "y-diaglist") {
		t.Fatalf("the diagnostics did not render at all:\n%s", got)
	}
	before, _, _ := strings.Cut(got, "y-diaglist")
	if !strings.Contains(before, "<summary") {
		t.Errorf("the diagnostics are not inside a disclosure; they open above the prose:\n%s", got)
	}
	if !strings.Contains(got, "筆記狀況") {
		t.Errorf("the disclosure does not name what it holds:\n%s", got)
	}
	if !strings.Contains(got, ">2<") {
		t.Errorf("the count is not stated on the closed disclosure:\n%s", got)
	}
}

// TestFlipReceiptStatesTheChangeItCanVouchFor holds what a reader is told
// after a transition installs. The redirect used to land on the note with the
// new chip and nothing else, so the whole confirmation was a coloured word
// that looks the same whether the press worked, did nothing, or was somebody
// else's. A reader who cannot see the chip got no answer at all.
//
// The line is a live region so it reaches a reader on arrival, and it states
// only what the page can stand behind: the note's status now, and what the
// form said it was leaving. It is not rendered when those agree, so a hand
// typed address cannot manufacture a change that did not happen.
func TestFlipReceiptStatesTheChangeItCanVouchFor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		view        NoteView
		wantPresent []string
		wantAbsent  []string
	}{
		{
			name:        "a real transition is stated once, in a live region",
			view:        NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "ready", FlippedFrom: "draft"},
			wantPresent: []string{`role="status"`, "已從", "draft", "ready"},
		},
		{
			name:       "an ordinary reading carries no receipt",
			view:       NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "ready"},
			wantAbsent: []string{"已從"},
		},
		{
			name:       "a hand typed origin equal to the current status states nothing",
			view:       NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "ready", FlippedFrom: "ready"},
			wantAbsent: []string{"已從"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Note(tt.view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			for _, want := range tt.wantPresent {
				if !strings.Contains(html, want) {
					t.Errorf("the receipt is missing %q", want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(html, absent) {
					t.Errorf("a receipt appeared where none is true (%q)", absent)
				}
			}
		})
	}
}

// TestNoteMetarowOmitsAMissingDate holds the one dateless case: a view built
// with no date at all renders no time element and neither label, rather than
// an empty claim. Every served note has a scanned file behind it, so the case
// is a generation that could not produce the file's identity — and a blank
// beats a fabricated moment.
func TestNoteMetarowOmitsAMissingDate(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	view := NoteView{Title: "A", RelPath: "Writing/A.md"}
	if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render note: %v", err)
	}
	html := buf.String()
	at := strings.Index(html, `class="y-metarow"`)
	if at < 0 {
		t.Fatalf("the page carries no metarow; html = %q", html)
	}
	metarow := html[at:]
	if end := strings.Index(metarow, "</div>"); end >= 0 {
		metarow = metarow[:end]
	}
	if strings.Contains(metarow, "<time") {
		t.Errorf("a dateless view still renders a time element; metarow = %q", metarow)
	}
	for _, label := range []string{"更新於", "檔案變更於", "Updated", "File changed"} {
		if strings.Contains(metarow, label) {
			t.Errorf("a dateless view still carries the label %q; metarow = %q", label, metarow)
		}
	}
}
