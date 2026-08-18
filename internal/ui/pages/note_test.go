package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

func TestNoteArticleLanguageIsExplicit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "missing authority is undetermined", want: `lang="und"`},
		{name: "Japanese", lang: "ja", want: `lang="ja"`},
		{name: "Traditional Chinese", lang: "zh-Hant", want: `lang="zh-Hant"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Note(NoteView{Language: tt.lang}, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if html := buf.String(); !strings.Contains(html, `<article class="y-article" `+tt.want+`>`) {
				t.Errorf("article language missing: want %q in %q", tt.want, html)
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
// so the fixed-bottom seal bar must carry the status face, including the
// fail-closed notice, whenever the rail is absent. The status panel is the
// one surface the vault is written through: no combination of aids and
// contract state may leave its state with nowhere to appear.
//
// Each case constructs the view directly and asserts the collapse predicate
// plus the rendered page. The closed-contract, no-aid note is the pinned
// worst case: before the seal bar carried the notices, that combination
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
			name:     "no aids, open contract: the seal bar carries the seal form",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft", SealTarget: schema.SealStatus, Transitions: []string{schema.SealStatus}},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-sealbar",
				"data-seal",
				"操作者 · koopa",
			},
		},
		{
			name:     "no aids, closed contract: the seal bar carries the fail-closed notice",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft", SealTarget: schema.SealStatus, Transitions: []string{schema.SealStatus, "archived"}, WriteDiagnostic: "contract unavailable"},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-sealbar",
				"ui-status--draft",
				"生命週期寫入目前無法使用",
			},
			wantAbsent: []string{`action="/status"`, "操作者 · koopa"},
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
				SealTarget:  schema.SealStatus,
				Transitions: []string{schema.SealStatus, "archived"},
				NonInstance: true,
				Diagnostic:  "bad yaml",
			},
			wantAids: true,
			wantPresent: []string{
				"y-statuspanel",
				"y-sealbar",
				"筆記狀況",
			},
			wantAbsent: []string{`action="/status"`, "操作者 · koopa", "ui-status--draft"},
			wantCounts: map[string]int{
				`data-status-state="non-instance"`: 2,
				"不屬於生命週期治理範圍":                      2,
			},
		},
		{
			name:     "no aids, no frontmatter: the seal bar says so",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", NoFrontmatter: true},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-sealbar",
				"沒有 frontmatter（合法）。",
			},
		},
		{
			name:     "headings keep the rail and add the inline disclosure",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft", SealTarget: schema.SealStatus, Transitions: []string{schema.SealStatus}, TOC: []render.TOCEntry{{Level: 2, Text: "H", ID: "h"}}},
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
			// An empty transition list has two producers, and the rail must not
			// use the schema-defines-nothing sentence for the one where onward
			// steps exist but belong to another owner.
			name:     "owner-withheld transitions name the owner boundary",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft", SealTarget: schema.SealStatus, WithheldByOwner: true},
			wantAids: false,
			wantPresent: []string{
				"y-statuspanel",
				"接下來的狀態轉換由其他 owner 持有；目前操作者沒有執行權限。",
			},
			wantAbsent: []string{"目前沒有合法的狀態轉換"},
		},
		{
			name:     "broken frontmatter: the diagnostics face owns the page, no seal bar",
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
			view:     NoteView{Title: "T", RelPath: "a.md", Status: "draft", SealTarget: schema.SealStatus},
			wantAids: false,
			wantPresent: []string{
				"y-shell--rail-empty",
				"y-title",
			},
			wantAbsent: []string{
				"y-statuspanel",
				"y-sealbar",
				"操作者 · koopa",
				"目前沒有合法的狀態轉換",
				"生命週期寫入目前無法使用",
				`action="/status"`,
			},
		},
		{
			name:     "render diagnostics alone keep the rail",
			view:     NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: "draft", SealTarget: schema.SealStatus, Transitions: []string{schema.SealStatus}, RenderDiagnostics: []render.Diagnostic{{Kind: render.DiagWikilinkBroken, Target: "X", Message: "broken"}}},
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

func TestSealFormUsesViewTarget(t *testing.T) {
	t.Parallel()

	v := NoteView{
		Governed:    true,
		Title:       "T",
		RelPath:     "a.md",
		Status:      "draft",
		SealTarget:  "candidate",
		Transitions: []string{"candidate"},
	}
	var buf bytes.Buffer
	if err := Note(v, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "data-seal") {
		t.Fatalf("rendered page has no seal form: %q", html)
	}
	if want := `name="to" value="candidate"`; !strings.Contains(html, want) {
		t.Errorf("seal form is missing %q", want)
	}
}

// TestSealToastRidesTheRedirectSignal locks the toast's one-shot contract:
// it renders exactly when the page carries the seal redirect's signal, so
// the confirmation is server-rendered CSS that plays with or without the
// enhancement script.
func TestSealToastRidesTheRedirectSignal(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name       string
		justSealed bool
	}{
		{name: "just sealed renders the toast", justSealed: true},
		{name: "an ordinary view does not", justSealed: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v := NoteView{Governed: true, Title: "T", RelPath: "a.md", Status: schema.SealStatus, SealTarget: schema.SealStatus, Sealed: true, JustSealed: tt.justSealed}
			var buf bytes.Buffer
			if err := Note(v, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := strings.Contains(buf.String(), "y-toast"); got != tt.justSealed {
				t.Errorf("toast rendered = %v, want %v", got, tt.justSealed)
			}
		})
	}
}

// TestSealBarMirrorsTheStatusPanelGuard records the seal bar's render
// condition as the invariant it is: the bar renders exactly when the status
// panel does. A frontmatter diagnostic ordinarily suppresses both because no
// status was parsed; a path-classified non-instance is the exception and keeps
// both quiet notices visible beside the frontmatter diagnostic.
func TestSealBarMirrorsTheStatusPanelGuard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		view        NoteView
		wantSealBar bool
	}{
		{name: "open contract", view: NoteView{Governed: true, Status: "draft", SealTarget: schema.SealStatus, Transitions: []string{schema.SealStatus}}, wantSealBar: true},
		{name: "closed contract", view: NoteView{Governed: true, Status: "draft", WriteDiagnostic: "contract unavailable"}, wantSealBar: true},
		{name: "no frontmatter", view: NoteView{Governed: true, NoFrontmatter: true}, wantSealBar: true},
		{name: "sealed note", view: NoteView{Governed: true, Status: schema.SealStatus, SealTarget: schema.SealStatus, Sealed: true}, wantSealBar: true},
		{name: "frontmatter diagnostic", view: NoteView{Governed: true, Diagnostic: "bad yaml"}, wantSealBar: false},
		{name: "non-instance remains named beside frontmatter diagnostic", view: NoteView{Governed: true, Diagnostic: "bad yaml", NonInstance: true}, wantSealBar: true},
		// The same views on a folder nothing governs: the bar has no lifecycle
		// to mirror, so it is absent in every one of them.
		{name: "ungoverned open-looking view", view: NoteView{Status: "draft", SealTarget: schema.SealStatus, Transitions: []string{schema.SealStatus}}, wantSealBar: false},
		{name: "ungoverned no frontmatter", view: NoteView{NoFrontmatter: true}, wantSealBar: false},
		{name: "ungoverned non-instance", view: NoteView{Diagnostic: "bad yaml", NonInstance: true}, wantSealBar: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			if err := Note(tt.view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			if got := strings.Contains(buf.String(), "y-sealbar"); got != tt.wantSealBar {
				t.Errorf("seal bar rendered = %v, want %v", got, tt.wantSealBar)
			}
		})
	}

	t.Run("unsealed note carries every legal transition", func(t *testing.T) {
		t.Parallel()

		v := NoteView{
			Governed:    true,
			RelPath:     "a.md",
			Status:      "draft",
			SealTarget:  schema.SealStatus,
			Transitions: []string{schema.SealStatus, "archived"},
		}
		var buf bytes.Buffer
		if err := sealBar(v).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render seal bar: %v", err)
		}
		html := buf.String()
		for _, want := range []string{
			`name="to" value="ready"`,
			`name="to" value="archived"`,
		} {
			if !strings.Contains(html, want) {
				t.Errorf("seal bar is missing %q", want)
			}
		}
		if got, want := strings.Count(html, `name="to"`), 2; got != want {
			t.Errorf("seal bar transition form count = %d, want %d", got, want)
		}
	})

	t.Run("sealed note carries onward transition after badge", func(t *testing.T) {
		t.Parallel()

		v := NoteView{
			Governed:    true,
			RelPath:     "a.md",
			Status:      schema.SealStatus,
			SealTarget:  schema.SealStatus,
			Sealed:      true,
			Transitions: []string{"archived"},
		}
		var buf bytes.Buffer
		if err := sealBar(v).Render(t.Context(), &buf); err != nil {
			t.Fatalf("render seal bar: %v", err)
		}
		html := buf.String()
		badgeIndex := strings.Index(html, "y-sealed")
		if badgeIndex < 0 {
			t.Fatal("seal bar is missing the sealed badge")
		}
		toIndex := strings.Index(html, `name="to" value="archived"`)
		if toIndex < 0 {
			t.Error(`seal bar is missing name="to" value="archived"`)
		} else if toIndex < badgeIndex {
			t.Error("seal bar renders the onward transition before the sealed badge")
		}
		if got, want := strings.Count(html, `name="to"`), 1; got != want {
			t.Errorf("seal bar transition form count = %d, want %d", got, want)
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
