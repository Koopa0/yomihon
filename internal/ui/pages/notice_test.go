package pages

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/ui/layouts"
)

// testNotices are two standing facts with nothing else wrong, so a page that
// draws them has drawn them for their own sake and not beside a finding. The
// second carries no detail, which is a notice with no file to name.
var testNotices = []FolderNotice{
	{Title: "first notice title", Summary: "first notice sentence", Detail: `"first/file.md" and "second/file.md"`},
	{Title: "second notice title", Summary: "second notice sentence"},
}

// TestTheDeskDrawsEveryNoticeItIsGiven holds the desk's half of the contract.
// Each notice is its own block with its own heading, so a reader who has two
// of them is not told about one, and each heading is the one its block is
// labelled by.
func TestTheDeskDrawsEveryNoticeItIsGiven(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Home(HomeView{Notices: testNotices}, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	page := buf.String()
	if got := strings.Count(page, `data-home-block="notice"`); got != len(testNotices) {
		t.Fatalf("the desk drew %d notice blocks from %d notices", got, len(testNotices))
	}
	for i, notice := range testNotices {
		id := "home-notice-title-" + strconv.Itoa(i)
		if !strings.Contains(page, `aria-labelledby="`+id+`"`) || !strings.Contains(page, `id="`+id+`"`) {
			t.Errorf("notice %d is not labelled by a heading of its own (%s)", i, id)
		}
		for _, part := range []string{notice.Title, notice.Summary, strings.ReplaceAll(notice.Detail, `"`, "&#34;")} {
			if !strings.Contains(page, part) {
				t.Errorf("the desk dropped %q from notice %d", part, i)
			}
		}
	}
	if !strings.Contains(page, `data-nothing="notice"`) {
		t.Error("the desk's notice is not drawn as the one notice component every empty or faulted surface uses")
	}

	buf.Reset()
	if err := Home(HomeView{}, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(buf.String(), `data-home-block="notice"`) {
		t.Error("the desk draws a notice for a folder that has none")
	}
}

// TestTheHealthPageDrawsEveryNoticeAndNeverSaysAllClearOverThem holds the
// health page's half. A folder whose only trouble is a notice has no row to
// tabulate, which is exactly where the page used to say everything was clear.
func TestTheHealthPageDrawsEveryNoticeAndNeverSaysAllClearOverThem(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := Health(HealthView{Notices: testNotices}, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	page := buf.String()
	if got := strings.Count(page, "data-health-notice"); got != len(testNotices) {
		t.Fatalf("the health page drew %d notices from %d", got, len(testNotices))
	}
	for i, notice := range testNotices {
		for _, part := range []string{notice.Title, notice.Summary, strings.ReplaceAll(notice.Detail, `"`, "&#34;")} {
			if !strings.Contains(page, part) {
				t.Errorf("the health page dropped %q from notice %d", part, i)
			}
		}
	}
	if strings.Contains(page, `data-nothing="health"`) {
		t.Error("the health page says everything is clear over a notice that says changes are not being published")
	}

	// Beside a finding, the notice is drawn ahead of the table it qualifies:
	// a table is sorted and paged, and a notice below one is a notice a long
	// report pushes off the screen.
	buf.Reset()
	withRow := HealthView{Notices: testNotices, Blocked: []HealthBlockedSource{{Path: "Notes/a.md", Reason: "denied"}}}
	if err := Health(withRow, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	notice, table := strings.Index(buf.String(), "data-health-notice"), strings.Index(buf.String(), `class="y-findings"`)
	if notice < 0 || table < 0 || notice > table {
		t.Errorf("the notice is at %d and the table at %d, want the notice drawn first", notice, table)
	}

	buf.Reset()
	if err := Health(HealthView{}, layouts.Chrome{}).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(buf.String(), "data-health-notice") {
		t.Error("the health page draws a notice for a folder that has none")
	}
}
