package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koopa0/yomihon/internal/vault"
)

// BenchmarkBuildSnapshot measures one scan-and-rebuild of all three derived
// models over a small fixed vault written once to a temp directory. It reads
// the vault from disk each iteration, matching the work the scanner does on a
// change. The vault is stable so the number is meaningful across runs of a
// single machine.
func BenchmarkBuildSnapshot(b *testing.B) {
	root := b.TempDir()
	for rel, content := range benchVault {
		writeBenchNote(b, root, rel, content)
	}
	log := discardLogger()
	contract := testContract(b, root)
	reader, err := vault.Open(root)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { closeReader(b, reader) })
	b.ReportAllocs()
	for b.Loop() {
		scan, err := reader.ScanAvailable(b.Context())
		if err != nil {
			b.Fatal(err)
		}
		if _, _, err := buildView(
			b.Context(),
			reader,
			scan,
			log,
			contract.NavigationRoles(),
			contract.ArtifactPolicy(),
			contract.ArticleLanguage(),
		); err != nil {
			b.Fatal(err)
		}
	}
}

// benchVault is a small vault spanning the directories the three builders read:
// a study path for navigation, concepts for the graph and search, a map that
// links them, and a report.
var benchVault = map[string]string{
	"Maps/topics/Go MOC.md":        "---\ntitle: Go MOC\ntype: map\n---\n\nlinks to [[Goroutine]] and [[Channel]].\n",
	"Concepts/golang/Goroutine.md": "---\ntitle: Goroutine\ntype: concept\ndomain: golang\nstatus: evergreen\n---\n\nA goroutine is a lightweight thread. See [[Channel]].\n",
	"Concepts/golang/Channel.md":   "---\ntitle: Channel\ntype: concept\ndomain: golang\nstatus: growing\n---\n\nA channel carries values between goroutines.\n",
	"Concepts/japanese/Kana.md":    "---\ntitle: Kana\ntype: concept\ndomain: japanese\nstatus: seed\n---\n\nThe kana syllabary maps sounds to symbols.\n",
	"Writing/paths/Go Path.md":     "---\ntitle: Go Path\ntype: study-path\n---\n\n- [[Goroutine]]\n- [[Channel]]\n",
	"System/reports/coverage.md":   "---\ntitle: Coverage\ntype: report\n---\n\nA report note.\n",
}

// writeBenchNote writes root/rel (rel in slash form), creating parent
// directories. A write failure stops the benchmark.
func writeBenchNote(tb testing.TB, root, rel, content string) {
	tb.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		tb.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		tb.Fatalf("write %s: %v", rel, err)
	}
}
