package render

import (
	"strings"
	"testing"
)

func TestStripObsidianCommentsPreservesFenceOpeningLine(t *testing.T) {
	t.Parallel()

	body := "```text %%literal info%%\n%%literal body%%\n```\nafter %%hidden%%"
	want := "```text %%literal info%%\n%%literal body%%\n```\nafter "
	got, line := stripObsidianComments(body)
	if got != want {
		t.Errorf("stripObsidianComments() = %q, want %q", got, want)
	}
	if line != 0 {
		t.Errorf("stripObsidianComments() unclosed line = %d, want 0", line)
	}
}

// TestStripObsidianCommentsReportsUnclosedLine holds both halves of what the
// scan now answers: the bytes that survive, and which line a marker that never
// met its pair was written on. They are asserted together because they can
// only be wrong together — a scan that mistakes a percent sign inside a code
// span for a marker both hides the wrong words and blames the wrong line, and
// a case checking one without the other would report half of that.
//
// The expected bytes are written out by hand rather than derived from the
// input, including the double spaces a removed marker leaves behind.
func TestStripObsidianCommentsReportsUnclosedLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		want     string
		wantLine int
	}{
		{
			name: "no marker at all",
			body: "plain text\nmore text",
			want: "plain text\nmore text",
		},
		{
			name: "paired on one line",
			body: "before %%hidden%% after",
			want: "before  after",
		},
		{
			name: "paired across lines",
			body: "before\n%%hidden first\nhidden second%%\nafter",
			want: "before\n\n\nafter",
		},
		{
			name:     "unclosed from the third line",
			body:     "before\nmiddle\n%%hidden\nmore",
			want:     "before\nmiddle\n\n",
			wantLine: 3,
		},
		{
			name:     "unclosed from the first line",
			body:     "%%hidden\nmore",
			want:     "\n",
			wantLine: 1,
		},
		{
			// A fence hands its lines over as written, so a marker inside one
			// is a code sample rather than a marker and opens nothing.
			name: "unclosed inside a fence",
			body: "```text\n%%unclosed\n```\nafter",
			want: "```text\n%%unclosed\n```\nafter",
		},
		{
			// The positive lock on code spans: a percent sign an author is
			// displaying stays on the page and shifts no pairing.
			name: "code span holds the marker as text",
			body: "MIDDLE `printf(\"%d%%\")` after",
			want: "MIDDLE `printf(\"%d%%\")` after",
		},
		{
			name: "double backtick span holding a single one",
			body: "a ``x`y%%z`` b",
			want: "a ``x`y%%z`` b",
		},
		{
			// The stated limit: this scan reads one line at a time, so a span
			// the author spread over two is not one, and the marker inside it
			// opens a comment for real.
			name:     "span across lines is not a span",
			body:     "`start\n%%end` after",
			want:     "`start\n",
			wantLine: 2,
		},
		{
			// A backtick that never meets its match is ordinary text, and the
			// markers after it still pair. Reading the rest of the line as
			// code would hide it on the strength of a typo.
			name: "unmatched backtick leaves the markers pairing",
			body: "a ` b %%c%% d",
			want: "a ` b  d",
		},
		{
			// One typo further on: the stray run is text, so the span after
			// it is a span, and the percent signs it holds are still the
			// author's own characters.
			name: "a stray run does not expose the markers in the span after it",
			body: "``a `b%%c%%d` e",
			want: "``a `b%%c%%d` e",
		},
		{
			// Backticks are inert while a comment is open: the marker that
			// closes it is looked for first, so a quoted span inside a comment
			// stays hidden with everything else.
			name: "backticks inside an open comment protect nothing",
			body: "%% hide `code` %% after",
			want: " after",
		},
		{
			name:     "three percent signs open and never close",
			body:     "a %%%b",
			want:     "a ",
			wantLine: 1,
		},
		{
			name: "four percent signs are an empty pair",
			body: "a %%%% b",
			want: "a  b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, line := stripObsidianComments(tt.body)
			if got != tt.want {
				t.Errorf("stripObsidianComments(%q) = %q, want %q", tt.body, got, tt.want)
			}
			if line != tt.wantLine {
				t.Errorf("stripObsidianComments(%q) unclosed line = %d, want %d", tt.body, line, tt.wantLine)
			}
		})
	}
}

func FuzzStripObsidianComments(f *testing.F) {
	for _, seed := range []string{
		"plain text",
		"before %%hidden%% after",
		"%%unclosed",
		"one%%first%%%%second%%two",
		"```text\n%%literal%%\n```",
		"```text %%literal info%%\n%%literal%%\n```",
		"%%hidden%% ```text\n%%literal%%\n```",
		"MIDDLE `printf(\"%d%%\")` after",
		"a ``x`y%%z`` b",
		"`start\n%%end` after",
		"a ` b %%c%% d",
		"%% hide `code` %% after",
		// Removing the matched pair here leaves two backtick runs touching,
		// which is the one shape that makes a second scan read the line
		// differently from the first.
		"``%%``%%%%`",
		"``a%%b``%%note%%`c`",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body string) {
		got, line := stripObsidianComments(body)
		if second, secondLine := stripObsidianComments(body); second != got || secondLine != line {
			t.Fatalf("stripObsidianComments() is not deterministic: first %q/%d, second %q/%d", got, line, second, secondLine)
		}
		if len(got) > len(body) {
			t.Fatalf("stripObsidianComments() length = %d, want at most input length %d", len(got), len(body))
		}
		// The line number addresses the body it was read from, so it can only
		// name a line that body has. Nothing is claimed about what the number
		// becomes on a second pass: stripping removes the marker it named, so
		// a body that reported one legitimately reports none afterwards.
		if lines := strings.Count(body, "\n") + 1; line < 0 || line > lines {
			t.Fatalf("stripObsidianComments() unclosed line = %d, want between 0 and %d", line, lines)
		}
		// Running the scan over its own output is not asserted to change
		// nothing, and the seed ``%%``%%%%` is the case that proves it cannot
		// be: removing a matched pair there leaves two backtick runs touching,
		// which merges them, and a percent sign the first pass ruled to be
		// text inside a code span is a live marker to the second. That was
		// harmless while the scan could not see code spans, and it is why no
		// body is scanned twice any more — a body is read where it is first
		// read and nowhere else, which is what TestATranscludedBodyIsScanned-
		// ForCommentsOnce holds. What survives here is the honest half: a
		// second pass can only take more away, never invent text.
		again, _ := stripObsidianComments(got)
		if len(again) > len(got) {
			t.Fatalf("stripObsidianComments(%q) = %q, which is longer than its own input", got, again)
		}
	})
}
