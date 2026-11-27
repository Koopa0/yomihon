package snapshot

import (
	"flag"
	"fmt"
	"testing"
)

var representativeBench = flag.Bool("snapshot-bench", false, "include representative snapshot lifecycle benchmarks")

// BenchmarkBuildSnapshot measures one scan and build of a generation over a
// small fixed vault, read from disk each iteration as the scanner reads it on a
// change. The vault is stable so the number is comparable across runs on one
// machine.
func BenchmarkBuildSnapshot(b *testing.B) {
	root := b.TempDir()
	for rel, content := range benchVault {
		writeNote(b, root, rel, content)
	}
	contract := testContract(b, root)
	reader := openReader(b, root)
	store := newStore(reader, discardLogger(), contract, contract.Governance())
	b.ReportAllocs()
	for b.Loop() {
		scan, err := reader.ScanAvailable(b.Context())
		if err != nil {
			b.Fatal(err)
		}
		if _, _, err := store.buildGeneration(b.Context(), nil, scan, store.capabilities, nil); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRepresentativeSnapshot measures the lifecycle of synthetic vaults
// of several sizes: the first build, a rebuild after an edit, an idle rescan,
// the heap while two generations overlap, and how long an edit takes to reach
// a reader. It runs only with -snapshot-bench, and skips before building any
// fixture without it.
func BenchmarkRepresentativeSnapshot(b *testing.B) {
	for _, notes := range representativeSizes {
		b.Run(fmt.Sprintf("notes=%d", notes), func(b *testing.B) {
			for _, phase := range representativePhases {
				b.Run(phase.name, func(b *testing.B) {
					if !*representativeBench {
						b.Skip("representative snapshots require -snapshot-bench")
					}
					f := newRepresentativeFixture(b, notes)
					b.ReportAllocs()
					gen := phase.run(b, f)
					f.check(b, gen)
					f.report(b)
				})
			}
		})
	}
}

// benchVault spans the kinds of note a generation projects: a study path,
// concepts, a map that links them, and a report.
var benchVault = map[string]string{
	"Maps/topics/Go MOC.md":        "---\ntype: map\n---\n\nlinks to [[Goroutine]] and [[Channel]].\n",
	"Concepts/golang/Goroutine.md": "---\ntype: concept\n---\n\nA goroutine is a lightweight thread. See [[Channel]].\n",
	"Concepts/golang/Channel.md":   "---\ntype: concept\n---\n\nA channel carries values between goroutines.\n",
	"Concepts/japanese/Kana.md":    "---\ntype: concept\n---\n\nThe kana syllabary maps sounds to symbols.\n",
	"Writing/paths/Go Path.md":     "---\ntype: study-path\n---\n\n## Course {sequence=primary}\n\n- [[Goroutine]]\n- [[Channel]]\n",
	"System/reports/coverage.md":   "---\ntype: report\n---\n\nA report note.\n",
}
