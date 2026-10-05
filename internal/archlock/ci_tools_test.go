package archlock

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const ciToolsScript = "../../tools/check-ci-tools.sh"

type toolPin struct {
	name    string
	version string
}

type toolInputs struct {
	makefile  string
	workflow  string
	bootstrap string
}

func shippedToolInputs(t *testing.T) (toolInputs, []toolPin) {
	t.Helper()
	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	read := func(rel string) string {
		t.Helper()
		data, err := root.ReadFile(rel)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	inputs := toolInputs{makefile: read("Makefile"), workflow: read(".github/workflows/ci.yml"), bootstrap: read(".cursor/install.sh")}
	var pins []toolPin
	for line := range strings.SplitSeq(inputs.makefile, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasSuffix(fields[0], "_VERSION") && fields[1] == ":=" {
			if len(fields) < 3 {
				t.Fatalf("pin %q has no version", fields[0])
			}
			pins = append(pins, toolPin{name: fields[0], version: fields[2]})
		}
	}
	if len(pins) == 0 {
		t.Fatal("no declared tool pins")
	}
	slices.SortFunc(pins, func(a, b toolPin) int { return strings.Compare(a.name, b.name) })
	return inputs, pins
}

func runToolChecker(t *testing.T, inputs toolInputs) (exit int, output string) {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for _, file := range []struct{ name, text string }{
		{"Makefile", inputs.makefile}, {"ci.yml", inputs.workflow}, {"install.sh", inputs.bootstrap},
	} {
		name := filepath.Join(dir, file.name)
		if err := os.WriteFile(name, []byte(file.text), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, name)
	}
	return runToolCheckerPaths(t, paths)
}

func runToolCheckerPaths(t *testing.T, paths []string) (exit int, output string) {
	t.Helper()
	t.Log("invoked: actual CI tool pin checker")
	cmd := exec.CommandContext(t.Context(), "sh", append([]string{ciToolsScript}, paths...)...) // #nosec G204 -- fixed checker script and test-owned fixture paths
	out, err := cmd.CombinedOutput()
	exitErr, isExit := errors.AsType[*exec.ExitError](err)
	if err != nil && !isExit {
		t.Fatal(err)
	}
	if exitErr != nil {
		return exitErr.ExitCode(), string(out)
	}
	return 0, string(out)
}

func replaceToolInput(t *testing.T, input, old, replacement string) string {
	t.Helper()
	if strings.Count(input, old) != 1 {
		t.Fatalf("fixture needle %q does not have exactly one site", old)
	}
	return strings.Replace(input, old, replacement, 1)
}

func TestCIToolPinsCoverEveryDeclaration(t *testing.T) {
	t.Parallel()
	inputs, pins := shippedToolInputs(t)
	t.Run("complete success set", func(t *testing.T) {
		t.Parallel()
		exit, out := runToolChecker(t, inputs)
		if exit != 0 {
			t.Fatalf("caught: valid declared tool set rejected: exit=%d, output=%s", exit, out)
		}
		const prefix = "check-ci-tools: every declared tool pin agrees with CI and bootstrap:"
		if !strings.HasPrefix(out, prefix) {
			t.Fatalf("caught: incomplete success report: %q", out)
		}
		var want []string
		for _, pin := range pins {
			want = append(want, pin.name+"="+strings.TrimPrefix(pin.version, "v"))
		}
		if diff := cmp.Diff(want, strings.Fields(strings.TrimPrefix(out, prefix))); diff != "" {
			t.Errorf("caught: declared success set (-want +got):\n%s", diff)
		}
	})
	for _, pin := range pins {
		t.Run(pin.name, func(t *testing.T) {
			t.Parallel()
			broken := inputs
			broken.makefile = replaceToolInput(t, inputs.makefile, pin.name+" := "+pin.version, pin.name+" := v0.0.0")
			exit, out := runToolChecker(t, broken)
			if exit != 1 || !strings.Contains(out, "pins "+pin.name+" at") {
				t.Errorf("caught: declared pin drift escaped: %s exit=%d, output=%s", pin.name, exit, out)
			}
		})
	}
}

func TestCIToolPinsIncludeBootstrapAndFutureTools(t *testing.T) {
	t.Parallel()
	inputs, _ := shippedToolInputs(t)
	inputs.makefile += "\nNEUTRAL_PROBE_VERSION := v3.2.1\n"
	inputs.workflow += "\n  NEUTRAL_PROBE_VERSION: 3.2.1\n  install: tool@${NEUTRAL_PROBE_VERSION}\n"
	for _, tc := range []struct {
		name, copy string
		want       int
	}{
		{"no bootstrap copy", "", 0},
		{"bare equivalent", "NEUTRAL_PROBE_VERSION=3.2.1\n", 0},
		{"quoted exported equivalent", "export NEUTRAL_PROBE_VERSION='v3.2.1' # release\n", 0},
		{"double quoted equivalent", "NEUTRAL_PROBE_VERSION=\"3.2.1\"\n", 0},
		{"drift", "NEUTRAL_PROBE_VERSION=v3.2.2\n", 1},
		{"empty", "NEUTRAL_PROBE_VERSION=\n", 1},
		{"second copy drift", "NEUTRAL_PROBE_VERSION=3.2.1\nNEUTRAL_PROBE_VERSION=3.2.2\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := inputs
			fixture.bootstrap += "\n" + tc.copy
			exit, out := runToolChecker(t, fixture)
			if exit != tc.want || !strings.Contains(out, "NEUTRAL_PROBE_VERSION") {
				t.Errorf("caught: future tool/bootstrap mismatch: exit=%d,want=%d,output=%s", exit, tc.want, out)
			}
		})
	}
}

func TestCIToolPinsRejectMissingAndCollectFailures(t *testing.T) {
	t.Parallel()
	inputs, pins := shippedToolInputs(t)
	for _, tc := range []struct{ name, makefile, workflow string }{
		{"no declarations", "# no pins\n", inputs.workflow},
		{"missing CI pin", inputs.makefile, strings.ReplaceAll(inputs.workflow, "SHELLCHECK_VERSION:", "UNRELATED_VERSION:")},
		{"missing install use", inputs.makefile, strings.ReplaceAll(inputs.workflow, "${SHELLCHECK_VERSION}", "literal-shellcheck")},
		{"empty Make pin", replaceToolInput(t, inputs.makefile, pins[0].name+" := "+pins[0].version, pins[0].name+" :="), inputs.workflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			exit, out := runToolChecker(t, toolInputs{makefile: tc.makefile, workflow: tc.workflow, bootstrap: inputs.bootstrap})
			if exit != 1 || strings.Contains(out, "every declared tool pin agrees") {
				t.Errorf("caught: missing pin prerequisite escaped: exit=%d,output=%s", exit, out)
			}
		})
	}
	t.Run("all failures collected", func(t *testing.T) {
		t.Parallel()
		broken := inputs
		for _, pin := range pins {
			broken.makefile = replaceToolInput(t, broken.makefile, pin.name+" := "+pin.version, pin.name+" := v0.0.0")
		}
		exit, out := runToolChecker(t, broken)
		if exit != 1 {
			t.Errorf("caught: aggregate status=%d,want1,output=%s", exit, out)
		}
		for _, pin := range pins {
			if !strings.Contains(out, "pins "+pin.name+" at") {
				t.Errorf("caught: unreported drift: %s,output=%s", pin.name, out)
			}
		}
	})
}

func TestCIToolPinsRejectMissingInputs(t *testing.T) {
	t.Parallel()
	for missing := range 3 {
		t.Run([]string{"Makefile", "workflow", "bootstrap"}[missing], func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			var paths []string
			for index, name := range []string{"Makefile", "ci.yml", "install.sh"} {
				p := filepath.Join(dir, name)
				if index != missing {
					if err := os.WriteFile(p, []byte("# present\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				paths = append(paths, p)
			}
			exit, out := runToolCheckerPaths(t, paths)
			if exit != 1 || !strings.Contains(out, "cannot read "+paths[missing]) {
				t.Errorf("caught: unreadable input escaped: exit=%d,output=%s", exit, out)
			}
		})
	}
}

func FuzzCIToolPinEquivalence(f *testing.F) {
	for _, version := range []string{"0.11.0", "3.2.1", "0.0.0-20260709024250-82a0b07e230d"} {
		f.Add(version)
	}
	f.Fuzz(func(t *testing.T, version string) {
		if version == "" || len(version) > 80 || version[0] < '0' || version[0] > '9' {
			t.Skip()
		}
		for _, ch := range version {
			if !strings.ContainsRune("0123456789abcdefghijklmnopqrstuvwxyz.-", ch) {
				t.Skip()
			}
		}
		inputs := toolInputs{
			makefile:  "NEUTRAL_VERSION := " + version + "\n",
			workflow:  "  NEUTRAL_VERSION: v" + version + "\n  install: tool@${NEUTRAL_VERSION}\n",
			bootstrap: "export NEUTRAL_VERSION='v" + version + "'\n",
		}
		exit, out := runToolChecker(t, inputs)
		if exit != 0 || !strings.Contains(out, "NEUTRAL_VERSION="+version) {
			t.Fatalf("caught: equivalent pin rejected: exit=%d,output=%s", exit, out)
		}
		inputs.bootstrap = "NEUTRAL_VERSION=" + version + ".drift\n"
		exit, out = runToolChecker(t, inputs)
		if exit != 1 || !strings.Contains(out, "pins NEUTRAL_VERSION at") {
			t.Errorf("caught: unequal fuzz pin escaped: exit=%d,output=%s", exit, out)
		}
	})
}
