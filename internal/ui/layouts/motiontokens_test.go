package layouts

import (
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestComponentMotionIsWrittenInTokens holds the interface to the speeds it
// chose. Motion here exists so a state change can be read: an overlay says
// where it came from and will go back to, a fold says the hidden thing was
// here. Two durations carry all of that, and both are declared once as tokens,
// so a reader's whole sense of how fast this interface moves can be read and
// changed in one place. A number written straight into a rule is invisible
// from there — it is a speed nobody chose, and it is how the overlays came to
// arrive at three different tempos and the hovers at two.
//
// Three values are deliberately not tokens and are named below with the exact
// declaration each belongs to. Two are indeterminate waits — a diagram being
// drawn, an index answering — which turn until the work finishes and so
// measure nothing the reader is waiting a known time for. The third is how
// motion is turned off for a reader who asked for less: a span small enough to
// be imperceptible, chosen over none so that an animation still reports that
// it ended. Each row has to match the sheet exactly once, so a row left behind
// by an edit that removed the declaration it excused turns this red too.
//
// The token file itself is not read here: that is where the durations are
// declared, so it is the one place a number is the point.
func TestComponentMotionIsWrittenInTokens(t *testing.T) {
	t.Parallel()
	const path = "../../../assets/css/components.css"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	css := blankComments(string(source))

	deliberate := []struct {
		why         string
		declaration string
	}{
		{
			why:         "the turning that stands in for a diagram while it is being drawn, which lasts as long as the drawing does",
			declaration: "animation: y-shimmer 1.4s linear infinite;",
		},
		{
			why:         "the turning that stands in for results while the index answers, which lasts as long as the answer takes",
			declaration: "animation: y-search-spin 0.7s linear infinite;",
		},
		{
			why:         "motion turned off for a reader who asked for less, written as a span too short to see rather than as none so an animation still reports that it ended",
			declaration: "animation-duration: 0.001ms !important;",
		},
		{
			why:         "the same for transitions",
			declaration: "transition-duration: 0.001ms !important;",
		},
	}
	for _, row := range deliberate {
		switch count := strings.Count(css, row.declaration); count {
		case 1:
			css = strings.Replace(css, row.declaration, blank(row.declaration), 1)
		case 0:
			t.Fatalf("%s no longer writes %q, which was excused because it is %s; an excuse outliving its declaration excuses whatever moves into its place", path, row.declaration, row.why)
		default:
			t.Fatalf("%s writes %q %d times; this excuses %s, and a second copy of it is a second decision that has not been made", path, row.declaration, count, row.why)
		}
	}

	for _, found := range timeLiterals(css) {
		t.Errorf("%s:%d sets a duration of %s in %q; the interface moves at --dur-base and --dur-slow, and dwells at --dur-dwell, so a rule reaches for one of those rather than writing a number of its own", path, found.line, found.text, found.context)
	}
}

// TestTimeLiteralsReadsEveryWayADurationCanBeWritten proves the reading above
// can see what it is looking for. The durations this repository actually wrote
// into rules were spelled three ways — with the leading zero, without it, and
// in milliseconds — so a reading that knows only one of them would have passed
// over most of them while reporting a clean sheet. The second half of the
// table is the other failure: a number that is not a duration at all, counted
// as one, would make the check red on a sheet with nothing wrong in it, and a
// check whose every result is a false alarm teaches a reader to skip it.
func TestTimeLiteralsReadsEveryWayADurationCanBeWritten(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		css  string
		want []string
	}{
		{"the leading zero left out", ".y-tts { transition: color .12s; }", []string{".12s"}},
		{"the leading zero written", ".y-rail-left { transition: transform 0.18s; }", []string{"0.18s"}},
		{"milliseconds", ".y-metarow::details-content { transition: height 120ms; }", []string{"120ms"}},
		{"whole seconds", ".y-preview { transition: opacity 1s; }", []string{"1s"}},
		{"a fraction of a second", ".y-prose :target { animation: y-arrive 1.1s var(--ease-standard); }", []string{"1.1s"}},
		{"two in one declaration", ".y-slotbtn { transition: color .12s, border-color .12s; }", []string{".12s", ".12s"}},
		{"a delay pulled backwards", ".y-flipreceipt { animation-delay: -1s; }", []string{"-1s"}},
		{"a token", ".y-diag { transition: background var(--dur-base) var(--ease-standard); }", nil},
		{"a digit inside a token's name", ".y-result__meta { font-size: var(--fs-11); }", nil},
		{"a length", ".y-conceptsheet { transform: translateX(16px); }", nil},
		{"a curve", ":root { --ease-standard: cubic-bezier(0.2, 0, 0, 1); }", nil},
		{"a hex colour", ".y-navdot { background: #0ff; }", nil},
		{"a keyframe stop", "@keyframes y-arrive { 50% { opacity: 0; } }", nil},
		{"a width the layout turns at", "@media (min-width: 1280px) { .y-facets { display: block; } }", nil},
		{"a duration named in prose", "/* the fold opens over 120ms, which is --dur-base */ .y-facets { height: 0; }", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, found := range timeLiterals(blankComments(c.css)) {
				got = append(got, found.text)
			}
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("timeLiterals(%q) (-want +got):\n%s", c.css, diff)
			}
		})
	}
}

// timeLiteral is one duration written into a rule, with enough of the rule
// beside it that the report names the thing that moves rather than a number.
type timeLiteral struct {
	line    int
	text    string
	context string
}

// timeLiterals finds every time value in a stylesheet: a number carrying the s
// or ms unit and standing on its own, rather than a digit inside a word such
// as --fs-11 or a length such as 16px.
func timeLiterals(css string) []timeLiteral {
	var out []timeLiteral
	for i := 0; i < len(css); {
		at := i
		if !opensNumber(css, at) {
			i++
			continue
		}
		j := at
		if css[j] == '-' || css[j] == '+' {
			j++
		}
		for j < len(css) && (isDigit(css[j]) || css[j] == '.') {
			j++
		}
		unitAt := j
		for j < len(css) && isLetter(css[j]) {
			j++
		}
		unit := strings.ToLower(css[unitAt:j])
		if unit != "s" && unit != "ms" {
			i = j
			if i == at {
				i++
			}
			continue
		}
		// A word may continue past the unit, in which case the whole of it is a
		// word and not a value: 1st is not one second.
		if j < len(css) && (isDigit(css[j]) || css[j] == '-' || css[j] == '_') {
			i = j
			continue
		}
		out = append(out, timeLiteral{
			line:    strings.Count(css[:at], "\n") + 1,
			text:    css[at:j],
			context: declarationAround(css, at, j),
		})
		i = j
	}
	return out
}

// opensNumber reports whether a value starts at this byte. A digit standing
// after a letter, another digit, a hyphen or a hash continues the word it is
// in; a hyphen opens a value only where it is a sign rather than part of a
// name, which is what tells -1s from --fs-11.
func opensNumber(css string, at int) bool {
	switch {
	case css[at] == '-' || css[at] == '+':
		if at > 0 && (isLetter(css[at-1]) || isDigit(css[at-1]) || css[at-1] == '-' || css[at-1] == '_') {
			return false
		}
		return at+1 < len(css) && (isDigit(css[at+1]) ||
			(css[at+1] == '.' && at+2 < len(css) && isDigit(css[at+2])))
	case isDigit(css[at]), css[at] == '.':
		if at > 0 && continuesWord(css[at-1]) {
			return false
		}
		if css[at] == '.' {
			return at+1 < len(css) && isDigit(css[at+1])
		}
		return true
	default:
		return false
	}
}

func isDigit(b byte) bool  { return b >= '0' && b <= '9' }
func isLetter(b byte) bool { return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') }

// continuesWord reports whether a byte can stand before a digit and make that
// digit part of what it is in, rather than the start of a value of its own.
func continuesWord(b byte) bool {
	return isLetter(b) || isDigit(b) || b == '_' || b == '-' || b == '.' || b == '#'
}

// declarationAround returns the declaration a value sits in, so a report names
// the property and the thing it moves.
func declarationAround(css string, at, end int) string {
	start := strings.LastIndexAny(css[:at], ";{}\n") + 1
	stop := strings.IndexAny(css[end:], ";{}\n")
	if stop < 0 {
		stop = len(css)
	} else {
		stop += end
	}
	return strings.Join(strings.Fields(css[start:stop]), " ")
}

// blankComments replaces each comment with spaces of its own length, keeping
// the newlines, so prose that names a duration is not read as a rule that sets
// one and the line numbers still point into the file.
func blankComments(css string) string {
	out := []byte(css)
	for i := 0; i+1 < len(out); {
		if out[i] != '/' || out[i+1] != '*' {
			i++
			continue
		}
		end := strings.Index(css[i+2:], "*/")
		if end < 0 {
			end = len(out)
		} else {
			end += i + 4
		}
		copy(out[i:end], blank(string(out[i:end])))
		i = end
	}
	return string(out)
}

// blank returns a run of spaces the same length as its argument, with newlines
// kept where they were.
func blank(s string) string {
	out := []byte(s)
	for i := range out {
		if out[i] != '\n' {
			out[i] = ' '
		}
	}
	return string(out)
}
