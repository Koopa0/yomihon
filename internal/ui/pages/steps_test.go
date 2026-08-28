package pages

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
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
				StepsLabel:  "Go course 課程順序",
				StepsCourse: true,
			},
			want: []string{
				`<nav class="y-steps" aria-label="Go course 課程順序">`,
				`<p class="y-steps__source">Go course 課程順序</p>`,
				`<span class="y-steps__role">上一課</span>`,
				`<span class="y-steps__role">下一課</span>`,
				`href="/notes/Writing/Setup.md" rel="prev"`,
				`href="/notes/Writing/Basics.md" rel="next"`,
			},
			forbidden: []string{"上一份", "下一份"},
		},
		{
			name: "a folder hands over neighbouring files",
			view: NoteView{
				Prev:       nav.NoteRef{Name: "2026-08-01", RelPath: "Diary/2026-08-01.md"},
				Next:       nav.NoteRef{Name: "2026-08-03", RelPath: "Diary/2026-08-03.md"},
				StepsLabel: "同資料夾的前後檔案",
			},
			want: []string{
				`<nav class="y-steps" aria-label="同資料夾的前後檔案">`,
				`<p class="y-steps__source">同資料夾的前後檔案</p>`,
				`<span class="y-steps__role">上一份</span>`,
				`<span class="y-steps__role">下一份</span>`,
				`rel="prev"`,
				`rel="next"`,
			},
			forbidden: []string{"上一課", "下一課", "課程順序"},
		},
		{
			name: "a first lesson has no step back",
			view: NoteView{
				Next:        nav.NoteRef{Name: "Basics", RelPath: "Writing/Basics.md"},
				StepsLabel:  "Go course 課程順序",
				StepsCourse: true,
			},
			want:      []string{`<p class="y-steps__source">Go course 課程順序</p>`, `<span class="y-steps__role">下一課</span>`},
			forbidden: []string{`rel="prev"`, "上一課"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := sequenceSteps(tt.view).Render(t.Context(), &buf); err != nil {
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
// course flag that now travels with it — against the real navigation build.
// A note two courses teach falls back to the folder, because which course the
// reader is walking is not knowable; a side branch's last lesson ends its
// branch rather than rejoining the main line; and the main line steps over a
// side branch hanging inside it.
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
			name:     "the main line steps over the side branch",
			current:  "Course/C02.md",
			wantPrev: "Course/C01.md",
			// The folder's neighbour is S01; the course's is C03.
			wantNext:   "Course/C03.md",
			wantLabel:  "Branch course 課程順序",
			wantCourse: true,
		},
		{
			name:     "a side branch's last lesson closes it",
			current:  "Course/S02.md",
			wantPrev: "Course/S01.md",
			// No next: the branch never rejoins the main line.
			wantNext:   "",
			wantLabel:  "Branch course 課程順序",
			wantCourse: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prev, next, label, course := FooterSequence(model, tt.current)
			if prev.RelPath != tt.wantPrev {
				t.Errorf("FooterSequence(%q) prev = %q, want %q", tt.current, prev.RelPath, tt.wantPrev)
			}
			if next.RelPath != tt.wantNext {
				t.Errorf("FooterSequence(%q) next = %q, want %q", tt.current, next.RelPath, tt.wantNext)
			}
			if label != tt.wantLabel {
				t.Errorf("FooterSequence(%q) label = %q, want %q", tt.current, label, tt.wantLabel)
			}
			if course != tt.wantCourse {
				t.Errorf("FooterSequence(%q) course = %v, want %v", tt.current, course, tt.wantCourse)
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
			"- [[C03]]\n",
		"Maps/Second course.md": "---\ntitle: Second course\ntype: study-path\ndomain: golang\n---\n\n" +
			"## 導讀 {sequence=primary}\n\n" +
			"- [[C01]]\n",
		"Course/C01.md": lesson("C01"),
		"Course/C02.md": lesson("C02"),
		"Course/C03.md": lesson("C03"),
		"Course/S01.md": lesson("S01"),
		"Course/S02.md": lesson("S02"),
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
	)
}

// TestAFootWithNoStepsSaysNothing keeps the foot silent on a note with nowhere
// onward: a printed source line over zero links would name an order that
// offers nothing.
func TestAFootWithNoStepsSaysNothing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := sequenceSteps(NoteView{StepsLabel: "同資料夾的前後檔案"}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render sequence steps: %v", err)
	}
	if html := buf.String(); strings.Contains(html, "y-steps") {
		t.Errorf("a note with no steps still renders the foot; html = %q", html)
	}
}
