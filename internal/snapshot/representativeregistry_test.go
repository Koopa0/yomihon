package snapshot

import (
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRepresentativeBenchmarkEntryAndOptIn(t *testing.T) {
	t.Parallel()
	f := newRepresentativeFixture(t, 100)
	root, err := os.OpenRoot(f.root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	var sourceBytes int
	if walkErr := filepath.WalkDir(f.root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(f.root, path)
		if relErr != nil {
			return relErr
		}
		data, readErr := root.ReadFile(rel)
		if readErr != nil {
			return readErr
		}
		sourceBytes += len(data)
		return nil
	}); walkErr != nil {
		t.Fatal(walkErr)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), binary, append([]string{"-test.run=^$"}, args...)...) // #nosec G204 -- current compiled test binary and fixed benchmark arguments
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("caught: benchmark command failed: %v, output=%s", err, out)
		}
		return string(out)
	}
	inventory := run("-test.list=^BenchmarkRepresentativeSnapshot$")
	if !strings.Contains(inventory, "\nBenchmarkRepresentativeSnapshot\n") && !strings.HasPrefix(inventory, "BenchmarkRepresentativeSnapshot\n") {
		t.Fatal("caught: representative benchmark entry missing from compiled inventory")
	}
	const selector = "-test.bench=^BenchmarkRepresentativeSnapshot$/^notes=100$/^(initial|rebuild)$"
	disabled := run("-test.bench=^BenchmarkRepresentativeSnapshot$", "-test.benchtime=1x", "-test.benchmem", "-test.v")
	if !strings.Contains(disabled, "representative snapshots require -snapshot-bench") {
		t.Fatalf("caught: representative benchmark was not explicitly skipped: %s", disabled)
	}
	rows := regexp.MustCompile(`(?m)^(BenchmarkRepresentativeSnapshot/\S+)\s+1\s+[^\n]+$`)
	if strings.Contains(disabled, "invoked: representative fixture setup") {
		t.Fatal("caught: disabled representative benchmark constructed a fixture")
	}
	if rows.MatchString(disabled) {
		t.Fatal("caught: disabled representative benchmark produced measured rows")
	}
	leaves := regexp.MustCompile(`(?m)^[\t ]*--- SKIP: (BenchmarkRepresentativeSnapshot/notes=\d+/\w+)$`)
	var skipped []string
	for _, match := range leaves.FindAllStringSubmatch(disabled, -1) {
		skipped = append(skipped, match[1])
	}
	var wantSkipped []string
	for _, size := range []string{"100", "1000", "5000"} {
		for _, name := range []string{"initial", "rebuild", "idle", "overlap", "visible"} {
			wantSkipped = append(wantSkipped, "BenchmarkRepresentativeSnapshot/notes="+size+"/"+name)
		}
	}
	if diff := cmp.Diff(wantSkipped, skipped); diff != "" {
		t.Fatalf("caught: disabled benchmark leaf inventory (-want +got):\n%s", diff)
	}
	enabled := run(selector, "-test.benchtime=1x", "-test.benchmem", "-snapshot-bench")
	var got []string
	for _, match := range rows.FindAllStringSubmatch(enabled, -1) {
		name, _, _ := strings.Cut(match[1], "-")
		got = append(got, name)
		fields := strings.Fields(match[0])
		if len(fields)%2 != 0 {
			t.Fatalf("caught: malformed benchmark row: %q", match[0])
		}
		values := make(map[string]float64)
		var units []string
		for i := 2; i < len(fields); i += 2 {
			value, err := strconv.ParseFloat(fields[i], 64)
			if err != nil || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
				t.Fatalf("caught: invalid benchmark metric %q: %v", fields[i], err)
			}
			values[fields[i+1]] = value
			units = append(units, fields[i+1])
		}
		slices.Sort(units)
		if diff := cmp.Diff([]string{"B/op", "allocs/op", "files", "links", "notes", "ns/op", "source-B"}, units); diff != "" {
			t.Errorf("caught: benchmark metric set (-want +got):\n%s", diff)
		}
		wantCounts := map[string]float64{"files": 101, "links": 200, "notes": 100, "source-B": float64(sourceBytes)}
		gotCounts := map[string]float64{"files": values["files"], "links": values["links"], "notes": values["notes"], "source-B": values["source-B"]}
		if diff := cmp.Diff(wantCounts, gotCounts); diff != "" {
			t.Errorf("caught: named workload counts (-want +got):\n%s", diff)
		}
	}
	want := []string{"BenchmarkRepresentativeSnapshot/notes=100/initial", "BenchmarkRepresentativeSnapshot/notes=100/rebuild"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("caught: compiled benchmark rows (-want +got):\n%s", diff)
	}
}
