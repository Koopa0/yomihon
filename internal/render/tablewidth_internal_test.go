package render

import (
	"math"
	"strings"
	"testing"
)

// TestATableCellIsCountedByTheWordsAReaderSees holds what a cell is measured
// by: the characters on the page, in the face they are set in. A reading over a
// word sits above it and takes no room along the line; a note only a screen
// reader hears takes none at all; a code span is the mono face at its own size
// with its padding; and a header is counted whole, since it is set on one line.
func TestATableCellIsCountedByTheWordsAReaderSees(t *testing.T) {
	t.Parallel()
	const table = `<thead><tr><th>Ab</th><th>Second</th></tr></thead>` +
		`<tbody><tr><td>漢<ruby>字<rt>じ</rt></ruby> <span class="wikilink"><span class="y-offscreen">(hidden words)</span></span><code>x</code></td><td>a<br>bb</td></tr></tbody></table></div><p>after</p>`
	got := tableColumns(table)
	if len(got) != 2 {
		t.Fatalf("tableColumns() read %d columns, want 2: %+v", len(got), got)
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	// 漢 and 字 at an em each, a space, and the code span: its padding and one
	// mono character. The longest unbroken run is the code span itself.
	if want := 1 + 1 + 0.25 + tableCodePadding + tableCodeAdvance; !near(got[0].line, want) {
		t.Errorf("first column's line = %v, want %v", got[0].line, want)
	}
	if want := tableCodePadding + tableCodeAdvance; !near(got[0].word, want) {
		t.Errorf("first column's word = %v, want %v", got[0].word, want)
	}
	if want := 0.668 + 0.586 + 0.04; !near(got[0].header, want) {
		t.Errorf("first column's header = %v, want %v", got[0].header, want)
	}
	// A break starts a new line, so the column is as wide as its longer one.
	if want := 2 * 0.586; !near(got[1].line, want) {
		t.Errorf("second column's line = %v, want %v (the longer of its two lines)", got[1].line, want)
	}
}

// TestATableIsWideWhereItsColumnsCrowdTheText holds the rule's two edges. A
// column of sentences is counted no wider than the cap, so two such columns fit
// the text at every measure while three do not fit at any; a table of short
// figures fits wherever it is; and a header that cannot break makes its column
// as wide as itself.
func TestATableIsWideWhereItsColumnsCrowdTheText(t *testing.T) {
	t.Parallel()
	sentence := strings.Repeat("讀", 40)
	row := func(cells ...string) string {
		return "<tr><td>" + strings.Join(cells, "</td><td>") + "</td></tr>"
	}
	head := func(cells ...string) string {
		return "<thead><tr><th>" + strings.Join(cells, "</th><th>") + "</th></tr></thead>"
	}
	all := "latin-xl latin-l latin-m xl l m"
	tests := []struct {
		name  string
		table string
		want  string
	}{
		{name: "two columns of sentences", table: head("a", "b") + row(sentence, sentence), want: ""},
		{name: "three columns of sentences", table: head("a", "b", "c") + row(sentence, sentence, sentence), want: all},
		{name: "six columns of figures", table: head("n", "n", "n", "n", "n", "n") + row("1,024", "2,048", "4,096", "8,192", "16,384", "32,768"), want: ""},
		{name: "a header that cannot break", table: head(strings.Repeat("Header ", 14), "b") + row("1", "2"), want: "latin-xl latin-l latin-m xl l m"},
		{name: "a header that fits the Latin measure", table: head(strings.Repeat("標", 35)) + row("1"), want: ""},
		{name: "a header three characters past it", table: head(strings.Repeat("標", 39)) + row("1"), want: "latin-xl latin-l latin-m"},
		{name: "a table with no cells", table: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tableWide(tableColumns(tt.table + "</table>")); got != tt.want {
				t.Errorf("tableWide() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestADiagramIsWideUnlessItRunsDownThePage holds the one thing the server can
// know about a diagram's width: which way it is laid out.
func TestADiagramIsWideUnlessItRunsDownThePage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		source string
		down   bool
	}{
		{source: "graph TD\n  A --> B", down: true},
		{source: "flowchart TB\n  A --> B", down: true},
		{source: "graph BT;\n  A --> B", down: true},
		{source: "graph\n  A --> B", down: true},
		{source: "graph LR\n  A --> B", down: false},
		{source: "flowchart RL\n  A --> B", down: false},
		{source: "%% a comment\n%%{init: {}}%%\n\ngraph LR\n  A --> B", down: false},
		{source: "---\ntitle: Across\n---\nflowchart LR\n  A --> B", down: false},
		{source: "stateDiagram-v2\n  [*] --> Still", down: true},
		{source: "classDiagram\n  direction LR\n  A <|-- B", down: false},
		{source: "erDiagram\n  A ||--o{ B : has", down: true},
		{source: "pie title Pets\n  \"Dogs\" : 3", down: true},
		{source: "gitGraph\n  commit", down: false},
		{source: "gitGraph TB:\n  commit", down: true},
		{source: "sequenceDiagram\n  A->>B: hi", down: false},
		{source: "gantt\n  title Plan", down: false},
		{source: "aNewKindOfDiagram\n  x", down: false},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			if got := diagramRunsDown(tt.source); got != tt.down {
				t.Errorf("diagramRunsDown() = %v, want %v", got, tt.down)
			}
		})
	}
}

// TestADiagramIsCaptionedByItsAccessibleTitle holds where a diagram's caption
// comes from: the accTitle its author wrote, and nothing else. A title in the
// front matter is drawn inside the diagram already.
func TestADiagramIsCaptionedByItsAccessibleTitle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		source string
		want   string
	}{
		{source: "graph LR\n  accTitle: Two transactions & a cycle\n  A --> B", want: "Two transactions & a cycle"},
		{source: "graph LR\n  accTitle:Tight\n  A --> B", want: "Tight"},
		{source: "---\ntitle: Drawn inside\n---\ngraph LR\n  A --> B", want: ""},
		{source: "graph LR\n  accDescr: Only a description\n  A --> B", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			if got := diagramTitle(tt.source); got != tt.want {
				t.Errorf("diagramTitle() = %q, want %q", got, tt.want)
			}
		})
	}
	got := diagramPlate("graph LR\n  accTitle: A < B\n  A --> B", "<div></div>")
	const want = `<figure class="mat mat--page" data-wide="latin-xl latin-l latin-m xl l m"><div></div><figcaption aria-hidden="true">A &lt; B</figcaption></figure>`
	if got != want {
		t.Errorf("diagramPlate() = %q, want %q", got, want)
	}
}
