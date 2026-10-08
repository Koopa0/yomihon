package snapshot

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/lexical"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

const representativePath = "Writing/paths/Benchmark Path.md"
const representativeMap = "Maps/topics/Benchmark Topic.md"

var representativeSizes = []int{100, 1000, 5000}

type representativeFixture struct {
	root     string
	sources  map[string]string
	notes    int
	contract *schema.Contract
	reader   *vault.Reader
}

type representativeMetrics struct {
	Notes       int
	Files       int
	SourceBytes int
	Links       int
}

func newRepresentativeFixture(tb testing.TB, notes int) *representativeFixture {
	tb.Helper()
	if notes < 4 {
		tb.Fatal("a representative fixture needs two targets and two navigation notes")
	}
	root := tb.TempDir()
	sources := make(map[string]string, notes+1)
	for i := range notes - 2 {
		name := fmt.Sprintf("Note-%05d", i)
		body := fmt.Sprintf("\n## Reading\n\n這段文字談的是閱讀與連結。 A paragraph keeps Latin words beside CJK text. sourcebody%05d\n\nSee [[Note-%05d]] and [[Note-%05d]].\n\n```go\nfmt.Println(\"read\")\n```\n", i, (i+1)%(notes-2), (i+2)%(notes-2))
		sources["Concepts/golang/"+name+".md"] = representativeFrontmatter(name, "concept") + "source_locator: synthetic benchmark\n---\n" + body
	}
	sources[representativePath] = representativeFrontmatter("Benchmark Path", "study-path") + "---\n\n## Course {sequence=primary}\n\n- [[Note-00000]]\n- [[Note-00001]]\n"
	sources[representativeMap] = representativeFrontmatter("Benchmark Topic", "topic-map") + "---\n\n## Topic\n\n- [[Note-00000]]\n- [[Note-00001]]\n"
	for rel, content := range sources {
		writeBenchNote(tb, root, rel, content)
	}
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
	entry, ok := scan.Entry(schema.ContractRelPath)
	if !ok {
		tb.Fatal("contract fixture missing from scan")
	}
	data, err := reader.ReadFile(tb.Context(), entry)
	if err != nil {
		tb.Fatal(err)
	}
	sources[schema.ContractRelPath] = string(data)
	return &representativeFixture{root: root, sources: sources, notes: notes, contract: contract, reader: reader}
}

func representativeFrontmatter(title, kind string) string {
	status := "archived"
	if kind == "concept" {
		status = "seedling"
	}
	return "---\ntitle: " + title + "\ntype: " + kind + "\ndomain: golang\nstatus: " + status + "\ncreated: 2026-01-01\nupdated: 2026-01-01\n"
}

func representativeGeneration(tb testing.TB, f *representativeFixture) *Store {
	tb.Helper()
	store, err := New(tb.Context(), f.reader, discardLogger(), f.contract, f.contract.Governance())
	if err != nil {
		tb.Fatal(err)
	}
	return store
}

func inspectRepresentative(ctx context.Context, f *representativeFixture, gen *Generation) (representativeMetrics, error) {
	scan, err := f.reader.ScanComplete(ctx)
	if err != nil {
		return representativeMetrics{}, err
	}
	observed := make(map[string]string, len(f.sources))
	for _, entry := range scan.Files() {
		data, readErr := f.reader.ReadFile(ctx, entry)
		if readErr != nil {
			return representativeMetrics{}, readErr
		}
		observed[entry.Path()] = string(data)
	}
	if diff := cmp.Diff(f.sources, observed); diff != "" {
		return representativeMetrics{}, fmt.Errorf("caught: source fixture changed (-want +got):\n%s", diff)
	}
	metrics := representativeMetrics{Notes: gen.NoteCount(), Files: len(gen.Files())}
	var paths, indexed []string
	for rel, source := range observed {
		metrics.SourceBytes += len(source)
		paths = append(paths, rel)
		if !vault.IsMarkdown(rel) {
			continue
		}
		frontmatter, opens := strings.CutPrefix(source, "---\n")
		_, body, closes := strings.Cut(frontmatter, "---\n")
		if !opens || !closes {
			return metrics, fmt.Errorf("caught: missing authored body at %s", rel)
		}
		reading, ok := gen.Note(rel)
		if !ok || reading.Body != body || reading.FMDiagnostic != "" || reading.Stale {
			return metrics, fmt.Errorf("caught: published reading differs from source at %s", rel)
		}
		for _, target := range judge.LinkTargets(body) {
			metrics.Links++
			answer := gen.Graph().Resolve(target)
			if answer.Kind != graph.KindUnique || answer.RelPath != "Concepts/golang/"+target+".md" {
				return metrics, fmt.Errorf("caught: target %q unresolved or wrong: %+v", target, answer)
			}
		}
		if strings.HasPrefix(rel, "Concepts/") {
			indexed = append(indexed, rel)
			for _, text := range []string{"## Reading", "閱讀與連結", "Latin words", "```go\nfmt.Println(\"read\")\n```"} {
				if !strings.Contains(body, text) {
					return metrics, fmt.Errorf("caught: content shape %q missing at %s", text, rel)
				}
			}
		}
	}
	slices.Sort(paths)
	var published []string
	for _, entry := range gen.Files() {
		published = append(published, entry.Path())
	}
	slices.Sort(published)
	if diff := cmp.Diff(paths, published); diff != "" {
		return metrics, fmt.Errorf("caught: published file set (-want +got):\n%s", diff)
	}
	if metrics.Notes != f.notes || metrics.Files != f.notes+1 || metrics.Links != 2*f.notes {
		return metrics, fmt.Errorf("caught: workload totals=%+v, want notes=%d files=%d links=%d", metrics, f.notes, f.notes+1, 2*f.notes)
	}
	health := gen.Health()
	if len(health.SchemaFaults) != 0 {
		rel := health.SchemaFaults[0].Note.RelPath
		return metrics, fmt.Errorf("caught: invalid generated note %s: %+v", rel, gen.schemaFindings[rel])
	}
	if len(health.SchemaFaults)+len(health.FrontmatterUnreadable)+len(health.NavigationFaults)+len(health.Unwritten)+len(health.Collisions) != 0 {
		return metrics, fmt.Errorf("caught: invalid generated workload: %+v", health)
	}
	if !gen.Freshness().Complete || len(gen.Freshness().Blocked) != 0 {
		return metrics, fmt.Errorf("caught: incomplete generation: %+v", gen.Freshness())
	}
	answer, err := gen.Search().Search(lexical.Parse("type:concept"), -1)
	if err != nil {
		return metrics, err
	}
	var hits []string
	for i := range answer.Results {
		hits = append(hits, answer.Results[i].RelPath)
	}
	slices.Sort(hits)
	slices.Sort(indexed)
	if diff := cmp.Diff(indexed, hits); diff != "" {
		return metrics, fmt.Errorf("caught: published search set (-want +got):\n%s", diff)
	}
	path := gen.Navigation().Path(representativePath)
	if path == nil || len(gen.Navigation().Paths()) != 1 || path.Planned != 2 || len(path.Groups) != 1 || len(path.Groups[0].Items) != 2 {
		return metrics, fmt.Errorf("caught: missing or invalid study path: %+v", path)
	}
	var stops []string
	for _, item := range path.Groups[0].Items {
		if item.Entry == nil || !item.Entry.Openable() {
			return metrics, fmt.Errorf("caught: study path row is not openable: %+v", item)
		}
		stops = append(stops, item.Entry.RelPath)
	}
	m := gen.Navigation().Map(representativeMap)
	if m == nil || len(gen.Navigation().Maps()) != 1 || len(m.Branches) != 1 {
		return metrics, fmt.Errorf("caught: missing or invalid topic map: %+v", m)
	}
	var mapped []string
	for _, entry := range m.Branches[0].Entries {
		mapped = append(mapped, entry.RelPath)
	}
	want := []string{"Concepts/golang/Note-00000.md", "Concepts/golang/Note-00001.md"}
	if diff := cmp.Diff([][]string{want, want}, [][]string{stops, mapped}); diff != "" {
		return metrics, fmt.Errorf("caught: published navigation targets (-want +got):\n%s", diff)
	}
	return metrics, nil
}

func representativeReceipt(tb testing.TB, f *representativeFixture, gen *Generation) representativeMetrics {
	tb.Helper()
	metrics, err := inspectRepresentative(tb.Context(), f, gen)
	if err != nil {
		tb.Fatal(err)
	}
	return metrics
}

func TestRepresentativeSnapshotWorkload(t *testing.T) {
	t.Parallel()
	f := newRepresentativeFixture(t, 8)
	store := representativeGeneration(t, f)
	receipt := representativeReceipt(t, f, store.Current())
	if diff := cmp.Diff([]int{8, 9, 16}, []int{receipt.Notes, receipt.Files, receipt.Links}); diff != "" {
		t.Errorf("caught: tiny workload totals (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]int{100, 1000, 5000}, representativeSizes); diff != "" {
		t.Errorf("caught: scenario set (-want +got):\n%s", diff)
	}
	previous := store.Current()
	const edited = "Concepts/golang/Note-00000.md"
	f.sources[edited] += "\nA small edit extends this source.\n"
	writeBenchNote(t, f.root, edited, f.sources[edited])
	store.rescan(t.Context())
	if store.Current() == previous {
		t.Fatal("caught: edit did not publish a replacement generation")
	}
	representativeReceipt(t, f, store.Current())
}

func TestRepresentativeSnapshotRejectsInvalidFixture(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"missing target", "extra source", "invalid schema", "missing primary sequence"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newRepresentativeFixture(t, 8)
			const target = "Concepts/golang/Note-00000.md"
			switch name {
			case "missing target":
				root, err := os.OpenRoot(f.root)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if closeErr := root.Close(); closeErr != nil {
						t.Error(closeErr)
					}
				})
				if err := root.Remove(target); err != nil {
					t.Fatal(err)
				}
			case "extra source":
				writeBenchNote(t, f.root, "Notes/Unexpected.md", "an unexpected source\n")
			case "invalid schema":
				f.sources[target] = strings.Replace(f.sources[target], "status: seedling", "status: invalid-fixture", 1)
				writeBenchNote(t, f.root, target, f.sources[target])
			case "missing primary sequence":
				f.sources[representativePath] = strings.Replace(f.sources[representativePath], " {sequence=primary}", "", 1)
				writeBenchNote(t, f.root, representativePath, f.sources[representativePath])
			}
			store := representativeGeneration(t, f)
			if _, err := inspectRepresentative(t.Context(), f, store.Current()); err == nil || !strings.HasPrefix(err.Error(), "caught:") {
				t.Fatalf("caught: invalid fixture was accepted: %v", err)
			}
		})
	}
}
