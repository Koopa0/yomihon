package render

import (
	"strings"
	"testing"
)

func TestListRowOwnEndStopsAtTheNestedList(t *testing.T) {
	t.Parallel()

	htmlOut := "<li>If your notes are not all in English {sequence=local}\n<ul>\n<li>child</li>\n</ul>\n</li>\n"
	const innerStart = len("<li>")
	got := listRowOwnEnd(htmlOut, innerStart)
	want := strings.Index(htmlOut, "<ul>")
	if got != want {
		t.Errorf("listRowOwnEnd = %d (%q), want %d (up to the nested list, not the child)",
			got, htmlOut[innerStart:min(got, len(htmlOut))], want)
	}
}

func TestStripListRowRoleLeavesAQuotedMarker(t *testing.T) {
	t.Parallel()

	own := "a row naming <code>{sequence=local}</code>\n"
	if got := stripListRowRole(own); got != own {
		t.Errorf("stripListRowRole(%q) = %q, want the quoted marker left in place", own, got)
	}
}

func TestStripListRowRoleLeavesALineThatIsOnlyTheMarker(t *testing.T) {
	t.Parallel()

	own := "{sequence=local}\n"
	if got := stripListRowRole(own); got != own {
		t.Errorf("stripListRowRole(%q) = %q, want the line as written", own, got)
	}
}
