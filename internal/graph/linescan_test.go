package graph_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
)

// TestLineScanKeepsAShorterInnerFenceAsContent is the CommonMark close rule
// the shared scanner has to keep: a closing run is only a close when it is
// at least as long as the opener. A shorter all-marker line is code, so a
// heading-shaped line after it is still inside the fence.
func TestLineScanKeepsAShorterInnerFenceAsContent(t *testing.T) {
	t.Parallel()

	for _, marker := range []byte{'`', '~'} {
		t.Run(string(marker), func(t *testing.T) {
			t.Parallel()
			outer := string([]byte{marker, marker, marker, marker})
			inner := string([]byte{marker, marker, marker})
			lines := []string{outer, inner, "# Not a heading", outer, "# Real heading"}

			var scan graph.LineScan
			if !scan.Skip(lines[0]) {
				t.Fatalf("Skip(%q) = false, want the opener skipped", lines[0])
			}
			if !scan.Skip(lines[1]) {
				t.Fatalf("Skip(%q) = false, want the shorter inner fence kept as content", lines[1])
			}
			if !scan.Skip(lines[2]) {
				t.Fatalf("Skip(%q) = false, want the heading-shaped line still inside the outer fence", lines[2])
			}
			if !scan.Skip(lines[3]) {
				t.Fatalf("Skip(%q) = false, want the matching closer skipped", lines[3])
			}
			if scan.Skip(lines[4]) {
				t.Fatalf("Skip(%q) = true, want the real heading read after the fence closes", lines[4])
			}
		})
	}
}

// TestLineScanClosesAnOrdinaryFence is the control: a three-marker closer
// still ends a three-marker opener, or a heading after that closer would be
// swallowed as code.
func TestLineScanClosesAnOrdinaryFence(t *testing.T) {
	t.Parallel()

	for _, marker := range []byte{'`', '~'} {
		t.Run(string(marker), func(t *testing.T) {
			t.Parallel()
			fence := string([]byte{marker, marker, marker})
			lines := []string{fence, "# Not a heading", fence, "# Real heading"}

			var scan graph.LineScan
			if !scan.Skip(lines[0]) {
				t.Fatalf("Skip(%q) = false, want the opener skipped", lines[0])
			}
			if !scan.Skip(lines[1]) {
				t.Fatalf("Skip(%q) = false, want the heading-shaped line inside the fence", lines[1])
			}
			if !scan.Skip(lines[2]) {
				t.Fatalf("Skip(%q) = false, want the matching closer skipped", lines[2])
			}
			if scan.Skip(lines[3]) {
				t.Fatalf("Skip(%q) = true, want the real heading read after an ordinary fence", lines[3])
			}
		})
	}
}

// TestFenceClosesNeedsTheOpenerLength pins the close decision itself: a
// three-marker line does not close a four-marker opener, and a longer close
// still ends a shorter one.
func TestFenceClosesNeedsTheOpenerLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		line      string
		marker    byte
		openerLen int
		want      bool
	}{
		{name: "shorter backtick run stays content", line: "```", marker: '`', openerLen: 4, want: false},
		{name: "equal backtick run closes", line: "````", marker: '`', openerLen: 4, want: true},
		{name: "longer backtick run closes", line: "`````", marker: '`', openerLen: 4, want: true},
		{name: "ordinary backtick closer", line: "```", marker: '`', openerLen: 3, want: true},
		{name: "shorter tilde run stays content", line: "~~~", marker: '~', openerLen: 4, want: false},
		{name: "equal tilde run closes", line: "~~~~", marker: '~', openerLen: 4, want: true},
		{name: "ordinary tilde closer", line: "~~~", marker: '~', openerLen: 3, want: true},
		{name: "info string is not a close", line: "```text", marker: '`', openerLen: 3, want: false},
		{name: "trimmed ordinary closer", line: "  ```  ", marker: '`', openerLen: 3, want: true},
		{name: "trimmed shorter run stays content", line: "  ```  ", marker: '`', openerLen: 4, want: false},
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

// TestFenceClosesRefusesAnOverIndentedCloser is the closing half of the
// indent rule the opening half already keeps. CommonMark allows a closing
// marker at most three spaces of indent; written deeper it is text the block
// shows, which is how an author displays a fence without ending the one they
// are inside. The trailing side is untouched, so a closer followed by spaces
// or by a carriage return still closes, and the length rule still decides on
// its own.
func TestFenceClosesRefusesAnOverIndentedCloser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		line      string
		marker    byte
		openerLen int
		want      bool
	}{
		{name: "no indent closes", line: "```", marker: '`', openerLen: 3, want: true},
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
		{name: "indent does not excuse a short run", line: "  ```", marker: '`', openerLen: 4, want: false},
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

// TestLineSkipZonesCoversWhatSkipHides is the range form of Skip: a heading
// shaped line inside an HTML block or either fence sits in the span, and the
// real heading after the zone does not.
func TestLineSkipZonesCoversWhatSkipHides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		in   string
		out  string
	}{
		{
			name: "HTML block",
			body: "## Real\n<div>\n## Hidden\n</div>\n\n## After\n",
			in:   "## Hidden\n",
			out:  "## After\n",
		},
		{
			name: "backtick fence",
			body: "## Real\n```\n## Hidden\n```\n## After\n",
			in:   "## Hidden\n",
			out:  "## After\n",
		},
		{
			name: "tilde fence",
			body: "## Real\n~~~\n## Hidden\n~~~\n## After\n",
			in:   "## Hidden\n",
			out:  "## After\n",
		},
		{
			// The indented marker is content the block shows, so the fence
			// runs on to the marker at the margin. Ending it early hands the
			// code text to the reader as prose and turns the real closer into
			// an opening, which then swallows everything the note has left.
			name: "a fence stays open past a marker written four spaces in",
			body: "## Real\n```\n    ```\n## Hidden\n```\n## After\n",
			in:   "## Hidden\n",
			out:  "## After\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			zones := graph.LineSkipZones(tt.body)
			hidden := strings.Index(tt.body, tt.in)
			after := strings.Index(tt.body, tt.out)
			if hidden < 0 || after < 0 {
				t.Fatalf("fixture lost %q or %q", tt.in, tt.out)
			}
			if !graph.In(zones, hidden) {
				t.Errorf("LineSkipZones() missed %q at %d: %#v", tt.in, hidden, zones)
			}
			if graph.In(zones, after) {
				t.Errorf("LineSkipZones() swallowed %q at %d: %#v", tt.out, after, zones)
			}
		})
	}
}

func TestFenceOpensReportsTheRunLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line       string
		wantMarker byte
		wantN      int
		wantOK     bool
	}{
		{line: "```text", wantMarker: '`', wantN: 3, wantOK: true},
		{line: "````text", wantMarker: '`', wantN: 4, wantOK: true},
		{line: "~~~", wantMarker: '~', wantN: 3, wantOK: true},
		{line: "~~~~~foo", wantMarker: '~', wantN: 5, wantOK: true},
		{line: "  ```", wantMarker: '`', wantN: 3, wantOK: true},
		{line: "``", wantOK: false},
		{line: "plain", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()
			marker, n, ok := graph.FenceOpens(tt.line)
			if ok != tt.wantOK || marker != tt.wantMarker || n != tt.wantN {
				t.Errorf("FenceOpens(%q) = (%q, %d, %v), want (%q, %d, %v)",
					tt.line, marker, n, ok, tt.wantMarker, tt.wantN, tt.wantOK)
			}
		})
	}
}

// TestFenceOpensRefusesAnIndentedExample is CommonMark's opening-indent rule,
// which every other pattern in this scanner already keeps: at most three
// spaces before an opening fence. Four or more makes the line indented code,
// so an author showing what a fence looks like is writing content. Read as an
// opener it never meets a closer, and the exclusion it starts runs to the end
// of the note — taking the lesson rows, headings and links below it with it.
func TestFenceOpensRefusesAnIndentedExample(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		open bool
	}{
		{name: "no indent", line: "```", open: true},
		{name: "one space", line: " ```", open: true},
		{name: "three spaces, the deepest CommonMark allows", line: "   ```", open: true},
		{name: "four spaces is indented code", line: "    ```", open: false},
		{name: "four spaces of tildes is indented code", line: "    ~~~", open: false},
		{name: "eight spaces is indented code", line: "        ```", open: false},
		{name: "a tab is four columns, so already too deep", line: "\t```", open: false},
		{name: "three spaces then a tab", line: "   \t```", open: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, _, got := graph.FenceOpens(tt.line); got != tt.open {
				t.Errorf("FenceOpens(%q) opened = %v, want %v", tt.line, got, tt.open)
			}
		})
	}
}

// TestLineSkipZonesLeavesAnIndentedExampleAlone is the same rule where it is
// felt: the zones an indented example produces, and what a real fence and an
// authored HTML block still produce beside it.
func TestLineSkipZonesLeavesAnIndentedExampleAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		hidden []string
		shown  []string
	}{
		{
			name:  "an indented backtick example hides nothing after it",
			body:  "    ```\n\ntail text\n",
			shown: []string{"tail text"},
		},
		{
			name:  "an indented tilde example hides nothing after it",
			body:  "    ~~~\n\ntail text\n",
			shown: []string{"tail text"},
		},
		{
			name:   "a real fence still hides its contents",
			body:   "```\nhidden text\n```\n\ntail text\n",
			hidden: []string{"hidden text"},
			shown:  []string{"tail text"},
		},
		{
			name:   "an authored HTML block still hides its contents",
			body:   "<div>\nhidden text\n</div>\n\ntail text\n",
			hidden: []string{"hidden text"},
			shown:  []string{"tail text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			zones := graph.LineSkipZones(tt.body)
			covered := func(needle string) bool {
				at := strings.Index(tt.body, needle)
				if at < 0 {
					t.Fatalf("the body does not contain %q, so this case asserts nothing", needle)
				}
				for _, z := range zones {
					if at >= z.Start && at < z.Stop {
						return true
					}
				}
				return false
			}
			for _, needle := range tt.hidden {
				if !covered(needle) {
					t.Errorf("LineSkipZones(%q) leaves %q live, want it hidden", tt.body, needle)
				}
			}
			for _, needle := range tt.shown {
				if covered(needle) {
					t.Errorf("LineSkipZones(%q) hides %q, want it live", tt.body, needle)
				}
			}
		})
	}
}
