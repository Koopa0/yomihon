package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
)

func TestObsidianCommentsExcludedFromAllProjections(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	tests := []struct {
		name    string
		body    string
		present []string
		absent  []string
	}{
		{
			name:    "inline",
			body:    "before %%hidden inline%% after",
			present: []string{"before", "after"},
			absent:  []string{"hidden inline", "%%"},
		},
		{
			name:    "multiline",
			body:    "before\n%%hidden first\nhidden second%%\nafter",
			present: []string{"before", "after"},
			absent:  []string{"hidden first", "hidden second", "%%"},
		},
		{
			name:    "unclosed",
			body:    "before %%hidden through\nthe end",
			present: []string{"before"},
			absent:  []string{"hidden through", "the end", "%%"},
		},
		{
			name:    "empty",
			body:    "before %%%% after",
			present: []string{"before", "after"},
			absent:  []string{"%%"},
		},
		{
			name:    "adjacent",
			body:    "one%%first%%%%second%%two",
			present: []string{"one", "two"},
			absent:  []string{"first", "second", "%%"},
		},
		{
			name:    "fenced literal",
			body:    "before\n```text\n%%literal example%%\n```\nafter %%hidden outside%%",
			present: []string{"before", "%%literal example%%", "after"},
			absent:  []string{"hidden outside"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projections := map[string]string{
				"HTML":          r.HTML("note.md", "", tt.body).HTML,
				"PlainText":     render.PlainText(tt.body),
				"PlainSections": joinedSectionText(render.PlainSections(tt.body)),
			}
			for projection, got := range projections {
				for _, want := range tt.present {
					if !strings.Contains(got, want) {
						t.Errorf("%s projection = %q, want substring %q", projection, got, want)
					}
				}
				for _, unwanted := range tt.absent {
					if strings.Contains(got, unwanted) {
						t.Errorf("%s projection = %q, must not contain %q", projection, got, unwanted)
					}
				}
			}
		})
	}
}

func TestObsidianCommentsExcludedFromEmbeddedNotes(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{Path: "Embedded.md"}}, nil, transclusions{
		"Embedded.md": "visible embed %%hidden embed%%",
	})

	got := r.HTML("note.md", "", "![[Embedded]]").HTML
	if !strings.Contains(got, "visible embed") {
		t.Errorf("HTML() = %q, want embedded visible text", got)
	}
	if strings.Contains(got, "hidden embed") || strings.Contains(got, "%%") {
		t.Errorf("HTML() = %q, must not contain the embedded Obsidian comment", got)
	}
}

// unclosedCommentDiagnostics collects what a page said about markers on it
// that opened a comment and never met a second one.
func unclosedCommentDiagnostics(page *render.Result) []string {
	var messages []string
	for _, d := range page.Diagnostics {
		if d.Kind == render.DiagCommentUnclosed {
			messages = append(messages, d.Message)
		}
	}
	return messages
}

// TestUnclosedCommentReportsADiagnostic closes the case where this package
// broke its own promise never to return a blank page. A note whose first line
// opened a comment that never closed rendered as nothing at all, and said
// nothing about why — the words were on disk, the page was empty, and the
// reader had no way to tell a swallowed note from an empty one.
//
// The hiding itself is right and stays: Obsidian hides the same words, so
// restoring them here would make the two disagree about one file. What was
// missing was the account of it.
//
// The controls are the other half. A note with no marker and a note whose
// markers pair must report nothing, or a check that reported on every page
// would satisfy the first half while telling the reader nothing.
func TestUnclosedCommentReportsADiagnostic(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "marker in prose swallows the rest",
			body: "before\n\n完成度 50%% 而已。\n\nafter\n",
			want: "an unclosed %% comment opened at line 3 of the note body hides everything after it",
		},
		{
			name: "marker on the first line empties the page",
			body: "%% 這是註解\n\nbefore\n\nafter\n",
			want: "an unclosed %% comment opened at line 1 of the note body hides everything after it",
		},
		{name: "no marker anywhere", body: "before\n\n沒有任何百分號。\n\nafter\n"},
		{name: "markers that pair", body: "before\n\n%% 這段是註解 %%\n\nafter\n"},
		{
			name: "marker displayed inside a code span",
			body: "before\n\n行內 `printf(\"%d%%\")` 這樣。\n\nafter\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := r.HTML("note.md", "", tt.body)
			messages := unclosedCommentDiagnostics(&got)
			switch {
			case tt.want == "":
				if len(messages) != 0 {
					t.Errorf("a note whose markers are in order was reported on: %q", messages)
				}
			case len(messages) != 1 || messages[0] != tt.want:
				t.Errorf("unclosed-comment diagnostics = %q, want exactly one reading %q", messages, tt.want)
			}
		})
	}
}

// TestUnclosedCommentInsideANestedBodyReachesThePage holds the report's reach.
// A callout and a transcluded excerpt are each parsed as a body of their own,
// so a marker left open inside one is found by a separate scan from the one
// that reads the host note. Those scans answer to the page the reader is on:
// an excerpt that stops mid-sentence looks exactly like an excerpt that ended,
// and the reader is looking at the citing note, not the cited one.
func TestUnclosedCommentInsideANestedBodyReachesThePage(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, []graph.NoteInput{{Path: "Embedded.md"}}, nil, transclusions{
		"Embedded.md": "visible embed\n\n%% swallowed from here\n\ngone\n",
	})

	embed := r.HTML("note.md", "", "![[Embedded]]\n")
	if !strings.Contains(embed.HTML, "visible embed") {
		t.Fatalf("the excerpt lost the words before its marker:\n%s", embed.HTML)
	}
	if strings.Contains(embed.HTML, "gone") {
		t.Errorf("the excerpt showed words its own marker hides:\n%s", embed.HTML)
	}
	if messages := unclosedCommentDiagnostics(&embed); len(messages) != 1 {
		t.Errorf("an embedded body's runaway marker did not reach the citing page: %q", messages)
	}

	callout := r.HTML("note.md", "", "> [!note] Title\n> visible callout\n> %% swallowed from here\n")
	if !strings.Contains(callout.HTML, "visible callout") {
		t.Fatalf("the callout lost the words before its marker:\n%s", callout.HTML)
	}
	if messages := unclosedCommentDiagnostics(&callout); len(messages) != 1 {
		t.Errorf("a callout body's runaway marker did not reach the page: %q", messages)
	}
}

// TestATranscludedBodyIsScannedForCommentsOnce holds the rule that a body's
// comments come off exactly once, where that body is first read. It used to be
// twice — an excerpt was scanned when it was cut and again when it was
// rendered — which was harmless only for as long as the scan could not tell a
// percent sign inside a code span from a marker.
//
// It can now, and that makes a second pass destructive. The fixture is the
// shape that shows it: two code spans with a comment between them and no
// spaces anywhere. The first pass keeps both spans, percent signs and all, and
// removes the comment — which leaves the two spans' backticks touching, so
// they merge into one longer run. A second pass reading that merged run finds
// no code span at all, takes the percent signs inside the first span for a
// marker, and hides the rest of the line behind a fault the author never
// wrote.
//
// So the assertions are: the displayed percent signs reach the page, the words
// after them survive, and nothing is reported. A second scan fails all three.
func TestATranscludedBodyIsScannedForCommentsOnce(t *testing.T) {
	t.Parallel()

	const body = "before\n\n``a%%b``%%note%%`c`\n\nafter\n"
	r := newRenderer(t, []graph.NoteInput{{Path: "B.md"}}, nil, transclusions{"B.md": body})

	got := r.HTML("note.md", "", "![[B]]\n")
	if !strings.Contains(got.HTML, "a%%b") {
		t.Errorf("a percent sign the author was displaying was read as a marker and removed:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, "after") {
		t.Errorf("the excerpt lost the words after the code spans:\n%s", got.HTML)
	}
	if messages := unclosedCommentDiagnostics(&got); len(messages) != 0 {
		t.Errorf("a body whose markers all pair was reported as leaving one open: %q", messages)
	}
	if strings.Contains(got.HTML, "note") {
		t.Errorf("the comment between the two spans was not removed:\n%s", got.HTML)
	}
}

// TestUnclosedCommentInAnotherNoteStaysOffThisPage is the boundary the report
// must not cross. Resolving a link reads the destination's body to check the
// address, and that read is not a rendering of the destination — a note is
// reported on by its own page, so a citing page listing faults in every note
// it names would blame the wrong author on the wrong screen.
func TestUnclosedCommentInAnotherNoteStaysOffThisPage(t *testing.T) {
	t.Parallel()

	r := newRenderer(t, []graph.NoteInput{{Path: "B.md"}}, nil, transclusions{
		"B.md": "# Top\n\n%% swallowed from here\n",
	})

	got := r.HTML("note.md", "", "[[B#Top]]\n")
	if messages := unclosedCommentDiagnostics(&got); len(messages) != 0 {
		t.Errorf("a citing page reported a marker written in the note it links to: %q", messages)
	}
}

func joinedSectionText(sections []render.PlainSection) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		parts = append(parts, section.Text)
	}
	return strings.Join(parts, "\n")
}
