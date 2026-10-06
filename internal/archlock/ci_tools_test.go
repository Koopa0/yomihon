package archlock

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	pins, parseErr := declaredToolPins(inputs.makefile)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	return inputs, pins
}

var toolPinNamePattern = regexp.MustCompile(`^([^[:space:]:=!?+]*_VERSION)(?:[[:space:]:=!?+]|$)`)

var canonicalToolPinNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*_VERSION$`)

// declaredToolPins refuses a declaration it cannot compare as a literal pin.
func declaredToolPins(makefile string) ([]toolPin, error) {
	var pins []toolPin
	seen := make(map[string]bool)
	definitions := 0
	for raw := range strings.SplitSeq(makefile, "\n") {
		if strings.HasPrefix(raw, "\t") {
			continue
		}
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if index := strings.IndexByte(line, '#'); index > 0 && (line[index-1] == ' ' || line[index-1] == '\t') {
			line = strings.TrimSpace(line[:index])
		}
		fields := strings.Fields(line)
		inside := definitions > 0
		if fields[0] == "define" {
			definitions++
		} else if fields[0] == "endef" {
			definitions = max(definitions-1, 0)
			continue
		}
		name, canonical := toolPinCandidate(line)
		if name == "" {
			continue
		}
		if seen[name] {
			return nil, fmt.Errorf("%s: duplicate tool pin declaration", name)
		}
		seen[name] = true
		if inside || !canonical || !canonicalToolPinNamePattern.MatchString(name) || len(fields) != 3 || fields[0] != name || fields[1] != ":=" || !literalToolPin(fields[2]) {
			return nil, fmt.Errorf("%s: unsupported tool pin declaration", name)
		}
		pins = append(pins, toolPin{name: name, version: fields[2]})
	}
	if len(pins) == 0 {
		return nil, errors.New("no declared tool pins")
	}
	slices.SortFunc(pins, func(a, b toolPin) int { return strings.Compare(a.name, b.name) })
	return pins, nil
}

// toolPinCandidate recognizes a declaration before checking its spelling.
func toolPinCandidate(line string) (string, bool) {
	canonical := true
	for {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return "", false
		}
		switch fields[0] {
		case "export", "override", "private", "define", "undefine":
			line = strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
			canonical = false
		default:
			match := toolPinNamePattern.FindStringSubmatch(line)
			if match == nil {
				if index := strings.IndexByte(line, ':'); index >= 0 {
					match = toolPinNamePattern.FindStringSubmatch(strings.TrimSpace(line[index+1:]))
					canonical = false
				}
			}
			if match == nil {
				return "", false
			}
			return match[1], canonical
		}
	}
}

// literalToolPin leaves normalization to the checker, without evaluating Make.
func literalToolPin(value string) bool {
	if value == "" {
		return false
	}
	if value[0] == '\'' || value[0] == '"' {
		if len(value) < 3 || value[len(value)-1] != value[0] {
			return false
		}
		value = value[1 : len(value)-1]
	}
	return value != "" && !strings.ContainsAny(value, "$\\'\"#")
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

type pinDeclarationCase struct {
	name, text, pin, version, wantErr string
}

func pinDeclarationCases() []pinDeclarationCase {
	return []pinDeclarationCase{
		{name: "canonical", text: "NEUTRAL_VERSION := v3.2.1\n", version: "v3.2.1"},
		{name: "spacing and comment", text: "  NEUTRAL_VERSION   :=   v3.2.1  # release\n", version: "v3.2.1"},
		{name: "single quote", text: "NEUTRAL_VERSION := 'v3.2.1'\n", version: "'v3.2.1'"},
		{name: "double quote", text: "NEUTRAL_VERSION := \"v3.2.1\"\n", version: "\"v3.2.1\""},
		{name: "equals", text: "NEUTRAL_VERSION = v3.2.1\n", wantErr: "unsupported"},
		{name: "conditional", text: "NEUTRAL_VERSION ?= v3.2.1\n", wantErr: "unsupported"},
		{name: "immediate", text: "NEUTRAL_VERSION ::= v3.2.1\n", wantErr: "unsupported"},
		{name: "lowercase equals", text: "neutral_VERSION = 9\n", pin: "neutral_VERSION", wantErr: "unsupported"},
		{name: "lowercase immediate", text: "neutral_VERSION := 9\n", pin: "neutral_VERSION", wantErr: "unsupported"},
		{name: "underscore equals", text: "_NEUTRAL_VERSION = 9\n", pin: "_NEUTRAL_VERSION", wantErr: "unsupported"},
		{name: "underscore immediate", text: "_NEUTRAL_VERSION := 9\n", pin: "_NEUTRAL_VERSION", wantErr: "unsupported"},
		{name: "numeric immediate", text: "9NEUTRAL_VERSION := 9\n", pin: "9NEUTRAL_VERSION", wantErr: "unsupported"},
		{name: "empty stem", text: "_VERSION := 9\n", pin: "_VERSION", wantErr: "unsupported"},
		{name: "hyphen equals", text: "odd-tool_VERSION = 9\n", pin: "odd-tool_VERSION", wantErr: "unsupported"},
		{name: "hyphen immediate", text: "odd-tool_VERSION := 9\n", pin: "odd-tool_VERSION", wantErr: "unsupported"},
		{name: "dot immediate", text: "odd.tool_VERSION := 9\n", pin: "odd.tool_VERSION", wantErr: "unsupported"},
		{name: "unicode immediate", text: "工具_VERSION := 9\n", pin: "工具_VERSION", wantErr: "unsupported"},
		{name: "no spaces", text: "NEUTRAL_VERSION:=v3.2.1\n", wantErr: "unsupported"},
		{name: "adjacent equals", text: "NEUTRAL_VERSION=9\n", wantErr: "unsupported"},
		{name: "adjacent value", text: "NEUTRAL_VERSION :=v3.2.1\n", wantErr: "unsupported"},
		{name: "append", text: "NEUTRAL_VERSION += v3.2.1\n", wantErr: "unsupported"},
		{name: "expanded immediate", text: "NEUTRAL_VERSION :::= v3.2.1\n", wantErr: "unsupported"},
		{name: "shell assignment", text: "NEUTRAL_VERSION != v3.2.1\n", wantErr: "unsupported"},
		{name: "export", text: "export NEUTRAL_VERSION := v3.2.1\n", wantErr: "unsupported"},
		{name: "override", text: "override NEUTRAL_VERSION := v3.2.1\n", wantErr: "unsupported"},
		{name: "private", text: "private NEUTRAL_VERSION := v3.2.1\n", wantErr: "unsupported"},
		{name: "bare export", text: "export NEUTRAL_VERSION\n", wantErr: "unsupported"},
		{name: "target specific", text: "target: NEUTRAL_VERSION := v3.2.1\n", wantErr: "unsupported"},
		{name: "adjacent target", text: "target:NEUTRAL_VERSION=9\n", wantErr: "unsupported"},
		{name: "pin definition", text: "define NEUTRAL_VERSION\nv3.2.1\nendef\n", wantErr: "unsupported"},
		{name: "pin inside definition", text: "define helper\nNEUTRAL_VERSION := v3.2.1\nendef\n", wantErr: "unsupported"},
		{name: "empty", text: "NEUTRAL_VERSION :=\n", wantErr: "unsupported"},
		{name: "unterminated quote", text: "NEUTRAL_VERSION := 'v3.2.1\n", wantErr: "unsupported"},
		{name: "extra word", text: "NEUTRAL_VERSION := v3.2.1 extra\n", wantErr: "unsupported"},
		{name: "dynamic", text: "NEUTRAL_VERSION := $(OTHER)\n", wantErr: "unsupported"},
		{name: "dynamic function", text: "NEUTRAL_VERSION := $(shell command)\n", wantErr: "unsupported"},
		{name: "continuation", text: "NEUTRAL_VERSION := v3.2.1\\\n", wantErr: "unsupported"},
		{name: "duplicate", text: "NEUTRAL_VERSION := v3.2.1\nNEUTRAL_VERSION := v3.2.1\n", wantErr: "duplicate"},
		{name: "comment", text: "# NEUTRAL_VERSION = v3.2.1\n"},
		{name: "recipe", text: "\tNEUTRAL_VERSION = v3.2.1\n"},
		{name: "reference", text: "target: $(NEUTRAL_VERSION)\n"},
		{name: "bare reference", text: "$(NEUTRAL_VERSION)\n"},
		{name: "similar variable", text: "NEUTRAL_VERSION_EXTRA = 9\n"},
	}
}

func TestCIToolPinsDeclarationSyntax(t *testing.T) {
	t.Parallel()
	for _, tc := range pinDeclarationCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inputs := toolInputs{
				makefile:  "BASE_VERSION := 1\n" + tc.text,
				workflow:  "  BASE_VERSION: 1\n  NEUTRAL_VERSION: 3.2.1\n  use: tool@${BASE_VERSION} tool@${NEUTRAL_VERSION}\n",
				bootstrap: "# no copies\n",
			}
			exit, out := runToolChecker(t, inputs)
			const prefix = "check-ci-tools: every declared tool pin agrees with CI and bootstrap:"
			if tc.wantErr != "" {
				pin := tc.pin
				if pin == "" {
					pin = "NEUTRAL_VERSION"
				}
				if exit != 1 || !strings.Contains(out, pin+": "+tc.wantErr+" tool pin declaration") || strings.Contains(out, prefix) {
					t.Errorf("caught: unsupported pin declaration escaped: %s exit=%d output=%s", tc.name, exit, out)
				}
				return
			}
			if exit != 0 || !strings.HasPrefix(out, prefix) {
				t.Fatalf("caught: canonical declaration rejected: %s exit=%d output=%s", tc.name, exit, out)
			}
			want := []string{"BASE_VERSION=1"}
			if tc.version != "" {
				want = []string{"BASE_VERSION=1", "NEUTRAL_VERSION=3.2.1"}
			}
			if diff := cmp.Diff(want, strings.Fields(strings.TrimPrefix(out, prefix))); diff != "" {
				t.Errorf("caught: whole literal declaration set (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCIToolPinFixtureGrammar(t *testing.T) {
	t.Parallel()
	for _, tc := range pinDeclarationCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			t.Log("invoked: declared Makefile pin fixture parser")
			pins, err := declaredToolPins("BASE_VERSION := 1\n" + tc.text)
			if tc.wantErr != "" {
				got := ""
				if err != nil {
					got = err.Error()
				}
				pin := tc.pin
				if pin == "" {
					pin = "NEUTRAL_VERSION"
				}
				want := pin + ": " + tc.wantErr + " tool pin declaration"
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("caught: fixture declaration omitted (-want +got):\n%s", diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("caught: canonical fixture rejected: %s error=%v", tc.name, err)
			}
			want := []toolPin{{name: "BASE_VERSION", version: "1"}}
			if tc.version != "" {
				want = []toolPin{{name: "BASE_VERSION", version: "1"}, {name: "NEUTRAL_VERSION", version: tc.version}}
			}
			if diff := cmp.Diff(want, pins, cmp.AllowUnexported(toolPin{})); diff != "" {
				t.Errorf("caught: complete fixture pins (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCIToolPinsUnsupportedCollectFailures(t *testing.T) {
	t.Parallel()
	inputs := toolInputs{
		makefile:  "BASE_VERSION := 1\nNEW_A_VERSION = 9\nNEW_B_VERSION ?= 9\n",
		workflow:  "  BASE_VERSION: 2\n  use: tool@${BASE_VERSION}\n",
		bootstrap: "# no copies\n",
	}
	exit, out := runToolChecker(t, inputs)
	if exit != 1 || strings.Contains(out, "every declared tool pin agrees") {
		t.Fatalf("caught: unsupported declaration aggregate status=%d output=%s", exit, out)
	}
	for _, marker := range []string{"NEW_A_VERSION: unsupported tool pin declaration", "NEW_B_VERSION: unsupported tool pin declaration", "pins BASE_VERSION at"} {
		if !strings.Contains(out, marker) {
			t.Errorf("caught: unsupported declaration aggregate omitted: %q output=%s", marker, out)
		}
	}
}
