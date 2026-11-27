package graph_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
)

// TestFenceOpens is CommonMark's opening rule. The run length is what a closer
// has to meet, and four columns of indent make the line indented code: a fence
// an author writes as an example would otherwise never meet a closer, and the
// zone it starts would hide the rest of the note.
func TestFenceOpens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		line       string
		wantMarker byte
		wantN      int
		wantOK     bool
	}{
		{name: "backticks", line: "```", wantMarker: '`', wantN: 3, wantOK: true},
		{name: "an info string after the run", line: "```text", wantMarker: '`', wantN: 3, wantOK: true},
		{name: "four backticks", line: "````text", wantMarker: '`', wantN: 4, wantOK: true},
		{name: "tildes", line: "~~~", wantMarker: '~', wantN: 3, wantOK: true},
		{name: "five tildes", line: "~~~~~foo", wantMarker: '~', wantN: 5, wantOK: true},
		{name: "two markers are not a fence", line: "``"},
		{name: "plain text", line: "plain"},
		{name: "one space", line: " ```", wantMarker: '`', wantN: 3, wantOK: true},
		{name: "two spaces", line: "  ```", wantMarker: '`', wantN: 3, wantOK: true},
		{name: "three spaces, the deepest CommonMark allows", line: "   ```", wantMarker: '`', wantN: 3, wantOK: true},
		{name: "four spaces is indented code", line: "    ```"},
		{name: "four spaces of tildes is indented code", line: "    ~~~"},
		{name: "eight spaces is indented code", line: "        ```"},
		{name: "a tab is four columns, so already too deep", line: "\t```"},
		{name: "three spaces then a tab", line: "   \t```"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			marker, n, ok := graph.FenceOpens(tt.line)
			if marker != tt.wantMarker || n != tt.wantN || ok != tt.wantOK {
				t.Errorf("FenceOpens(%q) = (%q, %d, %v), want (%q, %d, %v)",
					tt.line, marker, n, ok, tt.wantMarker, tt.wantN, tt.wantOK)
			}
		})
	}
}

// TestFenceCloses is CommonMark's closing rule: all of the opener's marker, at
// least as long as its run, at most three spaces in. A shorter or deeper run is
// text the block shows, which is how a note displays a fence inside one
// without ending it.
func TestFenceCloses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		line      string
		marker    byte
		openerLen int
		want      bool
	}{
		{name: "ordinary backtick closer", line: "```", marker: '`', openerLen: 3, want: true},
		{name: "ordinary tilde closer", line: "~~~", marker: '~', openerLen: 3, want: true},
		{name: "shorter backtick run stays content", line: "```", marker: '`', openerLen: 4, want: false},
		{name: "equal backtick run closes", line: "````", marker: '`', openerLen: 4, want: true},
		{name: "longer backtick run closes", line: "`````", marker: '`', openerLen: 4, want: true},
		{name: "shorter tilde run stays content", line: "~~~", marker: '~', openerLen: 4, want: false},
		{name: "equal tilde run closes", line: "~~~~", marker: '~', openerLen: 4, want: true},
		{name: "info string is not a close", line: "```text", marker: '`', openerLen: 3, want: false},
		{name: "one space closes", line: " ```", marker: '`', openerLen: 3, want: true},
		{name: "two spaces close", line: "  ```", marker: '`', openerLen: 3, want: true},
		{name: "three spaces, the deepest CommonMark allows", line: "   ```", marker: '`', openerLen: 3, want: true},
		{name: "four spaces is code the block shows", line: "    ```", marker: '`', openerLen: 3, want: false},
		{name: "five spaces is code the block shows", line: "     ```", marker: '`', openerLen: 3, want: false},
		{name: "eight spaces is code the block shows", line: "        ```", marker: '`', openerLen: 3, want: false},
		{name: "a tab is four columns, so already too deep", line: "\t```", marker: '`', openerLen: 3, want: false},
		{name: "three spaces then a tab", line: "   \t```", marker: '`', openerLen: 3, want: false},
		{name: "four spaces of tildes is code the block shows", line: "    ~~~", marker: '~', openerLen: 3, want: false},
		{name: "trailing spaces still close", line: "```   ", marker: '`', openerLen: 3, want: true},
		{name: "a carriage return still closes", line: "```\r", marker: '`', openerLen: 3, want: true},
		{name: "spaces on both sides still close", line: "  ```  ", marker: '`', openerLen: 3, want: true},
		{name: "indent does not excuse a short run", line: "  ```", marker: '`', openerLen: 4, want: false},
		{name: "nor do spaces on both sides", line: "  ```  ", marker: '`', openerLen: 4, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := graph.FenceCloses(tt.line, tt.marker, tt.openerLen); got != tt.want {
				t.Errorf("FenceCloses(%q, %q, %d) = %v, want %v", tt.line, tt.marker, tt.openerLen, got, tt.want)
			}
		})
	}
}

// TestLineSkipZones pins the ranges a heading walk and the map's link scan
// both skip. A zone that ends early hands code to the reader as prose, and one
// that never ends hides everything the note has left.
func TestLineSkipZones(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		hidden string // empty when the body hides nothing
		shown  string
	}{
		{
			name:   "HTML block",
			body:   "## Real\n<div>\n## Hidden\n</div>\n\n## After\n",
			hidden: "## Hidden\n",
			shown:  "## After\n",
		},
		{
			name:   "backtick fence",
			body:   "## Real\n```\n## Hidden\n```\n## After\n",
			hidden: "## Hidden\n",
			shown:  "## After\n",
		},
		{
			name:   "tilde fence",
			body:   "## Real\n~~~\n## Hidden\n~~~\n## After\n",
			hidden: "## Hidden\n",
			shown:  "## After\n",
		},
		{
			name:   "a shorter backtick run inside a longer fence is content",
			body:   "## Real\n````\n```\n## Hidden\n````\n## After\n",
			hidden: "## Hidden\n",
			shown:  "## After\n",
		},
		{
			name:   "a shorter tilde run inside a longer fence is content",
			body:   "## Real\n~~~~\n~~~\n## Hidden\n~~~~\n## After\n",
			hidden: "## Hidden\n",
			shown:  "## After\n",
		},
		{
			// The indented marker is content the block shows, so the fence
			// runs on to the marker at the margin. Ending it early hands the
			// code text to the reader as prose and turns the real closer into
			// an opening, which then swallows everything the note has left.
			name:   "a fence stays open past a marker written four spaces in",
			body:   "## Real\n```\n    ```\n## Hidden\n```\n## After\n",
			hidden: "## Hidden\n",
			shown:  "## After\n",
		},
		{
			name:  "an indented backtick example hides nothing after it",
			body:  "    ```\n\ntail text\n",
			shown: "tail text",
		},
		{
			name:  "an indented tilde example hides nothing after it",
			body:  "    ~~~\n\ntail text\n",
			shown: "tail text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			offset := func(needle string) int {
				at := strings.Index(tt.body, needle)
				if at < 0 {
					t.Fatalf("body %q does not contain %q", tt.body, needle)
				}
				return at
			}
			zones := graph.LineSkipZones(tt.body)
			if tt.hidden != "" && !graph.In(zones, offset(tt.hidden)) {
				t.Errorf("LineSkipZones(%q) = %v, leaves %q live", tt.body, zones, tt.hidden)
			}
			if graph.In(zones, offset(tt.shown)) {
				t.Errorf("LineSkipZones(%q) = %v, hides %q", tt.body, zones, tt.shown)
			}
		})
	}
}

// TestHeadingsReadsBothFormsTheWayThePageDoes holds the one heading grammar
// the reading page, the check and the sidebar all read. Every row is a shape
// a reader of a note has to get right: the indents
// that make a marked heading and the two that do not, a closing run balanced
// and glued, a run of seven marks, a heading-shaped line inside each kind of
// block that reaches the reader as written, and every line kind that ends a run
// of prose before an underline could title it.
func TestHeadingsReadsBothFormsTheWayThePageDoes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want []graph.Heading
	}{
		{"empty", "", nil},
		{"setext equals", "Title\n=====\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 1, Text: "Title"}}},
		{"setext dashes", "Title\n-----\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "Title"}}},
		{"setext underline indented three", "Title\n   ---\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "Title"}}},
		{"setext underline indented four", "Title\n    ---\n\nprose\n", nil},
		{"multiline setext", "One line\nand another\n=========\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 1, Text: "One line\nand another"}}},
		{"two underlines", "Title\n===\n===\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 1, Text: "Title"}}},
		{"atx indent one", " # One\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 1, Text: "One"}}},
		{"atx indent three", "   ### Deep\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 3, Text: "Deep"}}},
		{"atx indent four", "    #### Not a heading\n\nprose\n", nil},
		{"atx tab indent", "\t# Tabbed in\n\nprose\n", nil},
		{"closing hashes", "## Balanced ##\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "Balanced"}}},
		{"closing hashes glued", "## Glued##\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "Glued##"}}},
		{"tab after the marks", "##\tTabbed\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "Tabbed"}}},
		{"seven marks", "####### Seven\n\n## Real\n", []graph.Heading{{Line: 2, Start: 15, Level: 2, Text: "Real"}}},
		{"hash glued to text", "#NotAHeading\n\n## Real\n", []graph.Heading{{Line: 2, Start: 14, Level: 2, Text: "Real"}}},
		{"marks with no words", "#### \n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 4, Text: ""}}},
		{"trailing spaces", "##  Spaced  \n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "Spaced"}}},
		{"hash in fence", "```\n# Fenced\n```\n\n## Real\n", []graph.Heading{{Line: 4, Start: 18, Level: 2, Text: "Real"}}},
		{"hash in fence indented two", "- item\n\n  ```\n  # Fenced\n  ```\n\n## Real\n", []graph.Heading{{Line: 6, Start: 32, Level: 2, Text: "Real"}}},
		{"hash in fence indented four", "- item\n\n    ```\n    # Fenced\n    ```\n\n## Real\n", []graph.Heading{{Line: 6, Start: 38, Level: 2, Text: "Real"}}},
		{"dedented hash in fence indented two", "- item\n\n  ```\n# Dedented\n  ```\n\n## Real\n", []graph.Heading{{Line: 6, Start: 32, Level: 2, Text: "Real"}}},
		{"hash in html block", "<div>\n# Inside\n</div>\n\n## Real\n", []graph.Heading{{Line: 4, Start: 23, Level: 2, Text: "Real"}}},
		{"hash in raw html block", "<pre>\n# Inside\n</pre>\n\n## Real\n", []graph.Heading{{Line: 4, Start: 23, Level: 2, Text: "Real"}}},
		{"underline in html block", "<div>\nTitle\n=====\n</div>\n\n## Real\n", []graph.Heading{{Line: 5, Start: 26, Level: 2, Text: "Real"}}},
		{"break rule in a paragraph", "prose\n---\nmore prose\n", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "prose"}}},
		{"break rule alone", "prose\n\n---\n\nmore prose\n", nil},
		{"underline after a list item", "- item\n===\n\nprose\n", nil},
		{"underline after a quote", "> quoted\n===\n\nprose\n", nil},
		{"underline after indented code", "prose\n\n    code\n===\n\ntail\n", nil},
		{"underline after a heading", "## Marked\n===\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "Marked"}}},
		{"a CRLF file's closing hashes", "## Balanced ##\r\n\r\nprose\r\n", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "Balanced"}}},
		{"a CRLF file's heading that is only a run", "# #\r\n", []graph.Heading{{Line: 0, Start: 0, Level: 1, Text: "#"}}},
		{"a CRLF file's underlined heading", "Title\r\n=====\r\n\r\nprose\r\n", []graph.Heading{{Line: 0, Start: 0, Level: 1, Text: "Title"}}},
		{"a CRLF file's dashed underline", "Intro\r\n\r\nSub\r\n---\r\n\r\nbody\r\n", []graph.Heading{{Line: 2, Start: 9, Level: 2, Text: "Sub"}}},
		{"a CRLF file's heading over two lines", "One\r\ntwo\r\n---\r\n", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "One\ntwo"}}},
		{"no trailing newline", "## Tail", []graph.Heading{{Line: 0, Start: 0, Level: 2, Text: "Tail"}}},
		{"heading then deeper heading", "# Title\n\nlede\n\n## Two\n\n### Three\n\n## Two again\n", []graph.Heading{{Line: 0, Start: 0, Level: 1, Text: "Title"}, {Line: 4, Start: 15, Level: 2, Text: "Two"}, {Line: 6, Start: 23, Level: 3, Text: "Three"}, {Line: 8, Start: 34, Level: 2, Text: "Two again"}}},
		{"setext then atx", "Words\n=====\n\n## After\n\nprose\n", []graph.Heading{{Line: 0, Start: 0, Level: 1, Text: "Words"}, {Line: 3, Start: 13, Level: 2, Text: "After"}}},
		{"paragraph then blank then underline", "prose\n\n===\n\ntail\n", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := graph.Headings(tt.body, graph.LineSkipZones(tt.body))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Headings(%q) mismatch (-want +got):\n%s", tt.body, diff)
			}
		})
	}
}

// TestHeadingsHidesWhateverTheCallersZonesHide is why the hidden ranges are an
// argument rather than a reading the walk makes for itself. A line walk sees
// fenced code and authored HTML; the map rail also hands over the ranges its
// parsed tree found, and an Obsidian comment is one of them. A heading parked
// in a comment is on neither the page a reader arrives at nor the rail.
func TestHeadingsHidesWhateverTheCallersZonesHide(t *testing.T) {
	t.Parallel()

	body := "%%\n## Parked\n%%\n\n## Live\n"
	lineWalk := graph.Headings(body, graph.LineSkipZones(body))
	want := []graph.Heading{
		{Line: 1, Start: 3, Level: 2, Text: "Parked"},
		{Line: 4, Start: 17, Level: 2, Text: "Live"},
	}
	if diff := cmp.Diff(want, lineWalk); diff != "" {
		t.Errorf("a line walk alone mismatch (-want +got):\n%s", diff)
	}

	zones := append(graph.LineSkipZones(body), graph.CommentZones(body, nil)...)
	withComments := graph.Headings(body, zones)
	wantLive := []graph.Heading{{Line: 4, Start: 17, Level: 2, Text: "Live"}}
	if diff := cmp.Diff(wantLive, withComments); diff != "" {
		t.Errorf("with the comment ranges handed over, mismatch (-want +got):\n%s", diff)
	}
}
