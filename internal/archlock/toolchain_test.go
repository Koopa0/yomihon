package archlock

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestToolInstallsUseACompatibleLocalToolchain runs the real tools recipe
// across the local-version boundary. The command substitute records every
// install; it does not choose the expected toolchain or repair the recipe.
func TestToolInstallsUseACompatibleLocalToolchain(t *testing.T) {
	for _, tc := range []struct {
		name, local, required, initial, want string
	}{
		{name: "older auto", local: "go1.24.7", required: "1.27.0", initial: "auto", want: "go1.27.0"},
		{name: "older local", local: "go1.26.8", required: "1.27.0", initial: "local", want: "go1.27.0"},
		{name: "equal offline", local: "go1.27.0", required: "1.27.0", initial: "local", want: "local"},
		{name: "equal minor directive offline", local: "go1.27.0", required: "1.27", initial: "local", want: "local"},
		{name: "older prerelease", local: "go1.27rc1", required: "1.27.0", initial: "auto", want: "go1.27.0"},
		{name: "newer prerelease offline", local: "go1.28beta2", required: "1.27.0", initial: "local", want: "local"},
		{name: "older patch", local: "go1.27.0", required: "1.27.1", initial: "auto", want: "go1.27.1"},
		{name: "older major", local: "go1.27.1", required: "2.0.0", initial: "auto", want: "go2.0.0"},
		{name: "newer patch offline", local: "go1.27.1", required: "1.27.0", initial: "local", want: "local"},
		{name: "newer minor offline", local: "go1.28.0", required: "1.27.0", initial: "auto", want: "auto"},
		{name: "newer explicit", local: "go1.27.1", required: "1.27.0", initial: "go1.27.1", want: "go1.27.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newToolRecipeHarness(t)
			h.setRequiredGo(tc.required)
			out, err := h.run("tools", "TEST_LOCAL_GO="+tc.local, "GOTOOLCHAIN="+tc.initial, "TEST_ALLOWED_TOOLCHAIN="+tc.want)
			if err != nil {
				t.Fatalf("caught: make tools: %v\n%s", err, out)
			}
			want := make([]string, len(h.tools))
			for i, tool := range h.tools {
				want[i] = tc.want + "\t" + tool
			}
			if diff := cmp.Diff(want, h.installs()); diff != "" {
				t.Fatalf("caught: installed toolchain/module receipts differ (-want +got):\n%s", diff)
			}
		})
	}
}

// TestToolInstallationStopsOnFailure keeps a failed setup or install from
// allowing a later binary to be replaced. It observes the actual make exit
// and every effect committed before that exit, including a middle failure.
func TestToolInstallationStopsOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name        string
		index, want int
	}{
		{name: "local version query", index: -1, want: 0},
		{name: "first install", index: 0, want: 1},
		{name: "middle install", index: 3, want: 4},
		{name: "last install", index: 6, want: 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newToolRecipeHarness(t)
			failure := "version"
			if tc.index >= 0 {
				failure = h.tools[tc.index]
			}
			out, err := h.run("tools", "TEST_FAIL="+failure)
			if err == nil || !strings.Contains(out, "CONTROLLED_FAILURE") {
				t.Fatalf("caught: make tools did not propagate %q: error=%v, output=%s", failure, err, out)
			}
			if got := h.installs(); len(got) != tc.want {
				t.Fatalf("caught: installs before failure = %q, want exactly %d", got, tc.want)
			}
		})
	}
}

// TestToolRefusalsNameTheInstalledIdentity binds diagnostics to the actual
// executable and its build metadata. Installation directories contain spaces
// so splitting the first buildinfo line on its second field cannot pass.
func TestToolRefusalsNameTheInstalledIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, build, pin, fail string
		wantError              bool
	}{
		{name: "accepted equal", build: "go1.27.0", pin: "v0.50.0", fail: "", wantError: false},
		{name: "accepted newer", build: "go1.27.1", pin: "v0.50.0", fail: "", wantError: false},
		{name: "older prerelease build", build: "go1.27rc1", pin: "v0.50.0", fail: "", wantError: true},
		{name: "older build", build: "go1.26.8", pin: "v0.50.0", fail: "", wantError: true},
		{name: "wrong pin", build: "go1.27.0", pin: "v0.49.0", fail: "", wantError: true},
		{name: "unreadable buildinfo", build: "go1.27.0", pin: "v0.50.0", fail: "metadata", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newToolRecipeHarness(t)
			h.addExecutable("deadcode")
			out, err := h.run("tool-identity-check", "TEST_BUILD_GO="+tc.build, "TEST_PIN="+tc.pin, "TEST_FAIL="+tc.fail)
			if tc.wantError {
				if err == nil || strings.Contains(out, "TOOL_ACCEPTED") || !strings.Contains(out, filepath.Join(h.bin, "deadcode")) {
					t.Fatalf("caught: refusal lost installed path or permitted tool: error=%v, output=%s", err, out)
				}
				if tc.fail == "" && !strings.Contains(out, tc.build) {
					t.Fatalf("caught: refusal lost build Go %s: %s", tc.build, out)
				}
				return
			}
			if err != nil || !strings.Contains(out, "TOOL_ACCEPTED") {
				t.Fatalf("caught: compatible pinned tool refused: error=%v, output=%s", err, out)
			}
		})
	}
}

func TestMissingToolsNameTheirInstallDirectory(t *testing.T) {
	for _, tc := range []struct {
		name, gobin, gopath, want string
	}{
		{name: "explicit bin", gobin: "/private/tool bin", gopath: "/ignored", want: "/private/tool bin/deadcode"},
		{name: "default bin", gobin: "", gopath: "/private/go path", want: "/private/go path/bin/deadcode"},
		{name: "multiple workspaces", gobin: "", gopath: "/first workspace:/second", want: "/first workspace/bin/deadcode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newToolRecipeHarness(t)
			out, err := h.run("tool-identity-check", "TEST_GOBIN="+tc.gobin, "TEST_GOPATH="+tc.gopath)
			if err == nil || !strings.Contains(out, tc.want) || !strings.Contains(out, "PATH") || strings.Contains(out, "TOOL_ACCEPTED") {
				t.Fatalf("caught: missing tool did not name install destination %q and PATH: error=%v, output=%s", tc.want, err, out)
			}
		})
	}
}

func TestToolInstallDirectoryQueryFailureStopsTheGate(t *testing.T) {
	for _, failure := range []string{"bin-query", "path-query"} {
		t.Run(failure, func(t *testing.T) {
			h := newToolRecipeHarness(t)
			out, err := h.run("tool-identity-check", "TEST_FAIL="+failure)
			if err == nil || !strings.Contains(out, "CONTROLLED_FAILURE") || strings.Contains(out, "TOOL_ACCEPTED") {
				t.Fatalf("caught: installation directory query failure was hidden: error=%v, output=%s", err, out)
			}
		})
	}
}

func pinnedInstalls(t *testing.T, makefile []byte) []string {
	t.Helper()
	versions := make(map[string]string)
	for _, match := range regexp.MustCompile(`(?m)^([A-Z_]+_VERSION) := (\S+)$`).FindAllStringSubmatch(string(makefile), -1) {
		versions[match[1]] = match[2]
	}
	var paths, tools []string
	for _, match := range regexp.MustCompile(`go install (\S+)@(v?)\$\(([A-Z_]+_VERSION)\)`).FindAllStringSubmatch(string(makefile), -1) {
		pin, ok := versions[match[3]]
		if !ok {
			t.Fatalf("caught: install %s references missing pin %s", match[1], match[3])
		}
		paths = append(paths, match[1])
		tools = append(tools, match[1]+"@"+match[2]+pin)
	}
	want := []string{
		"github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
		"github.com/securego/gosec/v2/cmd/gosec",
		"honnef.co/go/tools/cmd/staticcheck",
		"github.com/rhysd/actionlint/cmd/actionlint",
		"golang.org/x/vuln/cmd/govulncheck",
		"golang.org/x/perf/cmd/benchstat",
		"golang.org/x/tools/cmd/deadcode",
	}
	if diff := cmp.Diff(want, paths); diff != "" {
		t.Fatalf("caught: complete pinned install set differs (-want +got):\n%s", diff)
	}
	return tools
}

type toolRecipeHarness struct {
	t     *testing.T
	dir   string
	bin   string
	log   string
	make  string
	tools []string
}

func newToolRecipeHarness(t *testing.T) *toolRecipeHarness {
	t.Helper()
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Fatal(err)
	}
	h := &toolRecipeHarness{t: t, dir: t.TempDir(), make: makePath}
	h.bin = filepath.Join(h.dir, "tool bin")
	h.log = filepath.Join(h.dir, "installs")
	if err = os.Mkdir(h.bin, 0o700); err != nil {
		t.Fatal(err)
	}
	makefile, err := os.ReadFile(filepath.Join(repoRoot, "Makefile")) // #nosec G304 -- fixed repository source
	if err != nil {
		t.Fatal(err)
	}
	h.tools = pinnedInstalls(t, makefile)
	makefile = append(makefile, []byte("\ntool-identity-check:\n\t@$(call require-go-tool,deadcode,golang.org/x/tools,$(DEADCODE_VERSION))\n\t@echo TOOL_ACCEPTED\n")...)
	for path, contents := range map[string][]byte{
		"Makefile": makefile,
		"go.mod":   []byte("module tools-test\n\ngo 1.27.0\n"),
		"installs": nil,
	} {
		if err := os.WriteFile(filepath.Join(h.dir, path), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	h.writeCommand("go", toolBoundaryCommand)
	return h
}

func (h *toolRecipeHarness) setRequiredGo(version string) {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(h.dir, "go.mod"), []byte("module tools-test\n\ngo "+version+"\n"), 0o600); err != nil {
		h.t.Fatal(err)
	}
}

func (h *toolRecipeHarness) addExecutable(name string) {
	h.t.Helper()
	h.writeCommand(name, "#!/bin/sh\nexit 0\n")
}

func (h *toolRecipeHarness) writeCommand(name, contents string) {
	h.t.Helper()
	path := filepath.Join(h.bin, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		h.t.Fatal(err)
	}
	if err := os.Chmod(path, 0o700); err != nil { // #nosec G302 -- private test command needs owner execute permission
		h.t.Fatal(err)
	}
}

func (h *toolRecipeHarness) run(target string, overrides ...string) (string, error) {
	h.t.Helper()
	cmd := exec.CommandContext(h.t.Context(), h.make, "--no-print-directory", target) // #nosec G204 -- resolved make executable and fixed targets, in a harness-owned directory
	cmd.Dir = h.dir
	cmd.Env = []string{
		"PATH=" + h.bin + ":/usr/bin:/bin",
		"TEST_INSTALL_LOG=" + h.log,
		"TEST_LOCAL_GO=go1.24.7",
		"TEST_BUILD_GO=go1.27.0",
		"TEST_PIN=v0.50.0",
		"GOTOOLCHAIN=auto",
	}
	for _, override := range overrides {
		key, _, _ := strings.Cut(override, "=")
		cmd.Env = slices.DeleteFunc(cmd.Env, func(item string) bool { return strings.HasPrefix(item, key+"=") })
		cmd.Env = append(cmd.Env, override)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (h *toolRecipeHarness) installs() []string {
	h.t.Helper()
	data, err := os.ReadFile(h.log) // #nosec G304 -- harness-owned temporary receipt file
	if err != nil {
		h.t.Fatal(err)
	}
	if len(data) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

const toolBoundaryCommand = `#!/bin/sh
set -eu
case "$1" in
  env)
    shift
    for key do
      case "$key" in
        GOVERSION)
          [ "${GOTOOLCHAIN-}" = local ] || { echo 'local version query used an auto toolchain' >&2; exit 24; }
          [ "${TEST_FAIL-}" != version ] || { echo CONTROLLED_FAILURE >&2; exit 23; }
          printf '%s\n' "$TEST_LOCAL_GO" ;;
        GOBIN) [ "${TEST_FAIL-}" != bin-query ] || { echo CONTROLLED_FAILURE >&2; exit 23; }; printf '%s\n' "${TEST_GOBIN-}" ;;
        GOPATH) [ "${TEST_FAIL-}" != path-query ] || { echo CONTROLLED_FAILURE >&2; exit 23; }; printf '%s\n' "${TEST_GOPATH-/default workspace}" ;;
        *) echo 'unexpected env query' >&2; exit 25 ;;
      esac
    done ;;
  install)
    printf '%s\t%s\n' "${GOTOOLCHAIN-}" "$2" >> "$TEST_INSTALL_LOG"
    [ "${TEST_FAIL-}" != "$2" ] || { echo CONTROLLED_FAILURE >&2; exit 23; }
    [ -z "${TEST_ALLOWED_TOOLCHAIN-}" ] || [ "${GOTOOLCHAIN-}" = "$TEST_ALLOWED_TOOLCHAIN" ] || { echo 'toolchain selection would download or use an older Go' >&2; exit 26; } ;;
  version)
    [ "$2" = -m ] || { echo 'unexpected version query' >&2; exit 27; }
    [ "${TEST_FAIL-}" != metadata ] || { echo CONTROLLED_FAILURE >&2; exit 23; }
    printf '%s: %s\n\tmod\tgolang.org/x/tools\t%s\n' "$3" "$TEST_BUILD_GO" "$TEST_PIN" ;;
  *) echo 'unexpected go command' >&2; exit 28 ;;
esac
`
