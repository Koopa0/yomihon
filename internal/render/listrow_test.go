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

func TestTaskListRowRolesPreserveTheirWrapperBytes(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, own, want string }{
		{name: "checked tight", own: `<label class="y-task"><input checked="" disabled="" type="checkbox"> Task {sequence=local}</label>`, want: `<label class="y-task"><input checked="" disabled="" type="checkbox"> Task</label>`},
		{name: "unchecked loose", own: "<p><label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Task {sequence=primary}</label></p>\n", want: "<p><label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Task</label></p>\n"},
		{name: "continuation outside", own: "<p><label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Task {sequence=none}</label></p>\n<p>Other {sequence=local}</p>\n", want: "<p><label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Task</label></p>\n<p>Other {sequence=local}</p>\n"},
		{name: "first line before continuation", own: "<label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Task {sequence=local}\ncontinued</label>", want: "<label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Task\ncontinued</label>"},
		{name: "continuation declaration retained", own: "<label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Task\ncontinued {sequence=local}</label>", want: "<label class=\"y-task\"><input disabled=\"\" type=\"checkbox\"> Task\ncontinued {sequence=local}</label>"},
		{name: "native marker only", own: `<label class="y-task"><input disabled="" type="checkbox"> {sequence=local}</label>`, want: `<label class="y-task"><input disabled="" type="checkbox"> {sequence=local}</label>`},
		{name: "code quotation", own: `<label class="y-task"><input disabled="" type="checkbox"> Task <code>{sequence=local}</code></label>`, want: `<label class="y-task"><input disabled="" type="checkbox"> Task <code>{sequence=local}</code></label>`},
		{name: "tag quotation", own: `<label class="y-task"><input disabled="" type="checkbox"> Task <span>{sequence=local}</span></label>`, want: `<label class="y-task"><input disabled="" type="checkbox"> Task <span>{sequence=local}</span></label>`},
		{name: "nonterminal", own: `<label class="y-task"><input disabled="" type="checkbox"> Task {sequence=local} tail</label>`, want: `<label class="y-task"><input disabled="" type="checkbox"> Task {sequence=local} tail</label>`},
		{name: "unknown", own: `<label class="y-task"><input disabled="" type="checkbox"> Task {sequence=supplementary}</label>`, want: `<label class="y-task"><input disabled="" type="checkbox"> Task {sequence=supplementary}</label>`},
		{name: "duplicate", own: `<label class="y-task"><input disabled="" type="checkbox"> Task {sequence=primary} {sequence=local}</label>`, want: `<label class="y-task"><input disabled="" type="checkbox"> Task {sequence=primary} {sequence=local}</label>`},
		{name: "incomplete", own: `<label class="y-task"><input disabled="" type="checkbox"> Task {sequence=local</label>`, want: `<label class="y-task"><input disabled="" type="checkbox"> Task {sequence=local</label>`},
		{name: "authored label", own: `<label>Task {sequence=local}</label>`, want: `<label>Task {sequence=local}</label>`},
		{name: "different input", own: `<label class="y-task"><input type="text"> Task {sequence=local}</label>`, want: `<label class="y-task"><input type="text"> Task {sequence=local}</label>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := stripListRowRole(tt.own); got != tt.want {
				t.Errorf("caught: task row role wrapper = %q, want %q", got, tt.want)
			}
		})
	}
}
