package render_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// The golden files under testdata/golden are this package's frozen output: one
// fixture per dialect surface, and beside it every byte the render produced for
// it. They exist because the rest of this package's tests assert substrings,
// which pins the shape a test happened to name and nothing else — a pass that
// quietly stopped emitting an id, moved a class, or renumbered a footnote would
// leave every one of them green.
//
// There is no flag that rewrites them, and the reason is the second of the
// three standards written down beside the regeneration tool the wire goldens
// have (internal/judge, TestRegenerateGoldens): a change to what this package
// emits is a change to what readers see, so the new bytes go in by hand and the
// diff is the review.
//
// Each golden holds the rendered HTML followed by a comment carrying the rest of
// what one render answers — the anchor the page title inherited, the contents in
// document order, and the faults reported — because a pass can change any of
// those three without moving a byte of the HTML.

// goldenNotes is the vault every golden fixture is rendered against: two notes
// answering to one name so an ambiguous link has candidates, one note that is
// never written so a broken link has a target, and one picture so an embed can
// resolve to bytes rather than to a body.
var goldenNotes = []graph.NoteInput{
	{RelPath: "Notes/Target Note.md"},
	{RelPath: "Notes/Concept Note.md"},
	{RelPath: "Notes/Embedded Note.md"},
	{RelPath: "A/Twin.md"},
	{RelPath: "B/Twin.md"},
}

// goldenBodies is the captured body set the embeds and the link-fragment checks
// read. Each is written to carry both a heading and a named block, so an address
// that lands and one that misses are both exercised against the same note.
var goldenBodies = transclusions{
	"Notes/Target Note.md":   "## Heading Inside\n\nThe paragraph a block address names. ^para\n",
	"Notes/Embedded Note.md": "## Heading Inside\n\nAn embedded paragraph, with ![[Deeper Note]] one level too far.\n",
}

// goldenTitles answers for exactly one name, so the title-only diagnostic has
// something to report and every other unresolved name stays broken.
type goldenTitles struct{}

func (goldenTitles) TitledBy(name string) []string {
	if name == "A Declared Title" {
		return []string{"Titled Note"}
	}
	return nil
}

// goldenConcepts is the concept lookup the trigger pass is given: one note is a
// concept, everything else is an ordinary link.
func goldenConcepts(relPath string) (string, bool) {
	if relPath == "Notes/Concept Note.md" {
		return "c-1", true
	}
	return "", false
}

// injection names the pass a fixture is rendered through after the pipeline has
// finished with it. Both are gated on a lesson by their caller rather than by
// this package, so a golden has to ask for one explicitly.
type injection int

const (
	injectNone injection = iota
	injectReadAloud
	injectConcepts
)

func TestTheRenderedBytesAreTheOnesTheGoldensHold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		relPath string
		title   string
		region  string
		inject  injection
	}{
		// The title matches the fixture's opening heading, so this one also
		// pins the anchor that heading hands to the page title, and the
		// collision a later section of the same name then has to avoid.
		{name: "headings", relPath: "Notes/Duplicate Title.md", title: "Duplicate Title"},
		// A region prefix, so the footnote ids a second body on one page would
		// mint are frozen alongside everything else.
		{name: "dialect", relPath: "Notes/Dialect.md", region: "sheet-"},
		// Two directories deep, so a source climbing out of the vault has
		// somewhere to climb from.
		{name: "assets", relPath: "Notes/Sub/Assets.md"},
		// Destinations carrying an ampersand, raw and entity-spelled, so the
		// one HTML escape an attribute receives stays one: a second layer
		// would send the browser to a different address.
		{name: "destinations", relPath: "Notes/Destinations.md"},
		{name: "readaloud", relPath: "Notes/Read Aloud.md", inject: injectReadAloud},
		{name: "concept", relPath: "Notes/Concept User.md", inject: injectConcepts},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixture := "testdata/golden/" + tt.name + ".md"
			body, err := os.ReadFile(fixture) // #nosec G304 -- a fixture name from this test's own table
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			r := render.New(graph.BuildFromNotes(goldenNotes, nil), goldenBodies, goldenTitles{}, vaultHolds{})
			res := r.HTMLIn(tt.region, tt.relPath, tt.title, string(body), wording.ZhHant)

			var refs []string
			switch tt.inject {
			case injectNone:
			case injectReadAloud:
				res.HTML = render.InjectTTS(res.HTML, wording.ZhHant)
			case injectConcepts:
				res.HTML, refs = render.InjectConceptTriggers(res.HTML, goldenConcepts)
			}

			golden := "testdata/golden/" + tt.name + ".golden.html"
			want, err := os.ReadFile(golden) // #nosec G304 -- a golden name from this test's own table
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			got := goldenReport(&res, refs)
			if got != string(want) {
				t.Errorf("rendered bytes differ from %s.\n"+
					"If the change is intended, replace that file with the bytes below and let the diff be the review.\n"+
					"--- got ---\n%s\n--- want ---\n%s", golden, got, want)
			}
		})
	}
}

// goldenReport writes one render's whole answer: the HTML a reader receives,
// then the three things it carries that no byte of that HTML would reveal.
func goldenReport(res *render.Result, refs []string) string {
	var b strings.Builder
	b.WriteString(res.HTML)
	if !strings.HasSuffix(res.HTML, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("<!--\ntitle anchor: ")
	b.WriteString(quoted(res.TitleAnchor))
	b.WriteString("\n\ncontents:\n")
	if len(res.TOC) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, entry := range res.TOC {
		fmt.Fprintf(&b, "  h%d %s id=%s\n", entry.Level, quoted(entry.Text), quoted(entry.ID))
	}
	b.WriteString("\ndiagnostics:\n")
	if len(res.Diagnostics) == 0 {
		b.WriteString("  (none)\n")
	}
	for i := range res.Diagnostics {
		d := &res.Diagnostics[i]
		fmt.Fprintf(&b, "  %s target=%s section=%s block=%s\n    %s\n",
			d.Kind, quoted(d.Target), quoted(d.Section), quoted(d.Block), quoted(d.Message))
	}
	if refs != nil {
		b.WriteString("\nconcept references:\n")
		for _, ref := range refs {
			fmt.Fprintf(&b, "  %s\n", quoted(ref))
		}
	}
	b.WriteString("-->\n")
	return b.String()
}

// quoted keeps a report line on one line and shows an empty value as one, so a
// field that stopped being set is visible in the diff rather than absent from it.
func quoted(s string) string { return fmt.Sprintf("%q", s) }
