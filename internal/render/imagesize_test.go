package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestImageSizeAndAltSurviveBothAuthoredForms(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, []graph.NoteInput{{RelPath: "N.md"}}, []string{"p.png"}, transclusions{"N.md": "note words\n"})
	tests := []struct{ name, body, want string }{
		{name: "wiki width", body: "![[p.png|300]]", want: `<img src="/raw/p.png" alt="p.png" width="300">`},
		{name: "wiki both", body: "![[p.png|300x200]]", want: `<img src="/raw/p.png" alt="p.png" width="300" height="200">`},
		{name: "wiki alt", body: "![[p.png|some alt]]", want: `<img src="/raw/p.png" alt="some alt">`},
		{name: "wiki escaped alt", body: `![[p.png|a "quote" & <text>]]`, want: `<img src="/raw/p.png" alt="a &#34;quote&#34; &amp; &lt;text&gt;">`},
		{name: "ordinary picture", body: "![[p.png]]", want: `<img src="/raw/p.png" alt="p.png">`},
		{name: "empty alias", body: "![[p.png|]]", want: `<img src="/raw/p.png" alt="p.png">`},
		{name: "markdown width", body: "![alt|300](p.png)", want: `<img src="/raw/p.png" alt="alt" width="300">`},
		{name: "markdown both", body: "![alt|300x200](p.png)", want: `<img src="/raw/p.png" alt="alt" width="300" height="200">`},
		{name: "markdown emphasis", body: "![**alt**|300](p.png)", want: `<img src="/raw/p.png" alt="alt" width="300">`},
		{name: "markdown escaped alt", body: `![a "quote" & words|300](p.png)`, want: `<img src="/raw/p.png" alt="a &quot;quote&quot; &amp; words" width="300">`},
		{name: "markdown empty alt", body: "![|300](p.png)", want: `<img src="/raw/p.png" alt="" width="300">`},
		{name: "markdown literal pipe", body: "![alt|words](p.png)", want: `<img src="/raw/p.png" alt="alt|words">`},
		{name: "markdown earlier pipe", body: "![alt|part|300](p.png)", want: `<img src="/raw/p.png" alt="alt|part" width="300">`},
		{name: "malformed height", body: "![[p.png|300x]]", want: `<img src="/raw/p.png" alt="300x">`},
		{name: "negative is alt", body: "![[p.png|-300]]", want: `<img src="/raw/p.png" alt="-300">`},
		{name: "decimal is alt", body: "![[p.png|3.5]]", want: `<img src="/raw/p.png" alt="3.5">`},
		{name: "units are alt", body: "![[p.png|300px]]", want: `<img src="/raw/p.png" alt="300px">`},
		{name: "unicode digits are alt", body: "![[p.png|３００]]", want: `<img src="/raw/p.png" alt="３００">`},
		{name: "uppercase separator is alt", body: "![[p.png|300X200]]", want: `<img src="/raw/p.png" alt="300X200">`},
		{name: "zero is a pixel count", body: "![[p.png|0]]", want: `<img src="/raw/p.png" alt="p.png" width="0">`},
		{name: "leading zero is numeric", body: "![[p.png|0300x0200]]", want: `<img src="/raw/p.png" alt="p.png" width="0300" height="0200">`},
		{name: "injection stays alt", body: `![[p.png|300" onerror="oops]]`, want: `<img src="/raw/p.png" alt="300&#34; onerror=&#34;oops">`},
		{name: "remote stays link", body: "![alt|300](https://example.org/p.png)", want: `<a href="https://example.org/p.png" rel="external noreferrer" referrerpolicy="no-referrer">alt|300</a>`},
		{name: "unsafe stays label", body: "![alt|300](javascript:alert(1))", want: "alt|300"},
		{name: "note alias stays note", body: "![[N|300]]", want: "note words"},
		{name: "plain link stays link", body: "[[p.png|300]]", want: `<a href="/notes/p.png" class="wikilink">300</a>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("note.md", "", tt.body, wording.En).HTML
			if !strings.Contains(got, tt.want) {
				t.Errorf("HTML(%q) missing %q:\n%s", tt.body, tt.want, got)
			}
			if !strings.Contains(tt.want, "width=") && (strings.Contains(got, " width=") || strings.Contains(got, " height=")) {
				t.Errorf("HTML(%q) sized a non-sized form:\n%s", tt.body, got)
			}
		})
	}
}
