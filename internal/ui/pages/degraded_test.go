package pages

import (
	"bytes"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/ui/layouts"
)

// TestNotePageSaysWhenItShowsACopyItCouldNotReRead pins the one difference
// between a note read from the file and a note carried from the last reading
// that could open it. Nothing else on the page differs, so without the line a
// reader comparing the words against what they last wrote would conclude the
// edit was lost rather than unread.
func TestNotePageSaysWhenItShowsACopyItCouldNotReRead(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		stale bool
	}{
		{name: "read from the file"},
		{name: "carried from the last reading", stale: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			view := NoteView{Title: "Carried", RelPath: "Concepts/Carried.md", Stale: tt.stale}
			if err := Note(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if got := strings.Contains(html, "data-note-stale"); got != tt.stale {
				t.Errorf("stale marker present = %t, want %t", got, tt.stale)
			}
			if got := strings.Contains(html, "下面是上一次讀到的內容"); got != tt.stale {
				t.Errorf("last-known-words sentence present = %t, want %t", got, tt.stale)
			}
		})
	}
}

// TestHealthSaysHowOldTheWholePictureIs pins the two things the blocked list
// can be read against. A folder that was read whole an hour ago and one that
// has never been read whole since startup both show blocked files, and the
// reader's question — how much of what is on screen can be trusted — has
// opposite answers. Saying nothing was ever read whole while a whole reading
// exists understates the page; claiming one that never happened overstates it.
func TestHealthSaysHowOldTheWholePictureIs(t *testing.T) {
	t.Parallel()
	blocked := []HealthBlockedSource{{Path: "Concepts/Carried.md", Reason: "permission denied"}}
	tests := []struct {
		name         string
		lastComplete string
		want         string
		absent       string
	}{
		{
			name:         "read whole before the file shut",
			lastComplete: "2026-08-19 10:32",
			want:         "最後一次完整讀取是 2026-08-19 10:32。",
			absent:       "啟動到現在還沒有一次完整的讀取",
		},
		{
			name:   "never read whole since startup",
			want:   "啟動到現在還沒有一次完整的讀取",
			absent: "最後一次完整讀取是",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			view := HealthView{Blocked: blocked, LastComplete: tt.lastComplete}
			if err := Health(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if !strings.Contains(html, tt.want) {
				t.Errorf("health page does not say %q", tt.want)
			}
			if strings.Contains(html, tt.absent) {
				t.Errorf("health page also says %q, which contradicts it", tt.absent)
			}
		})
	}
}

// TestHealthAllClearClaimsOnlyTheLinksItRead holds the scope of the sentence
// this page shows when it has nothing to report. Its link answers come from
// one extractor that reads [[…]] and nothing else, so an ordinary Markdown
// link to a file that is not there is invisible here — it is live navigation
// to a 404 on the reading page and a warning from the command line, and this
// page said every link had a target. A reader who trusts the all-clear cannot
// be told a wider thing than was checked.
func TestHealthAllClearClaimsOnlyTheLinksItRead(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := Health(HealthView{}, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if !strings.Contains(html, "每個 [[…]] 連結都有目標") {
		t.Errorf("the all-clear does not name which links it read:\n%s", html)
	}
	if strings.Contains(html, "每個連結都有目標") {
		t.Errorf("the all-clear still claims every link, including the ones this page never reads")
	}
	// The same limit applies to the sentence's opening. A note whose
	// frontmatter is missing a field the contract requires never reaches this
	// page at all — the status list here is built from the notes that hold a
	// status, so one that holds none is in no section — and a verdict over the
	// whole folder would certify it clean.
	if strings.Contains(html, "這個書庫目前沒有需要處理的事") {
		t.Errorf("the all-clear passes a verdict on the whole folder rather than on what it checked:\n%s", html)
	}
	if !strings.Contains(html, "yomihon check") {
		t.Errorf("the all-clear does not say where the checks it skips are answered:\n%s", html)
	}
}

// TestHealthDoesNotCallEveryMissingTargetANote holds the heading over the
// list of citations that land nowhere. The list is built from targets nothing
// answers to, and a target is whatever its author wrote — a note they have not
// written yet, and equally a picture or a PDF that is not in the folder. Under
// a heading naming notes, the second kind arrived as a note the reader is owed
// and would go looking to write; the two need opposite repairs.
func TestHealthDoesNotCallEveryMissingTargetANote(t *testing.T) {
	t.Parallel()
	view := HealthView{Unwritten: []HealthLink{
		{From: nav.NoteRef{Name: "A", RelPath: "Concepts/A.md"}, Target: "還沒寫的概念"},
		{From: nav.NoteRef{Name: "A", RelPath: "Concepts/A.md"}, Target: "diagram.png"},
	}}
	var buf bytes.Buffer
	if err := Health(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	html := buf.String()
	if strings.Contains(html, "連到還沒寫的筆記") {
		t.Errorf("the heading calls every missing target a note the reader has yet to write")
	}
	if !strings.Contains(html, "連到不存在的目標") {
		t.Errorf("the heading does not name what the list actually holds:\n%s", html)
	}
	// Both kinds still appear; narrowing the claim must not narrow the list.
	for _, target := range []string{"還沒寫的概念", "diagram.png"} {
		if !strings.Contains(html, target) {
			t.Errorf("the list dropped %q", target)
		}
	}
}

// TestHomeNamesTheGenerationItCouldNotFinish holds Home's own degraded block.
// The three fault blocks look alike and each is gated on its own field, so a
// page assembled from a reading that could not finish has to say which of them
// is speaking — and a folder whose reading did finish must not carry the
// apology at all.
func TestHomeNamesTheGenerationItCouldNotFinish(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		view     HomeView
		degraded bool
	}{
		{name: "a reading that finished", view: HomeView{}},
		{
			name:     "a reading that could not read every file",
			view:     HomeView{Degraded: "有檔案讀不進來", DegradedDetail: "permission denied"},
			degraded: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := Home(tt.view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if got := strings.Contains(html, "data-home-degraded"); got != tt.degraded {
				t.Errorf("Home's degraded notice present = %t, want %t", got, tt.degraded)
			}
			if tt.degraded && !strings.Contains(html, "permission denied") {
				t.Errorf("Home's degraded notice drops the reason the reading gave; html = %q", html)
			}
		})
	}
}

// TestAReportSaysWhatItCannotDraw holds the report shell's one honest
// admission. A briefing that draws part of itself with a script arrives here
// with scripting shut off, so that part is simply absent — and a hole with
// nothing beside it reads as a broken report rather than as the boundary
// working. A report that draws itself entirely in markup says nothing.
func TestAReportSaysWhatItCannotDraw(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		needsScript bool
	}{
		{name: "a report written entirely in markup"},
		{name: "a report that draws part of itself with a script", needsScript: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			view := ReportView{Name: "daily-briefing.html", Sidebar: NewSidebar(&nav.Model{}, ""), NeedsScript: tt.needsScript}
			if err := Report(view, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			html := buf.String()
			if got := strings.Contains(html, "data-report-inert"); got != tt.needsScript {
				t.Errorf("the report's inert notice present = %t, want %t", got, tt.needsScript)
			}
			if !strings.Contains(html, `sandbox=""`) {
				t.Errorf("the report frame lost its bare sandbox; html = %q", html)
			}
		})
	}
}
