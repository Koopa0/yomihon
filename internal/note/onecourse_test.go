package note_test

import (
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheRailAndTheFootWalkTheSameCourse holds one reading page to one order.
// Where several study paths teach a note, the page walks the one whose domain
// the note shares; where nothing picks a course out, it keeps the folder's
// alphabetical neighbour. Either way both onward controls have to name the
// same lesson: a reader who finishes the prose reaches for the step under it,
// and sending that step somewhere the rail does not go takes them out of the
// course they are reading.
func TestTheRailAndTheFootWalkTheSameCourse(t *testing.T) {
	t.Parallel()

	const (
		current     = "/notes/Writing/lessons/golang/Current.md"
		folderNext  = "/notes/Writing/lessons/golang/Folder%20next.md"
		primaryNext = "/notes/Writing/lessons/golang/Primary%20next.md"
	)
	body := func(title string) string {
		return "---\ntitle: " + title + "\ntype: lesson\ndomain: golang\nstatus: draft\n---\n\nbody\n"
	}
	// The folder sorts Current, Folder next, Other next, Primary next, so the
	// file beside this one is never the lesson any course teaches next.
	notes := map[string]string{
		"Writing/lessons/golang/Current.md":      body("Current"),
		"Writing/lessons/golang/Folder next.md":  body("Folder next"),
		"Writing/lessons/golang/Other next.md":   body("Other next"),
		"Writing/lessons/golang/Primary next.md": body("Primary next"),
	}
	syllabus := func(title, domain, second string) string {
		return "---\ntitle: " + title + "\ntype: study-path\ndomain: " + domain + "\n---\n\n" +
			"## Part {sequence=primary}\n\n- [[Current]]\n- [[" + second + "]]\n"
	}

	tests := []struct {
		name     string
		courses  map[string]string
		wantRail string
		wantFoot string
	}{
		{
			name: "two courses teach it and one of them is its own",
			courses: map[string]string{
				"Maps/Primary path.md":   syllabus("Primary path", "golang", "Primary next"),
				"Maps/Secondary path.md": syllabus("Secondary path", "japanese", "Other next"),
			},
			wantRail: primaryNext,
			wantFoot: primaryNext,
		},
		{
			name: "one course teaches it",
			courses: map[string]string{
				"Maps/Primary path.md": syllabus("Primary path", "golang", "Primary next"),
			},
			wantRail: primaryNext,
			wantFoot: primaryNext,
		},
		{
			// Two courses and neither is this note's own: nothing here knows
			// which one the reader is walking, so the rail carries the folder
			// and the foot has to walk it too.
			name: "two courses teach it and neither is its own",
			courses: map[string]string{
				"Maps/Primary path.md":   syllabus("Primary path", "japanese", "Primary next"),
				"Maps/Secondary path.md": syllabus("Secondary path", "meta", "Other next"),
			},
			wantRail: "",
			wantFoot: folderNext,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			files := make(map[string]string, len(notes)+len(tt.courses))
			maps.Copy(files, notes)
			maps.Copy(files, tt.courses)
			srv := newServerWithContract(t, writeNotes(t, files), loadHomeContract(t))

			code, page := get(t, srv.Client(), srv.URL+current)
			if code != http.StatusOK {
				t.Fatalf("note page status = %d, want %d", code, http.StatusOK)
			}
			rail := railOnwardHref(t, page)
			foot := footOnwardHref(t, page)
			if rail != tt.wantRail {
				t.Errorf("the rail offers %q as the step onward, want %q", rail, tt.wantRail)
			}
			if foot != tt.wantFoot {
				t.Errorf("the foot offers %q as the step onward, want %q", foot, tt.wantFoot)
			}
			if rail != "" && rail != foot {
				t.Errorf("one page offers two steps onward: the rail goes to %q and the foot to %q", rail, foot)
			}
		})
	}
}

// railOnwardHref is the address the book rail offers as the lesson after this
// one, empty where the rail carries a folder rather than a book.
func railOnwardHref(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, `<nav class="y-lessonsteps"`)
	if start < 0 {
		return ""
	}
	rail, _, closed := strings.Cut(page[start:], "</nav>")
	if !closed {
		t.Fatal("the rail's course steps never close")
	}
	at := strings.Index(rail, wording.RailNextLesson.In(wording.ZhHant))
	if at < 0 {
		return ""
	}
	return linkHref(t, rail, at)
}

// footOnwardHref is the address the foot of the article offers as the step
// after this note.
func footOnwardHref(t *testing.T, page string) string {
	t.Helper()
	foot := stepsBlock(t, page)
	at := strings.Index(foot, "y-steps__link--next")
	if at < 0 {
		return ""
	}
	return linkHref(t, foot, at)
}

// linkHref reads the address of the link whose text or class carries the mark
// at the given offset.
func linkHref(t *testing.T, markup string, at int) string {
	t.Helper()
	open := strings.LastIndex(markup[:at], "<a ")
	if open < 0 {
		t.Fatalf("the step onward is not a link; markup = %q", markup)
	}
	const marker = `href="`
	href := strings.Index(markup[open:], marker)
	if href < 0 {
		t.Fatalf("the step onward carries no address; markup = %q", markup)
	}
	value := markup[open+href+len(marker):]
	end := strings.IndexByte(value, '"')
	if end < 0 {
		t.Fatalf("the step onward has an unterminated address; markup = %q", markup)
	}
	return value[:end]
}
