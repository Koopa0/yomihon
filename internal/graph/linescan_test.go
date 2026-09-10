package graph_test

import (
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
