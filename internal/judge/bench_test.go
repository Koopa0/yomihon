package judge

import "testing"

// BenchmarkCheck measures a full diagnostics run — walk, parse, graph rules,
// disk-reference checks, and frontmatter schema validation — over a small fixed
// vault carrying a schema contract, so the whole engine is exercised. It reads
// the vault from disk each iteration, which is the shape a real run has. The
// input is stable so the number is meaningful across runs of a single machine.
func BenchmarkCheck(b *testing.B) {
	const root = "testdata/vault-report"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Check(root); err != nil {
			b.Fatalf("Check(%q): %v", root, err)
		}
	}
}
