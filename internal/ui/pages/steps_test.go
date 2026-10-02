package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/ui/layouts"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheFootNamesTheOrderItWalks locks what a sighted reader is shown at the
// foot of the article: the order's printed name and per-link step words, not
// just an accessible name. A course's declared order and the folder's file
// adjacency can disagree completely, and before the label was printed the two
// feet were pixel-identical. It would catch the label falling back into the
// accessible name alone, a folder foot borrowing the course's words, and the
// printed label drifting from what assistive technology is told.
func TestTheFootNamesTheOrderItWalks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		view      NoteView
		want      []string
		forbidden []string
	}{
		{
			name: "a course hands over lessons",
			view: NoteView{
				Prev:        nav.NoteRef{Name: "Setup", RelPath: "Writing/Setup.md"},
				Next:        nav.NoteRef{Name: "Basics", RelPath: "Writing/Basics.md"},
				StepsLabel:  "Go course 從此步往下",
				StepsCourse: true,
				StepsUnit:   nav.UnitLesson,
			},
			want: []string{
				`<nav class="y-steps y-steps--course" lang="zh-Hant" aria-label="Go course 從此步往下">`,
				`<p class="y-steps__source">Go course 從此步往下</p>`,
				`<span class="y-steps__role"><span class="y-steps__dir" aria-hidden="true">←</span> 上一課</span>`,
				`<span class="y-steps__role">下一課 <span class="y-steps__dir" aria-hidden="true">→</span></span>`,
				`href="/notes/Writing/Setup.md" rel="prev"`,
				`href="/notes/Writing/Basics.md" rel="next"`,
			},
			forbidden: []string{"上一份", "下一份", "上一篇", "下一篇"},
		},
		{
			// A path of anything but lessons hands over items. The words are the
			// same shape and a different noun, and a lesson is never promised.
			name: "a path of items hands over items",
			view: NoteView{
				Prev:        nav.NoteRef{Name: "Queue", RelPath: "Concepts/Queue.md"},
				Next:        nav.NoteRef{Name: "Worker", RelPath: "Concepts/Worker.md"},
				StepsLabel:  "Queue kit 從此步往下",
				StepsCourse: true,
				StepsUnit:   nav.UnitItem,
			},
			want: []string{
				`<span class="y-steps__role"><span class="y-steps__dir" aria-hidden="true">←</span> 上一篇</span>`,
				`<span class="y-steps__role">下一篇 <span class="y-steps__dir" aria-hidden="true">→</span></span>`,
			},
			forbidden: []string{"上一課", "下一課", "上一份", "下一份"},
		},
		{
			name: "a folder hands over neighbouring files",
			view: NoteView{
				Prev:       nav.NoteRef{Name: "2026-08-01", RelPath: "Diary/2026-08-01.md"},
				Next:       nav.NoteRef{Name: "2026-08-03", RelPath: "Diary/2026-08-03.md"},
				StepsLabel: "同資料夾的前後檔案",
			},
			want: []string{
				`<nav class="y-steps" lang="zh-Hant" aria-label="同資料夾的前後檔案">`,
				`<p class="y-steps__source">同資料夾的前後檔案</p>`,
				`<span class="y-steps__role"><span class="y-steps__dir" aria-hidden="true">←</span> 上一份</span>`,
				`<span class="y-steps__role">下一份 <span class="y-steps__dir" aria-hidden="true">→</span></span>`,
				`rel="prev"`,
				`rel="next"`,
			},
			forbidden: []string{"上一課", "下一課", "課程順序"},
		},
		{
			name: "a first lesson has no step back",
			view: NoteView{
				Next:        nav.NoteRef{Name: "Basics", RelPath: "Writing/Basics.md"},
				StepsLabel:  "Go course 從此步往下",
				StepsCourse: true,
				StepsUnit:   nav.UnitLesson,
			},
			want:      []string{`<p class="y-steps__source">Go course 從此步往下</p>`, `<span class="y-steps__role">下一課 <span class="y-steps__dir" aria-hidden="true">→</span></span>`},
			forbidden: []string{`rel="prev"`, "上一課"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := sequenceSteps(tt.view, wording.ZhHant).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render sequence steps: %v", err)
			}
			html := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(html, want) {
					t.Errorf("the foot does not print %q; html = %q", want, html)
				}
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(html, forbidden) {
					t.Errorf("the foot claims an order it is not walking: found %q; html = %q", forbidden, html)
				}
			}
		})
	}
}

// TestTheFootChoosesTheOrderItCanKnow pins FooterSequence's choice — and the
// course flag that travels with it — against the real navigation build. These
// lessons declare no domain of their own, so a note two courses teach falls
// back to the folder, nothing having picked one course out; a lesson the course
// teaches never does, whatever steps it has; a side branch's last lesson ends
// its branch rather than rejoining the main line's steps; and the main line
// steps over a side branch hanging inside it, which it offers beside its own
// steps as an aside.
//
// What a side branch hands over at its ends is asserted with it: the first
// lesson the way back to the lesson it hangs from, the last the way on to the
// main line's next lesson, the lesson it hangs from the branches themselves.
// None of them is a step, so Prev and Next are what they were without them.
func TestTheFootChoosesTheOrderItCanKnow(t *testing.T) {
	t.Parallel()

	model := buildStepsModel(t)
	tests := []struct {
		name       string
		current    string
		wantPrev   string
		wantNext   string
		wantLabel  string
		wantCourse bool
		wantBack   string
		wantOnward string
		wantAsides []string
	}{
		{
			name:    "a note two courses teach keeps the folder",
			current: "Course/C01.md",
			// The folder's own adjacency, which is not the course's order.
			wantNext:   "Course/C02.md",
			wantLabel:  "同資料夾的前後檔案",
			wantCourse: false,
		},
		{
			name:     "the main line steps over the side branch and points at it",
			current:  "Course/C02.md",
			wantPrev: "Course/C01.md",
			// The folder's neighbour is S01; the course's is C03.
			wantNext:   "Course/C03.md",
			wantLabel:  "Branch course 從此步往下",
			wantCourse: true,
			// Both branches hang from it, in the order they were written, each
			// by its first lesson.
			wantAsides: []string{"Course/S01.md", "Course/X01.md"},
		},
		{
			// No course teaches it, so no order was declared for it and the
			// folder's neighbour is the only one there is.
			name:       "a note no course teaches keeps the folder",
			current:    "Course/Z99.md",
			wantPrev:   "Course/X01.md",
			wantLabel:  "同資料夾的前後檔案",
			wantCourse: false,
		},
		{
			name:       "a side branch's first lesson can go back to where it hangs from",
			current:    "Course/S01.md",
			wantNext:   "Course/S02.md",
			wantLabel:  "Branch course 從此步往下",
			wantCourse: true,
			wantBack:   "Course/C02.md",
		},
		{
			name:     "a side branch's last lesson closes it and goes on to the main line",
			current:  "Course/S02.md",
			wantPrev: "Course/S01.md",
			// No next: the branch never rejoins the main line's steps. The way on
			// is its own, to the lesson after the one the branch hangs from.
			wantNext:   "",
			wantLabel:  "Branch course 從此步往下",
			wantCourse: true,
			wantOnward: "Course/C03.md",
		},
		{
			// The folder's neighbour is S02, which the course would have called
			// previous: the branch's only lesson steps nowhere, and the foot is
			// still the course's.
			name:       "a branch of one lesson goes back and on and steps nowhere",
			current:    "Course/X01.md",
			wantLabel:  "Branch course 從此步往下",
			wantCourse: true,
			wantBack:   "Course/C02.md",
			wantOnward: "Course/C03.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resolved := NewReadingRail(nav.Shell{Nav: model}, tt.current, "")
			foot := FooterSequence(&resolved, wording.ZhHant)
			if foot.Prev.RelPath != tt.wantPrev {
				t.Errorf("FooterSequence(%q) prev = %q, want %q", tt.current, foot.Prev.RelPath, tt.wantPrev)
			}
			if foot.Next.RelPath != tt.wantNext {
				t.Errorf("FooterSequence(%q) next = %q, want %q", tt.current, foot.Next.RelPath, tt.wantNext)
			}
			if foot.Label != tt.wantLabel {
				t.Errorf("FooterSequence(%q) label = %q, want %q", tt.current, foot.Label, tt.wantLabel)
			}
			if foot.Course != tt.wantCourse {
				t.Errorf("FooterSequence(%q) course = %v, want %v", tt.current, foot.Course, tt.wantCourse)
			}
			if foot.Back.RelPath != tt.wantBack {
				t.Errorf("FooterSequence(%q) back = %q, want %q", tt.current, foot.Back.RelPath, tt.wantBack)
			}
			if foot.Onward.RelPath != tt.wantOnward {
				t.Errorf("FooterSequence(%q) onward = %q, want %q", tt.current, foot.Onward.RelPath, tt.wantOnward)
			}
			var asides []string
			for _, a := range foot.Asides {
				asides = append(asides, a.RelPath)
			}
			if diff := cmp.Diff(tt.wantAsides, asides); diff != "" {
				t.Errorf("FooterSequence(%q) asides (-want +got):\n%s", tt.current, diff)
			}
		})
	}
}

// buildStepsModel writes a vault whose folder order and course order disagree
// on purpose — the folder sorts C01 C02 C03 S01 S02 while the course walks
// C01 C02 C03 with S01 S02 on a side branch — plus a second course listing
// C01, then reads it back through the real navigation build.
func buildStepsModel(t *testing.T) *nav.Model {
	t.Helper()
	root := t.TempDir()
	lesson := func(title string) string {
		return "---\ntitle: " + title + "\ntype: lesson\nstatus: draft\n---\nbody\n"
	}
	files := map[string]string{
		"Maps/Branch course.md": "---\ntitle: Branch course\ntype: study-path\ndomain: golang\n---\n\n" +
			"## 主線 {sequence=primary}\n\n" +
			"- [[C01]]\n" +
			"- [[C02]]\n" +
			"\t- 選修 {sequence=local}\n" +
			"\t\t- [[S01]]\n" +
			"\t\t- [[S02]]\n" +
			"\t- 獨行 {sequence=local}\n" +
			"\t\t- [[X01]]\n" +
			"- [[C03]]\n",
		"Maps/Second course.md": "---\ntitle: Second course\ntype: study-path\ndomain: golang\n---\n\n" +
			"## 導讀 {sequence=primary}\n\n" +
			"- [[C01]]\n",
		"Course/C01.md": lesson("C01"),
		"Course/C02.md": lesson("C02"),
		"Course/C03.md": lesson("C03"),
		"Course/S01.md": lesson("S01"),
		"Course/S02.md": lesson("S02"),
		"Course/X01.md": lesson("X01"),
		// In the same folder and taught by no course.
		"Course/Z99.md": "---\ntitle: Z99\ntype: writing\n---\nbody\n",
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			t.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatalf("ScanComplete() error = %v", err)
	}
	notes := make(map[string]*vault.Note)
	noteList := make([]*vault.Note, 0, len(scan.Files()))
	for _, entry := range scan.Files() {
		data, readErr := reader.ReadFile(t.Context(), entry)
		if readErr != nil {
			t.Fatalf("ReadFile() error = %v", readErr)
		}
		note := vault.Parse(entry.Path(), data)
		notes[entry.Path()] = note
		noteList = append(noteList, note)
	}
	contract, err := schema.LoadFile(filepath.Join("..", "..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("schema.LoadFile = %v", err)
	}
	return nav.New(
		scan.Files(), notes, graph.New(noteList, nil),
		contract.NavigationRoles(), contract.KnowledgeScope(), contract.ArtifactPolicy(),
		contract.JournalDir(),
		contract.ArticleLanguage(), contract.AuthoredDate(),
		contract.Settlement(),
		nil,
	)
}

// TestOnlyTheFootOffersTheCourseStep locks that on a note one course teaches,
// the step onward is one landmark, in the article foot: the book rail's head
// carries the course's name and extent and no step navigation of its own.
func TestOnlyTheFootOffersTheCourseStep(t *testing.T) {
	t.Parallel()

	model := buildStepsModel(t)
	current := "Course/C02.md"
	resolved := NewReadingRail(nav.Shell{Nav: model}, current, "golang")
	chosen := FooterSequence(&resolved, wording.ZhHant)
	if !chosen.Course || chosen.Label == "" {
		t.Fatalf("FooterSequence(%q) did not choose a course foot: label=%q course=%v", current, chosen.Label, chosen.Course)
	}

	var rail bytes.Buffer
	if err := readingRail(resolved, layouts.Chrome{Lang: wording.ZhHant}).Render(t.Context(), &rail); err != nil {
		t.Fatalf("render reading rail: %v", err)
	}
	if strings.Contains(rail.String(), "<nav") {
		t.Errorf("the book rail renders a step landmark, so the page offers the step onward twice; html = %q", rail.String())
	}

	var foot bytes.Buffer
	view := NoteView{Prev: chosen.Prev, Next: chosen.Next, StepsLabel: chosen.Label, StepsCourse: chosen.Course, StepsUnit: chosen.Unit}
	if err := sequenceSteps(view, wording.ZhHant).Render(t.Context(), &foot); err != nil {
		t.Fatalf("render sequence steps: %v", err)
	}
	footName := navAriaLabel(foot.String(), "y-steps")
	if footName == "" {
		t.Fatalf("the foot has no y-steps landmark; html = %q", foot.String())
	}
}

func navAriaLabel(html, class string) string {
	// The marker stops before the closing quote so a modifier class on the
	// same element — y-steps--course on the foot nav — still matches.
	marker := `class="` + class
	at := strings.Index(html, marker)
	if at < 0 {
		return ""
	}
	start := strings.LastIndex(html[:at+len(marker)], "<nav")
	if start < 0 {
		return ""
	}
	end := strings.Index(html[start:], ">")
	if end < 0 {
		return ""
	}
	tag := html[start : start+end]
	_, rest, ok := strings.Cut(tag, `aria-label="`)
	if !ok {
		return ""
	}
	label, _, ok := strings.Cut(rest, `"`)
	if !ok {
		return ""
	}
	return label
}

// TestAFootWithNoStepsSaysNothing keeps the foot silent on a note with nowhere
// onward: a printed source line over zero links would name an order that
// offers nothing.
func TestAFootWithNoStepsSaysNothing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := sequenceSteps(NoteView{StepsLabel: "同資料夾的前後檔案"}, wording.ZhHant).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render sequence steps: %v", err)
	}
	if html := buf.String(); strings.Contains(html, "y-steps") {
		t.Errorf("a note with no steps still renders the foot; html = %q", html)
	}
}
