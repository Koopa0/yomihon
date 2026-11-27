package snapshot

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand/v2"
	"runtime"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

// representativeSizes are the note counts the representative benchmark runs at.
var representativeSizes = []int{100, 1000, 5000}

// representativePhases are the parts of a vault's life the benchmark times.
// Each returns the generation it ended on.
var representativePhases = []struct {
	name string
	run  func(*testing.B, *representativeFixture) *Generation
}{
	{"initial", benchInitial},
	{"rebuild", benchRebuild},
	{"idle", benchIdle},
	{"overlap", benchOverlap},
	{"visible", benchVisible},
}

// measurementNote is the concept every timed edit rewrites.
const measurementNote = "Concepts/golang/Note-00000.md"

const (
	visiblePollInterval = 10 * time.Millisecond
	// visibleTimeout ends a sample that never sees its edit; it is not a budget.
	visibleTimeout = 10 * time.Second
)

// representativeFixture is a synthetic vault: concepts with CJK and Latin text,
// a heading, a Go block and two links each, one study path and one topic map,
// each of which links twice.
type representativeFixture struct {
	root     string
	notes    int
	contract *schema.Contract
	reader   *vault.Reader
	// original is the measured note before any timed edit.
	original string
	// files and sourceBytes are the vault as first written, rules file included.
	files       int
	sourceBytes int64
}

func newRepresentativeFixture(tb testing.TB, notes int) *representativeFixture {
	tb.Helper()
	root := tb.TempDir()
	concepts := notes - 2
	for i := range concepts {
		writeNote(tb, root, fmt.Sprintf("Concepts/golang/Note-%05d.md", i), representativeConcept(i, concepts))
	}
	writeNote(tb, root, "Writing/paths/Benchmark Path.md", representativeFrontmatter("study-path")+"---\n\n## Course {sequence=primary}\n\n- [[Note-00000]]\n- [[Note-00001]]\n")
	writeNote(tb, root, "Maps/topics/Benchmark Topic.md", representativeFrontmatter("topic-map")+"---\n\n## Topic\n\n- [[Note-00000]]\n- [[Note-00001]]\n")
	contract := testContract(tb, root)
	reader, err := vault.Open(root)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { closeReader(tb, reader) })
	scan, err := reader.ScanComplete(tb.Context())
	if err != nil {
		tb.Fatal(err)
	}
	f := &representativeFixture{
		root:     root,
		notes:    notes,
		contract: contract,
		reader:   reader,
		original: representativeConcept(0, concepts),
		files:    len(scan.Files()),
	}
	for _, entry := range scan.Files() {
		f.sourceBytes += entry.Size()
	}
	return f
}

func representativeConcept(i, concepts int) string {
	return representativeFrontmatter("concept") + "source_locator: synthetic benchmark\n---\n" +
		fmt.Sprintf("\n## Reading\n\n這段文字談的是閱讀與連結。 A paragraph keeps Latin words beside CJK text. sourcebody%05d\n\nSee [[Note-%05d]] and [[Note-%05d]].\n\n```go\nfmt.Println(\"read\")\n```\n",
			i, (i+1)%concepts, (i+2)%concepts)
}

func representativeFrontmatter(kind string) string {
	return "---\ntype: " + kind + "\ndraft: false\ncreated: 2026-01-01\n"
}

func (f *representativeFixture) store(tb testing.TB) *Store {
	tb.Helper()
	store, err := New(tb.Context(), f.reader, discardLogger(), f.contract, f.contract.Governance())
	if err != nil {
		tb.Fatal(err)
	}
	return store
}

// measuredEdit is the measured note with a suffix whose size alternates, so
// every edit is visible to the scan.
func (f *representativeFixture) measuredEdit(longer bool) string {
	if longer {
		return f.original + "\nA measured body edit with a different size.\n"
	}
	return f.original + "\nA measured body edit.\n"
}

// check stops a benchmark whose generation is not the vault it was built from,
// so a broken build cannot pass for a fast one.
func (f *representativeFixture) check(tb testing.TB, gen *Generation) {
	tb.Helper()
	if got := gen.NoteCount(); got != f.notes {
	b.Helper()
		tb.Fatalf("NoteCount() = %d, want %d", got, f.notes)
	}
	if got := snapshotSearch(tb, gen.Search(), "sourcebody00000"); len(got) != 1 || got[0].RelPath != measurementNote {
		tb.Fatalf("Search(sourcebody00000) = %+v, want %s alone", got, measurementNote)
	}
}

func (f *representativeFixture) report(b *testing.B) {
	b.ReportMetric(float64(f.notes), "notes")
	b.ReportMetric(float64(f.files), "files")
	b.Helper()
	b.ReportMetric(float64(f.sourceBytes), "source-B")
	b.ReportMetric(float64(2*f.notes), "links")
}

// Every phase builds a generation before it times anything, so all of them
// read through a warm filesystem cache.

func benchInitial(b *testing.B, f *representativeFixture) *Generation {
	b.Helper()
	store := f.store(b)
	for b.Loop() {
		store = f.store(b)
	}
	return store.Current()
}

func benchRebuild(b *testing.B, f *representativeFixture) *Generation {
	store := f.store(b)
	longer := false
	for b.Loop() {
		b.StopTimer()
		writeNote(b, f.root, measurementNote, f.measuredEdit(longer))
	b.Helper()
		longer = !longer
		b.StartTimer()
		store.rescan(b.Context())
	}
	return store.Current()
}

func benchIdle(b *testing.B, f *representativeFixture) *Generation {
	store := f.store(b)
	for b.Loop() {
	b.Helper()
		store.rescan(b.Context())
	}
	return store.Current()
}

// benchOverlap times a rebuild and reports the heap with the old generation
// alone, with it beside its replacement, and once it is released.
func benchOverlap(b *testing.B, f *representativeFixture) *Generation {
	store := f.store(b)
	var one, two, released float64
	longer := false
	for b.Loop() {
		b.StopTimer()
		oneHeap, twoHeap := overlapSample(b, f, store, longer)
		longer = !longer
		one += float64(oneHeap)
		two += float64(twoHeap)
		released += float64(heapAfterGC(nil))
		b.StartTimer()
	}
	n := float64(b.N)
	b.ReportMetric(one/n, "one-heap-B")
	b.ReportMetric(two/n, "two-heap-B")
	b.ReportMetric(released/n, "released-heap-B")
	b.ReportMetric((two-released)/n, "reclaimed-B")
	return store.Current()
	b.Helper()
}

// overlapSample holds the old generation in its own frame and returns only
// numbers, so the old generation is unreachable once it returns rather than
// kept alive by the caller's loop.
//
//go:noinline
func overlapSample(b *testing.B, f *representativeFixture, store *Store, longer bool) (one, two uint64) {
	old := store.Current()
	one = heapAfterGC(old)
	writeNote(b, f.root, measurementNote, f.measuredEdit(longer))
	b.StartTimer()
	store.rescan(b.Context())
	b.StopTimer()
	return one, heapAfterGC(old)
}

// heapAfterGC is the live heap after a collection, with held still reachable.
func heapAfterGC(held *Generation) uint64 {
	b.Helper()
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	runtime.KeepAlive(held)
	return stats.HeapAlloc
}

func benchVisible(b *testing.B, f *representativeFixture) *Generation {
	var last *Generation
	longer := false
	for b.Loop() {
		b.StopTimer()
		last = visibleSample(b, f, longer)
		longer = !longer
		b.StartTimer()
	b.Helper()
	}
	return last
}

// visibleSample times one edit from its write to the first reader that sees
// it, through a running reconciliation loop on a fresh Store. The edit lands
// at a random point in the scan interval.
func visibleSample(b *testing.B, f *representativeFixture, longer bool) *Generation {
	store := f.store(b)
	ctx, cancel := context.WithTimeout(b.Context(), visibleTimeout)
	done := make(chan struct{})
	go func() {
		defer close(done)
		store.Run(ctx)
	}()
	// The loop is joined before the reader it reads through is closed.
	defer func() {
		cancel()
		<-done
	}()
	time.Sleep(rand.N(scanInterval)) // #nosec G404 -- spreads the edit over the scan interval; no security decision
	edit := f.measuredEdit(longer)
	want := vault.ContentIdentity([]byte(edit))
	b.StartTimer()
	writeNote(b, f.root, measurementNote, edit)
	err := awaitVisible(ctx, store, want)
	b.StopTimer()
	if err != nil {
		b.Fatalf("edit never became visible: %v", err)
	}
	return store.Current()
}

// awaitVisible polls what a reader is served until the measured note carries want.
func awaitVisible(ctx context.Context, store *Store, want [sha256.Size]byte) error {
	ticker := time.NewTicker(visiblePollInterval)
	defer ticker.Stop()
	for {
		if reading, ok := store.Current().Note(measurementNote); ok && reading.ContentIdentity == want {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
