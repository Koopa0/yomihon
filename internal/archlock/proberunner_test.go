package archlock

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// The process double controls child verdicts; the copied production runner owns
// discovery, sequencing, continuation, marker validation and the final verdict.
const probeNode = `#!/bin/bash
set -euo pipefail
probe="${1##*/}"
mode="${MUTATE:-plain}"
printf '%s|%s|%s\n' "$probe" "${PAGE_PATH:-}" "$mode" >> "$PROBE_RUNNER_TRACE"
input="$PROBE_RUNNER_CASE/$probe/$mode"
if [ -f "$input.stdout" ]; then
  cat "$input.stdout"
elif [ "$mode" = list ]; then
  printf 'zeta\n\nalpha\n'
elif [ "$mode" != plain ]; then
  printf 'MUTATE-RESULT: caught %s\n' "$mode"
fi
if [ -f "$input.stderr" ]; then cat "$input.stderr" >&2; fi
if [ -f "$input.status" ]; then
  read -r status < "$input.status"
  exit "$status"
fi
if [ "$mode" = list ] || [ "$mode" = plain ]; then exit 0; fi
exit 1
`

type probeReply struct {
	probe  string
	mode   string
	status int
	stdout *string
	stderr string
}

type probeRunResult struct {
	Status   int
	Trace    []string
	Failures []string
	Success  []string
}

func TestProbeRunner(t *testing.T) {
	entries, source := probeRunnerSource(t)
	middle, _, _ := strings.Cut(entries[len(entries)/2], "|")
	later, _, _ := strings.Cut(entries[len(entries)/2+2], "|")
	final, _, _ := strings.Cut(entries[len(entries)/2+4], "|")
	diagnostics := strings.Repeat("diagnostic tail line\n", 1<<14)
	if len(diagnostics) <= 128<<10 {
		t.Fatal("large-output control must exceed 128 KiB")
	}
	cases := []struct {
		name         string
		mutate       bool
		callerMUTATE string
		replies      []probeReply
		modes        map[string][]string
		failures     []string
	}{
		{name: "plain success"},
		{name: "plain ignores caller list mode", callerMUTATE: "list"},
		{name: "plain ignores caller mutation mode", callerMUTATE: "alpha"},
		{
			name: "plain continues after failures",
			replies: []probeReply{
				{probe: middle, mode: "plain", status: 1, stderr: "first child diagnostic\n"},
				{probe: later, mode: "plain", status: 2, stderr: "later child diagnostic\n"},
				{probe: final, mode: "plain", status: 7},
			},
			failures: []string{middle + " plain exited 1, want 0", later + " plain exited 2, want 0", final + " plain exited 7, want 0"},
		},
		{name: "mutation success", mutate: true},
		{
			name: "mutation exact marker before large diagnostics", mutate: true,
			modes: map[string][]string{middle: {"early", "after"}},
			replies: []probeReply{
				{probe: middle, mode: "list", stdout: new("early\nafter\n")},
				{probe: middle, mode: "early", status: 1, stdout: new("MUTATE-RESULT: caught early\n" + diagnostics)},
			},
		},
		{
			name: "mutation exact marker after large diagnostics", mutate: true,
			modes: map[string][]string{middle: {"late", "after"}},
			replies: []probeReply{
				{probe: middle, mode: "list", stdout: new("late\nafter\n")},
				{probe: middle, mode: "late", status: 1, stdout: new(diagnostics + "MUTATE-RESULT: caught late\n")},
			},
		},
		{
			name: "mutation continues after status failures", mutate: true,
			modes: map[string][]string{middle: {"zero", "two", "seven", "after"}},
			replies: []probeReply{
				{probe: middle, mode: "list", stdout: new("zero\ntwo\nseven\nafter\n")},
				{probe: middle, mode: "zero", status: 0},
				{probe: middle, mode: "two", status: 2},
				{probe: middle, mode: "seven", status: 7},
			},
			failures: []string{
				middle + " MUTATE=zero: exited 0, want 1",
				middle + " MUTATE=two: exited 2, want 1",
				middle + " MUTATE=seven: exited 7, want 1",
			},
		},
		{
			name: "mutation requires exact stdout marker", mutate: true,
			modes: map[string][]string{middle: {"absent", "wrong", "prefix", "leading", "trailing", "stderr", "both", "after"}},
			replies: []probeReply{
				{probe: middle, mode: "list", stdout: new("absent\nwrong\nprefix\nleading\ntrailing\nstderr\nboth\nafter\n")},
				{probe: middle, mode: "absent", status: 1, stdout: new("")},
				{probe: middle, mode: "wrong", status: 1, stdout: new("MUTATE-RESULT: caught other\n")},
				{probe: middle, mode: "prefix", status: 1, stdout: new("MUTATE-RESULT: caught prefix-longer\n")},
				{probe: middle, mode: "leading", status: 1, stdout: new("garbage MUTATE-RESULT: caught leading\n")},
				{probe: middle, mode: "trailing", status: 1, stdout: new("MUTATE-RESULT: caught trailing garbage\n")},
				{probe: middle, mode: "stderr", status: 1, stdout: new(""), stderr: "MUTATE-RESULT: caught stderr\n"},
				{probe: middle, mode: "both", status: 2, stdout: new("")},
				{probe: middle, mode: "after", status: 1, stdout: new("before\nMUTATE-RESULT: caught after\nafter\n")},
			},
			failures: []string{
				middle + " MUTATE=absent: missing exact stdout line 'MUTATE-RESULT: caught absent'",
				middle + " MUTATE=wrong: missing exact stdout line 'MUTATE-RESULT: caught wrong'",
				middle + " MUTATE=prefix: missing exact stdout line 'MUTATE-RESULT: caught prefix'",
				middle + " MUTATE=leading: missing exact stdout line 'MUTATE-RESULT: caught leading'",
				middle + " MUTATE=trailing: missing exact stdout line 'MUTATE-RESULT: caught trailing'",
				middle + " MUTATE=stderr: missing exact stdout line 'MUTATE-RESULT: caught stderr'",
				middle + " MUTATE=both: exited 2, want 1; missing exact stdout line 'MUTATE-RESULT: caught both'",
			},
		},
		{
			name: "discovery failures continue", mutate: true,
			modes: map[string][]string{middle: {}, later: {}},
			replies: []probeReply{
				{probe: middle, mode: "list", status: 9, stdout: new("unusable\n"), stderr: "list child diagnostic\n"},
				{probe: later, mode: "list", stdout: new("")},
			},
			failures: []string{
				middle + " MUTATE=list exited 9, cannot discover mutation modes",
				later + " MUTATE=list names no runnable mutation modes",
			},
		},
		{
			name: "blank discovery continues", mutate: true,
			modes:    map[string][]string{middle: {}},
			replies:  []probeReply{{probe: middle, mode: "list", stdout: new("\n\n")}},
			failures: []string{middle + " MUTATE=list names no runnable mutation modes"},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newProbeRunnerFixture(t, entries, source)
			fixture.callerMUTATE = tt.callerMUTATE
			for _, reply := range tt.replies {
				fixture.reply(t, reply)
			}
			var args []string
			want := probeRunResult{Trace: expectedProbeTrace(entries, tt.mutate, tt.modes), Failures: tt.failures}
			if tt.mutate {
				args = []string{"--mutate"}
			}
			switch {
			case len(tt.failures) != 0:
				want.Status = 1
			case tt.mutate:
				want.Success = []string{"probes.sh: every mutation was caught"}
			default:
				want.Success = []string{"probes.sh: every lock passed"}
			}
			got, output := fixture.run(t, args, true)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("runner verdict and complete execution trace mismatch (-want +got):\n%s\nrunner output:\n%s", diff, output)
			}
			if len(tt.failures) != 0 {
				header := "FAIL probes.sh: " + strconv.Itoa(len(tt.failures)) + " failure(s):\n"
				if !strings.Contains(output, header) {
					t.Errorf("runner final summary missing %q\n%s", header, output)
				}
			}
			for _, reply := range tt.replies {
				if reply.mode != "list" && reply.stdout != nil && !strings.Contains(output, strings.TrimRight(*reply.stdout, "\n")) {
					t.Errorf("runner lost child stdout %q", *reply.stdout)
				}
				if reply.stderr != "" && !strings.Contains(output, reply.stderr) {
					t.Errorf("runner lost child stderr %q", reply.stderr)
				}
			}
		})
	}
}

func TestProbeRunnerPreflight(t *testing.T) {
	entries, source := probeRunnerSource(t)
	firstProbe, _, _ := strings.Cut(entries[0], "|")
	cases := []struct {
		name        string
		args        []string
		withoutBase bool
		remove      bool
		extra       bool
		status      int
		message     string
	}{
		{name: "missing base", withoutBase: true, status: 1, message: "probes.sh needs a running server"},
		{name: "unknown command", args: []string{"--unknown"}, status: 2, message: "usage: probes.sh [--mutate]"},
		{name: "missing probe", remove: true, status: 1, message: "the table names probes that are not here"},
		{name: "unlisted probe", extra: true, status: 1, message: "these probe files are driven by nothing"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newProbeRunnerFixture(t, entries, source)
			if tt.remove {
				if err := os.Remove(filepath.Join(fixture.dir, firstProbe)); err != nil {
					t.Fatal(err)
				}
			}
			if tt.extra {
				writeProbeFixture(t, filepath.Join(fixture.dir, "unlisted.mjs"), nil, 0o600)
			}
			got, output := fixture.run(t, tt.args, !tt.withoutBase)
			want := probeRunResult{Status: tt.status}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("runner preflight mismatch (-want +got):\n%s\n%s", diff, output)
			}
			if !strings.Contains(output, tt.message) {
				t.Errorf("runner output = %q, want diagnostic %q", output, tt.message)
			}
		})
	}
}

type probeRunnerFixture struct {
	dir          string
	bin          string
	caseDir      string
	trace        string
	bash         string
	callerMUTATE string
}

func probeRunnerSource(t *testing.T) (entries []string, data []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, ".github", "e2e", "probes.sh")) // #nosec G304 -- fixed repository source
	if err != nil {
		t.Fatal(err)
	}
	_, body, found := strings.Cut(string(data), "\nprobes=(\n")
	if !found {
		t.Fatal("production runner has no readable probe registry")
	}
	body, _, found = strings.Cut(body, "\n)\n")
	if !found {
		t.Fatal("production probe registry has no end")
	}
	entryPattern := regexp.MustCompile(`^\s*"([^"|]+\.mjs\|[^"|]+)"\s*$`)
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		match := entryPattern.FindStringSubmatch(line)
		if match == nil {
			t.Fatalf("unreadable registry entry %q", line)
		}
		probe, _, _ := strings.Cut(match[1], "|")
		if seen[probe] || filepath.Base(probe) != probe {
			t.Fatalf("duplicate or nonlocal registry probe %q", probe)
		}
		seen[probe] = true
		entries = append(entries, match[1])
	}
	if len(entries) < 10 {
		t.Fatal("probe registry cannot supply distinct middle and later test cases")
	}
	return entries, data
}

func newProbeRunnerFixture(t *testing.T, entries []string, source []byte) *probeRunnerFixture {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("required Bash unavailable: %v", err)
	}
	if _, statErr := os.Stat("/bin/bash"); statErr == nil {
		bash = "/bin/bash"
	}
	version, err := exec.CommandContext(t.Context(), bash, "--version").Output() // #nosec G204 -- installed Bash, fixed version argument
	if err != nil {
		t.Fatalf("required Bash cannot run: %v", err)
	}
	versionLine, _, _ := strings.Cut(string(version), "\n")
	t.Log(versionLine)
	dir := t.TempDir()
	fixture := &probeRunnerFixture{dir: dir, bin: filepath.Join(dir, "bin"), caseDir: filepath.Join(dir, "cases"), trace: filepath.Join(dir, "trace"), bash: bash}
	for _, path := range []string{fixture.bin, fixture.caseDir} {
		if mkdirErr := os.Mkdir(path, 0o700); mkdirErr != nil {
			t.Fatal(mkdirErr)
		}
	}
	writeProbeFixture(t, filepath.Join(dir, "probes.sh"), source, 0o600)
	copied, err := os.ReadFile(filepath.Join(dir, "probes.sh")) // #nosec G304 -- test-owned temporary copy
	if err != nil || !bytes.Equal(source, copied) {
		t.Fatalf("runner copy is not byte-identical: %v", err)
	}
	for _, entry := range entries {
		probe, _, _ := strings.Cut(entry, "|")
		writeProbeFixture(t, filepath.Join(dir, probe), nil, 0o600)
	}
	writeProbeFixture(t, filepath.Join(fixture.bin, "node"), []byte(probeNode), 0o700)
	writeProbeFixture(t, fixture.trace, nil, 0o600)
	return fixture
}

func writeProbeFixture(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil { // #nosec G306 -- executable fixture needs owner execute permission
		t.Fatal(err)
	}
}

func (f *probeRunnerFixture) reply(t *testing.T, reply probeReply) {
	t.Helper()
	dir := filepath.Join(f.caseDir, reply.probe)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(dir, reply.mode)
	writeProbeFixture(t, prefix+".status", []byte(strconv.Itoa(reply.status)+"\n"), 0o600)
	if reply.stdout != nil {
		writeProbeFixture(t, prefix+".stdout", []byte(*reply.stdout), 0o600)
	}
	if reply.stderr != "" {
		writeProbeFixture(t, prefix+".stderr", []byte(reply.stderr), 0o600)
	}
}

func (f *probeRunnerFixture) run(t *testing.T, args []string, withBase bool) (result probeRunResult, outputText string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.bash, append([]string{filepath.Join(f.dir, "probes.sh")}, args...)...) // #nosec G204 -- installed Bash running the exact test-owned production copy
	for _, variable := range os.Environ() {
		name, _, _ := strings.Cut(variable, "=")
		if name == "MUTATE" || name == "PAGE_PATH" || name == "YOMIHON_BASE" || name == "PATH" || strings.HasPrefix(name, "PROBE_RUNNER_") {
			continue
		}
		cmd.Env = append(cmd.Env, variable)
	}
	cmd.Env = append(cmd.Env, "PATH="+f.bin+string(os.PathListSeparator)+os.Getenv("PATH"), "PROBE_RUNNER_TRACE="+f.trace, "PROBE_RUNNER_CASE="+f.caseDir)
	if withBase {
		cmd.Env = append(cmd.Env, "YOMIHON_BASE=http://127.0.0.1:1")
	}
	if f.callerMUTATE != "" {
		cmd.Env = append(cmd.Env, "MUTATE="+f.callerMUTATE)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			result.Status = exitErr.ExitCode()
		} else {
			t.Fatalf("runner cannot execute: %v", err)
		}
	}
	if ctx.Err() != nil {
		t.Fatalf("runner exceeded bounded subprocess timeout: %v", ctx.Err())
	}
	trace, err := os.ReadFile(f.trace) // #nosec G304 -- test-owned temporary trace
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(trace), "\n") {
		if line != "" {
			result.Trace = append(result.Trace, line)
		}
	}
	for line := range strings.SplitSeq(string(output), "\n") {
		switch {
		case strings.HasPrefix(line, "  "):
			result.Failures = append(result.Failures, strings.TrimPrefix(line, "  "))
		case strings.HasPrefix(line, "probes.sh: every "):
			result.Success = append(result.Success, line)
		}
	}
	return result, string(output)
}

func expectedProbeTrace(entries []string, mutate bool, overrides map[string][]string) []string {
	var trace []string
	for _, entry := range entries {
		probe, page, _ := strings.Cut(entry, "|")
		if !mutate {
			trace = append(trace, probe+"|"+page+"|plain")
			continue
		}
		trace = append(trace, probe+"||list")
		modes, found := overrides[probe]
		if !found {
			modes = []string{"zeta", "alpha"}
		}
		for _, mode := range modes {
			trace = append(trace, probe+"|"+page+"|"+mode)
		}
	}
	return trace
}
