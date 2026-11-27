package machinemarks

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark/sealedline"
)

// A year of use, simulated, to measure what the ledger grows to: a few hundred
// marks a month, a third of them edited once or more, the odd deletion, a new
// process each day, and twice a month a git operation that puts an older copy
// of this machine's file back (which is what makes the ledger keep a whole copy
// of the file). It is a measurement, not a check, so it runs only when asked:
//
//	YOMIHON_GROWTH=1 go test ./internal/mark/machinemarks -run Growth -v
func TestGrowthOverAYearOfUse(t *testing.T) {
	if os.Getenv("YOMIHON_GROWTH") == "" {
		t.Skip("a measurement; set YOMIHON_GROWTH=1 to run it")
	}
	rng := rand.New(rand.NewPCG(3, 5)) // #nosec G404 -- a test's own sequence
	a := newClone(t, "A")
	passage := strings.Repeat("a passage the reader marked, about this long. ", 4) // about 180 bytes
	words := strings.Repeat("what the reader wrote about it. ", 6)                 // about 190 bytes
	next := 1
	var snapshot []byte
	var slowest time.Duration
	var total time.Duration
	writes := 0
	timed := func(f func()) {
		start := time.Now()
		f()
		d := time.Since(start)
		total += d
		slowest = max(slowest, d)
		writes++
	}
	file := func() string { return filepath.Join(a.marksDir(), a.id()+".jsonl") }
	for range 12 {
		for day := range 30 {
			a.restart() // a new process each day
			for range 10 {
				n := next
				next++
				timed(func() {
					if rng.IntN(2) == 0 {
						a.put(highlight(n, passage), nil)
					} else {
						a.say(n, fmt.Sprintf("%d %s", n, words))
					}
				})
				if rng.IntN(3) == 0 {
					for range 1 + rng.IntN(3) {
						timed(func() { a.say(1+rng.IntN(n), fmt.Sprintf("edit %d %s", rng.IntN(1e6), words)) })
					}
				}
				if rng.IntN(40) == 0 {
					timed(func() { a.delete(1 + rng.IntN(n)) })
				}
			}
			if day == 9 {
				snapshot = readFile(t, file())
			}
			if day == 20 && snapshot != nil {
				// git puts an older copy back.
				if err := os.WriteFile(file(), snapshot, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if day == 28 && snapshot != nil {
				if err := os.WriteFile(file(), append(bytes.Clone(snapshot), []byte("<<<<<<< HEAD\n")...), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	ledger := a.ledgerBytes()
	info, err := os.Stat(file())
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	v := a.view()
	t.Logf("one read of the whole vault (View): %v", time.Since(start))
	start = time.Now()
	_ = v.Entries(open)
	t.Logf("Entries over %d marks: %v", len(v.Entries(open)), time.Since(start))
	t.Logf("marks: %d read, %d lines in the file, file %d bytes", len(v.Entries(open)), len(sealedline.Lines(readFile(t, file()))), info.Size())
	t.Logf("ledger: %d bytes, %d lines, %d whole copies of the file kept", len(ledger), len(sealedline.Lines(ledger)), bytes.Count(ledger, []byte(`"kind":"marks-replaced"`)))
	t.Logf("writes: %d, mean %v, slowest %v", writes, total/time.Duration(writes), slowest)
	if v.Missing() != 0 || v.DamagedOwn() != 0 {
		t.Fatalf("%d missing, %d damaged after the year", v.Missing(), v.DamagedOwn())
	}
}
