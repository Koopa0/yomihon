package note_test

import (
	"net/http"
	"strings"
	"testing"
)

// A note's one name is written as text as often as a list of one, and the page
// links it either way: the reader who followed [[Name]] to a note that answers
// to Name is not told the note does not exist.
func TestAnAliasWrittenAsOneNameLinksOnThePage(t *testing.T) {
	t.Parallel()
	for name, aliases := range map[string]string{
		"one piece of text": "aliases: Solo Name",
		"quoted text":       `aliases: "Solo Name"`,
		"a list of one":     "aliases: [Solo Name]",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := writeNotes(t, map[string]string{
				"Concepts/Alias target.md": "---\n" + aliases + "\n---\n# Alias target\n\nIt answers to one name.\n",
				"Concepts/Alias source.md": "# Alias source\n\nSee [[Solo Name]] for it.\n",
			})
			srv := newServer(t, root)
			code, body := get(t, srv.Client(), srv.URL+"/notes/Concepts/Alias%20source.md")
			if code != http.StatusOK {
				t.Fatalf("GET source status = %d", code)
			}
			const link = `href="/notes/Concepts/Alias%20target.md" class="wikilink">Solo Name</a>`
			if !strings.Contains(body, link) {
				t.Errorf("caught: [[Solo Name]] is not linked to the note that answers to it, want %q", link)
			}
			if strings.Contains(body, "wikilink-broken") {
				t.Errorf("caught: [[Solo Name]] is shown as a note that does not exist")
			}
		})
	}
}
