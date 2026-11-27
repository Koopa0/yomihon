package judge

import (
	"runtime"
	"strings"
	"testing"
)

// BenchmarkCheck measures a full diagnostics run — walk, parse, graph rules,
// disk-reference checks, and frontmatter schema validation — over a small fixed
// vault carrying a schema contract, so the whole engine is exercised. It reads
// the vault from disk each iteration, which is the shape a real run has. The
// input is stable so the number is meaningful across runs of a single machine.
func BenchmarkCheck(b *testing.B) {
	const root = "testdata/vault-report"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := runCheckAction(b.Context(), root, nil, false); err != nil {
			b.Fatalf("Check(%q): %v", root, err)
		}
	}
}

// BenchmarkParseNote measures reading one note, whose extractions share a
// single parse of the body.
func BenchmarkParseNote(b *testing.B) {
	data := []byte(strings.Repeat("## Visible heading\n\n[[Live]] [Manual](Manual.md) `quoted-path.md`\n\n> [!note] A title\n> A passage.\n\n%% [[Hidden]]\n## Hidden heading\n%%\n\n```text\n[[Code]]\n```\n\n", 32))
	marks := defaultPlannedMarks()
	b.ReportAllocs()
	for b.Loop() {
		runtime.KeepAlive(parseNoteWithMarks("Notes/source.md", data, marks))
	}
}
