package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/ui/pages"
	"github.com/koopa0/yomihon/internal/wording"
)

const exampleServeNote = "Lessons/go/G02 channel 的交接.md"

type exampleServeReceipt struct {
	Root    string
	PID     int
	Parent  int
	Status  int
	Changed string
}

func TestExampleLaunchersKeepTheCheckoutUntouched(t *testing.T) {
	for _, entry := range []string{"make", "environment"} {
		t.Run(entry, func(t *testing.T) {
			h := newExampleServeHarness(t)
			code, output := h.run(entry)
			if code != 0 {
				t.Fatalf("caught: example launcher failed: exit=%d, output=%s", code, output)
			}
			var receipt exampleServeReceipt
			data, err := os.ReadFile(h.receipt)
			if err != nil {
				t.Fatal(err)
			}
			if decodeErr := json.Unmarshal(data, &receipt); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			if receipt.Status != http.StatusSeeOther || !strings.Contains(receipt.Changed, "\nstatus: archived\n") {
				t.Errorf("caught: copied example did not accept the real private status form: %+v", receipt)
			}
			if _, err := os.Stat(receipt.Root); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("caught: launcher left its copied vault: %q, stat error=%v", receipt.Root, err)
			}
			if diff := cmp.Diff(h.originals, exampleServeFiles(t, filepath.Join(h.dir, "examples", "vault"))); diff != "" {
				t.Errorf("caught: launcher changed checkout files (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExampleLaunchersStopBeforeServingAfterSetupFailure(t *testing.T) {
	for _, entry := range []string{"make", "environment"} {
		for _, failure := range []string{"copy", "build"} {
			t.Run(entry+"/"+failure, func(t *testing.T) {
				h := newExampleServeHarness(t)
				if failure == "copy" {
					if err := os.RemoveAll(filepath.Join(h.dir, "examples", "vault")); err != nil {
						t.Fatal(err)
					}
				} else {
					h.env = append(h.env, "YOMIHON_TEST_BUILD_FAILURE=1")
				}
				code, output := h.run(entry)
				marker := "controlled build failure"
				if failure == "copy" {
					marker = "cp:"
				}
				if !strings.Contains(output, marker) {
					t.Errorf("caught: setup failure did not reach %q: %s", marker, output)
				}
				if code == 0 {
					t.Errorf("caught: %s setup failure was ignored: %s", failure, output)
				}
				trace, traceErr := os.ReadFile(h.buildTrace)
				switch failure {
				case "copy":
					if !errors.Is(traceErr, os.ErrNotExist) {
						t.Errorf("caught: copy failure reached the build: trace=%q, error=%v", trace, traceErr)
					}
				case "build":
					if traceErr != nil {
						t.Fatal(traceErr)
					}
					if string(trace) != "build\n" {
						t.Errorf("caught: build failure was not invoked exactly once: %q", trace)
					}
				}
				if _, err := os.Stat(h.receipt); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("caught: failed setup reached serving: stat error=%v", err)
				}
				entries, err := os.ReadDir(h.temp)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 0 {
					t.Errorf("caught: failed launcher leaked temporary files: %v", entries)
				}
			})
		}
	}
}

func TestExampleLaunchersPropagateServerFailure(t *testing.T) {
	for _, entry := range []string{"make", "environment"} {
		t.Run(entry, func(t *testing.T) {
			h := newExampleServeHarness(t)
			h.env = append(h.env, "YOMIHON_TEST_EXAMPLE_SERVER_FAILURE=1")
			code, output := h.run(entry)
			if code == 0 || !strings.Contains(output, "controlled server failure") {
				t.Errorf("caught: child failure lost: exit=%d, output=%s", code, output)
			}
			entries, err := os.ReadDir(h.temp)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("caught: failed child left private resources: %v", entries)
			}
		})
	}
}

func TestExampleLauncherForwardsShutdownAndCleansTheCopy(t *testing.T) {
	for _, tt := range []struct {
		name   string
		signal syscall.Signal
	}{{"HUP", syscall.SIGHUP}, {"INT", syscall.SIGINT}, {"TERM", syscall.SIGTERM}} {
		t.Run(tt.name, func(t *testing.T) {
			h := newExampleServeHarness(t)
			h.env = append(h.env, "YOMIHON_TEST_EXAMPLE_HOLD=1")
			cmd := h.commandFor("make")
			pipe, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = cmd.Stdout
			if startErr := cmd.Start(); startErr != nil {
				t.Fatal(startErr)
			}
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
						t.Error(killErr)
					}
					if waitErr := cmd.Wait(); waitErr != nil {
						t.Logf("cleanup after failed assertion: %v", waitErr)
					}
				}
			})
			scanner := bufio.NewScanner(pipe)
			var lines []string
			ready := false
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
				if strings.Contains(scanner.Text(), "EXAMPLE-READY") {
					ready = true
					break
				}
			}
			if scanErr := scanner.Err(); scanErr != nil {
				t.Fatal(scanErr)
			}
			if !ready {
				t.Fatalf("caught: actual launcher child did not become ready: %s", strings.Join(lines, "\n"))
			}
			data, err := os.ReadFile(h.receipt)
			if err != nil {
				t.Fatal(err)
			}
			var receipt exampleServeReceipt
			if decodeErr := json.Unmarshal(data, &receipt); decodeErr != nil {
				t.Fatal(decodeErr)
			}
			shell, err := os.FindProcess(receipt.Parent)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("invoked: actual launcher shutdown")
			if signalErr := shell.Signal(tt.signal); signalErr != nil {
				t.Fatal(signalErr)
			}
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			if scanErr := scanner.Err(); scanErr != nil {
				t.Fatal(scanErr)
			}
			err = cmd.Wait()
			if err == nil || !strings.Contains(strings.Join(lines, "\n"), "EXAMPLE-SHUTDOWN") {
				t.Errorf("caught: shutdown did not reach the owned child: error=%v, output=%s", err, strings.Join(lines, "\n"))
			}
			if _, err := os.Stat(receipt.Root); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("caught: shutdown left the copied vault: root=%q, error=%v", receipt.Root, err)
			}
		})
	}
}

// TestExampleServeChild is the compiled child reached by the launcher's real
// command boundary. Its form drives the production composition without a
// listener, and only after the root has been proven to be a private copy.
func TestExampleServeChild(t *testing.T) {
	if os.Getenv("YOMIHON_TEST_EXAMPLE_CHILD") != "1" {
		t.Skip("launcher subprocess")
	}
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 || len(os.Args) <= separator+1 || os.Args[separator+1] != "serve" {
		t.Fatal("launcher did not ask to serve")
	}
	shutdown, stop := signal.NotifyContext(t.Context(), syscall.SIGHUP, os.Interrupt, syscall.SIGTERM)
	defer stop()
	root, err := serveRoot(os.Args[separator+2:])
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	workspace := os.Getenv("YOMIHON_TEST_EXAMPLE_WORKSPACE")
	if root == workspace || root == filepath.Join(workspace, "examples", "vault") || !strings.HasPrefix(root, filepath.Join(workspace, "temporary")+string(filepath.Separator)) {
		t.Fatalf("caught: launcher serves checkout instead of private copy: %q", root)
	}
	originals := exampleServeFiles(t, filepath.Join(workspace, "examples", "vault"))
	if diff := cmp.Diff(originals, exampleServeFiles(t, root)); diff != "" {
		t.Fatalf("caught: example copy differs before serving (-want +got):\n%s", diff)
	}
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	page := readingPageIn(t, site, pages.VaultHref("/notes/", exampleServeNote), wording.ZhHant)
	form := exampleServeArchiveForm(t, page)
	req := siteRequest(t, http.MethodPost, "/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	site.ServeHTTP(recorder, req)
	res := recorder.Result()
	if _, readErr := io.ReadAll(res.Body); readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr := res.Body.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("caught: copied example status response=%d, want303", res.StatusCode)
	}
	want := originals[exampleServeNote]
	if strings.Count(want, "\nstatus: ready\n") != 1 {
		t.Fatal("example status fixture has no unique ready line")
	}
	want = strings.Replace(want, "\nstatus: ready\n", "\nstatus: archived\n", 1)
	changed := exampleServeFiles(t, root)
	originals[exampleServeNote] = want
	if diff := cmp.Diff(originals, changed); diff != "" {
		t.Fatalf("caught: private transition changed other bytes (-want +got):\n%s", diff)
	}
	receipt, err := json.Marshal(exampleServeReceipt{Root: root, PID: os.Getpid(), Parent: os.Getppid(), Status: res.StatusCode, Changed: changed[exampleServeNote]})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("YOMIHON_TEST_EXAMPLE_RECEIPT"), receipt, 0o600); err != nil { // #nosec G703 -- receipt under the parent test's temporary directory
		t.Fatal(err)
	}
	if os.Getenv("YOMIHON_TEST_EXAMPLE_HOLD") == "1" {
		t.Log("EXAMPLE-READY")
		<-shutdown.Done()
		t.Log("EXAMPLE-SHUTDOWN")
	}
	if os.Getenv("YOMIHON_TEST_EXAMPLE_SERVER_FAILURE") == "1" {
		t.Error("controlled server failure")
	}
}

func exampleServeArchiveForm(t *testing.T, page string) url.Values {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	for node := range doc.Descendants() {
		if node.Type != html.ElementNode || node.Data != "form" {
			continue
		}
		fields := make(url.Values)
		for child := range node.Descendants() {
			if child.Type != html.ElementNode || child.Data != "input" {
				continue
			}
			var name, value string
			for _, attr := range child.Attr {
				if attr.Key == "name" {
					name = attr.Val
				}
				if attr.Key == "value" {
					value = attr.Val
				}
			}
			fields.Set(name, value)
		}
		if fields.Get("path") == exampleServeNote && fields.Get("from") == "ready" && fields.Get("to") == "archived" && fields.Get("content_identity") != "" {
			return fields
		}
	}
	t.Fatal("example page has no complete archive form")
	return nil
}

func exampleServeFiles(t *testing.T, directory string) map[string]string {
	t.Helper()
	source, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := source.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	files := make(map[string]string)
	if walkErr := fs.WalkDir(source.FS(), ".", func(rel string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := source.ReadFile(rel)
		if readErr != nil {
			return readErr
		}
		files[rel] = string(data)
		return nil
	}); walkErr != nil {
		t.Fatal(walkErr)
	}
	return files
}

type exampleServeHarness struct {
	t          *testing.T
	dir        string
	temp       string
	receipt    string
	buildTrace string
	env        []string
	command    string
	originals  map[string]string
}

func newExampleServeHarness(t *testing.T) *exampleServeHarness {
	t.Helper()
	h := &exampleServeHarness{t: t, dir: filepath.Join(t.TempDir(), "checkout with spaces")}
	if err := os.Mkdir(h.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	h.temp = filepath.Join(h.dir, "temporary")
	h.receipt = filepath.Join(h.dir, "receipt.json")
	h.buildTrace = filepath.Join(h.dir, "build-trace")
	for _, dir := range []string{h.temp, filepath.Join(h.dir, "bin"), filepath.Join(h.dir, "command bin")} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.CopyFS(filepath.Join(h.dir, "examples", "vault"), os.DirFS("../../examples/vault")); err != nil {
		t.Fatal(err)
	}
	h.originals = exampleServeFiles(t, filepath.Join(h.dir, "examples", "vault"))
	workspace, openErr := os.OpenRoot(h.dir)
	if openErr != nil {
		t.Fatal(openErr)
	}
	t.Cleanup(func() {
		if closeErr := workspace.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	makefile, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := workspace.WriteFile("Makefile", makefile, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	data, err := os.ReadFile("../../.cursor/environment.json")
	if err != nil {
		t.Fatal(err)
	}
	var environment struct {
		Terminals []struct{ Command string }
	}
	if decodeErr := json.Unmarshal(data, &environment); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if len(environment.Terminals) != 1 || environment.Terminals[0].Command == "" {
		t.Fatal("environment must have one example serving command")
	}
	h.command = environment.Terminals[0].Command
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := workspace.WriteFile("command bin/go", []byte(exampleServeGoBoundary), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if chmodErr := workspace.Chmod("command bin/go", 0o700); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	h.env = append(os.Environ(), "PATH="+filepath.Join(h.dir, "command bin")+string(os.PathListSeparator)+os.Getenv("PATH"), "TMPDIR="+h.temp, "YOMIHON_TEST_EXAMPLE_CHILD=1", "YOMIHON_TEST_EXAMPLE_WORKSPACE="+h.dir, "YOMIHON_TEST_EXAMPLE_RECEIPT="+h.receipt, "YOMIHON_TEST_EXAMPLE_BUILD_TRACE="+h.buildTrace, "YOMIHON_TEST_EXAMPLE_BINARY="+binary)
	return h
}

func (h *exampleServeHarness) commandFor(entry string) *exec.Cmd {
	h.t.Helper()
	command := "make run"
	if entry == "environment" {
		command = h.command
	}
	cmd := exec.CommandContext(h.t.Context(), "sh", "-c", command) // #nosec G204 -- actual tracked launcher command, isolated test-owned workspace and Go boundary
	cmd.Dir = h.dir
	cmd.Env = h.env
	return cmd
}

func (h *exampleServeHarness) run(entry string) (exit int, output string) {
	h.t.Helper()
	h.t.Log("invoked: actual example launcher")
	cmd := h.commandFor(entry)
	out, err := cmd.CombinedOutput()
	exitErr, exited := errors.AsType[*exec.ExitError](err)
	if err != nil && !exited {
		h.t.Fatal(err)
	}
	if exitErr != nil {
		return exitErr.ExitCode(), string(out)
	}
	return 0, string(out)
}

const exampleServeGoBoundary = `#!/bin/sh
set -eu
case "$1" in
  tool) [ "$2" = templ ] && [ "$3" = generate ]; exit 0 ;;
  build)
    printf 'build\n' >> "$YOMIHON_TEST_EXAMPLE_BUILD_TRACE"
    [ "${YOMIHON_TEST_BUILD_FAILURE-}" != 1 ] || { echo 'controlled build failure' >&2; exit 23; }
    [ "$2" = -o ] || exit 24
    cat > "$3" <<'CHILD'
#!/bin/sh
exec "$YOMIHON_TEST_EXAMPLE_BINARY" -test.run='^TestExampleServeChild$' -test.v -test.timeout=5s -- "$@"
CHILD
    chmod 700 "$3" ;;
  run) shift 2; exec "$YOMIHON_TEST_EXAMPLE_BINARY" -test.run='^TestExampleServeChild$' -test.v -test.timeout=5s -- "$@" ;;
  *) echo 'unexpected Go boundary' >&2; exit 25 ;;
esac
`
