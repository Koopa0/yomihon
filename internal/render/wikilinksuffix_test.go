package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestHTMLPathSuffix(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{
		{RelPath: "Notes/Projects/Atlas/README.md"},
		{RelPath: "Notes/Other/README.md"},
		{RelPath: "Notes/Container/Page.md"},
		{RelPath: "A/Shared/Twin.md"}, {RelPath: "B/Shared/Twin.md"},
	}, []string{"Assets/Atlas/chart.png"}, transclusions{
		"Notes/Projects/Atlas/README.md": "Whole intro.\n\n## Part\n\nSection payload.\n\nBlock payload. ^piece\n\n## Other\n\nOutside payload.\n\n![[Other/README]]\n",
		"Notes/Other/README.md":          "Nested payload.",
		"Notes/Container/Page.md":        "![[Atlas/README#Part]]\n",
	})
	tests := []struct {
		name, body   string
		want, absent []string
	}{
		{name: "link", body: "[[Atlas/README|Chosen]]", want: []string{`<a href="/notes/Notes/Projects/Atlas/README.md" class="wikilink">Chosen</a>`}},
		{name: "heading", body: "[[Atlas/README#Part]]", want: []string{`href="/notes/Notes/Projects/Atlas/README.md#part"`}, absent: []string{"wikilink-degraded"}},
		{name: "block", body: "[[Atlas/README#^piece]]", want: []string{`href="/notes/Notes/Projects/Atlas/README.md#%5Epiece"`}, absent: []string{"wikilink-degraded"}},
		{name: "missing section", body: "[[Atlas/README#Absent]]", want: []string{`href="/notes/Notes/Projects/Atlas/README.md#absent"`, `wikilink-degraded`}},
		{name: "missing block", body: "[[Atlas/README#^absent]]", want: []string{`href="/notes/Notes/Projects/Atlas/README.md"`, `wikilink-degraded`}, absent: []string{"#%5Eabsent"}},
		{name: "whole embed", body: "![[Atlas/README]]", want: []string{`class="embed"`, `href="/notes/Notes/Projects/Atlas/README.md"`, "Whole intro.", "Outside payload.", `href="/notes/Notes/Other/README.md"`}, absent: []string{"Nested payload."}},
		{name: "section embed", body: "![[Atlas/README#Part]]", want: []string{"Section payload.", "Block payload."}, absent: []string{"Whole intro.", "Outside payload."}},
		{name: "block embed", body: "![[Atlas/README#^piece]]", want: []string{"Block payload."}, absent: []string{"Section payload.", "Outside payload."}},
		{name: "heading supplied by embed", body: "[[Container/Page#Part]]", want: []string{`href="/notes/Notes/Container/Page.md#part"`}, absent: []string{"wikilink-degraded"}},
		{name: "picture", body: "![[Atlas/chart.png]]", want: []string{`<img src="/raw/Assets/Atlas/chart.png" alt="chart.png">`}},
		{name: "ambiguous", body: "[[Shared/Twin]]", want: []string{`class="wikilink-ambiguous"`, "A/Shared/Twin.md, B/Shared/Twin.md"}, absent: []string{"<a "}},
		{name: "missing", body: "[[Nope/README]]", want: []string{`class="wikilink-broken"`}, absent: []string{"<a "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Source.md", "", tt.body, wording.En).HTML
			t.Log("invoked: actual suffix resolution contract")
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("caught: suffix rendering changed: missing %q in %s", want, got)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(got, absent) {
					t.Errorf("caught: suffix rendering changed: unexpected %q in %s", absent, got)
				}
			}
		})
	}
}
