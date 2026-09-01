package render

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

type fuzzTransclusions map[string]string

func (b fuzzTransclusions) Transclusion(path string) (string, bool) {
	body, ok := b[path]
	return body, ok
}

func FuzzHTML(f *testing.F) {
	for _, seed := range []string{
		"",
		"plain <ruby>日<rt>にち</rt></ruby> text",
		"# Duplicate title\n## 見出し\n[[missing|表示]]",
		"```go\r\n[[literal]]\r\n> [!note] literal\r\n```",
		"```go\nunclosed [[literal]]",
		"> [!unknown] Title\n> body with **formatting**",
		"```mermaid\ngraph TD; A-->B\n```",
		"![[missing embed]] and [[unterminated",
		"before ![[target]] after",
		"before %%hidden%% after\n```text %%literal info%%\n%%literal%%\n```",
		"## 重複\n## 重複\n<h2>raw heading</h2>",
	} {
		f.Add(seed)
	}

	const targetPath = "target.md"
	renderer := New(
		graph.BuildFromNotes([]graph.NoteInput{{RelPath: targetPath}}, nil),
		fuzzTransclusions{targetPath: "## Embedded\nbody with ![[target]]"},
		internalNoTitles{},
	)
	f.Fuzz(func(t *testing.T, body string) {
		const maxInput = 32 << 10
		if len(body) > maxInput {
			return
		}

		first := renderer.HTML("note.md", "", body, wording.ZhHant)
		second := renderer.HTML("note.md", "", body, wording.ZhHant)
		if diff := cmp.Diff(first, second); diff != "" {
			t.Fatalf("Pipeline.HTML(, wording.ZhHant) is not deterministic (-first +second):\n%s", diff)
		}

		const fixedOverhead = 4 << 10
		const maxExpansion = 256
		if size := resultBytes(&first); size > fixedOverhead+maxExpansion*len(body) {
			t.Fatalf("Pipeline.HTML(, wording.ZhHant) output uses %d bytes for %d input bytes", size, len(body))
		}
		if len(first.TOC) > len(body)+1 {
			t.Fatalf("Pipeline.HTML(, wording.ZhHant) produced %d TOC entries from %d input bytes", len(first.TOC), len(body))
		}
	})
}

func resultBytes(result *Result) int {
	size := len(result.HTML)
	for _, diagnostic := range result.Diagnostics {
		size += len(diagnostic.Kind) + len(diagnostic.Target) + len(diagnostic.Message)
	}
	for _, entry := range result.TOC {
		size += len(entry.Text) + len(entry.ID)
	}
	return size
}

// internalNoTitles is the in-package double: these fuzz targets are about
// parsing and rendering, never about a name that turns out to be a title.
type internalNoTitles struct{}

func (internalNoTitles) TitledBy(string) []string { return nil }
