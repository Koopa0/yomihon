package nav

import "testing"

// TestBriefingNameIsTheDailyBriefingPathShape holds the shape the report index
// and the note-address builder share. A written .md report, HTML living
// anywhere else, and a nested file under daily-briefing/ must not match, or
// notesHref would send them to a report page that cannot serve them.
func TestBriefingNameIsTheDailyBriefingPathShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
		ok   bool
	}{
		{
			name: "a briefing html directly under daily-briefing",
			path: "System/reports/daily-briefing/browser-boundary.html",
			want: "browser-boundary.html",
			ok:   true,
		},
		{
			name: "latest.html is a briefing",
			path: "System/reports/daily-briefing/latest.html",
			want: "latest.html",
			ok:   true,
		},
		{
			name: "a written markdown report is not a briefing",
			path: "System/reports/Week of 2026-08-31.md",
		},
		{
			name: "unregistered html elsewhere is not a briefing",
			path: "Notes/page.html",
		},
		{
			name: "a nested file under daily-briefing is not a briefing",
			path: "System/reports/daily-briefing/sub/x.html",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := BriefingName(tt.path)
			if ok != tt.ok || got != tt.want {
				t.Errorf("BriefingName(%q) = %q, %v; want %q, %v", tt.path, got, ok, tt.want, tt.ok)
			}
		})
	}
}
