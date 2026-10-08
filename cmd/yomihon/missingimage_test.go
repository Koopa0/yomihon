package main

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/schema"
)

func missingImageCommandFixture(t *testing.T) (root string, want []byte) {
	t.Helper()
	root = t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../internal/judge/testdata/vault-missing-images")); err != nil {
		t.Fatalf("copy command fixture: %v", err)
	}
	files, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("open command fixture root: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := files.Close(); closeErr != nil {
			t.Errorf("close command fixture root: %v", closeErr)
		}
	})
	want, err = os.ReadFile("../../internal/judge/testdata/golden/missing-images.jsonl")
	if err != nil || len(want) == 0 {
		t.Fatalf("read command golden: error=%v bytes=%d", err, len(want))
	}
	before, err := files.ReadFile("Notes/Images.md")
	if err != nil {
		t.Fatalf("read command source: %v", err)
	}
	t.Cleanup(func() {
		after, readErr := files.ReadFile("Notes/Images.md")
		if readErr != nil {
			t.Errorf("read command source after invocation: %v", readErr)
			return
		}
		if !bytes.Equal(before, after) {
			t.Error("caught: missing-image command source changed")
		}
	})
	return root, want
}

func TestMissingImageCommand(t *testing.T) {
	t.Parallel()
	root, want := missingImageCommandFixture(t)
	baseline := filepath.Join(t.TempDir(), "baseline.jsonl")
	if err := os.WriteFile(baseline, want, 0o600); err != nil {
		t.Fatalf("write literal baseline: %v", err)
	}
	for _, tt := range []struct {
		name string
		args []string
		exit int
		out  []byte
	}{
		{name: "explicit json", args: []string{"--format=json"}, out: want},
		{name: "pipe chooses json", out: want},
		{name: "warn deny", args: []string{"--format=json", "--deny=warn"}, exit: 1, out: want},
		{name: "image rule deny", args: []string{"--format=json", "--deny=link.broken.image"}, exit: 1, out: want},
		{name: "baseline removes all rows and gate", args: []string{"--format=json", "--deny=link.broken.image", "--baseline=" + baseline}},
		{name: "scoped images", args: []string{"--format=json", "Notes/Images.md"}, out: want},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"--root=" + root}, tt.args...)
			var stdout, stderr bytes.Buffer
			exit := runCommand(t.Context(), "check", args, &stdout, &stderr, false)
			t.Log("invoked: missing-image command contract")
			if exit != tt.exit || stderr.Len() != 0 || !bytes.Equal(tt.out, stdout.Bytes()) {
				t.Errorf("caught: missing-image command contract: exit=%d want=%d stderr=%q\nwant=%s\ngot=%s", exit, tt.exit, stderr.String(), tt.out, stdout.Bytes())
			}
		})
	}
	for _, format := range []string{"human", "md"} {
		t.Run(format+" gate preserves payload", func(t *testing.T) {
			t.Parallel()
			var plain, gated, stderr bytes.Buffer
			args := []string{"--root=" + root, "--format=" + format}
			if exit := runCommand(t.Context(), "check", args, &plain, &stderr, false); exit != 0 || stderr.Len() != 0 {
				t.Fatalf("ungated command: exit=%d stderr=%q", exit, stderr.String())
			}
			if !strings.Contains(plain.String(), "link.broken.image") || !strings.Contains(plain.String(), "Notes/missing.png") {
				t.Fatalf("caught: missing-image command contract: missing human image record: %s", plain.String())
			}
			if exit := runCommand(t.Context(), "check", append(args, "--deny=link.broken.image"), &gated, &stderr, false); exit != 1 || stderr.Len() != 0 || !bytes.Equal(plain.Bytes(), gated.Bytes()) {
				t.Errorf("caught: missing-image command contract: gated exit=%d stderr=%q plain=%s gated=%s", exit, stderr.String(), plain.Bytes(), gated.Bytes())
			}
		})
	}
}

func TestMissingImageCommandCleanAndRefusals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../internal/judge/testdata/vault-missing-images")); err != nil {
		t.Fatalf("copy clean fixture: %v", err)
	}
	writeFile(t, root, "Notes/Images.md", "![](held.png) ![[held.png]]\n")
	for _, deny := range []string{"warn", "link.broken.image"} {
		var stdout, stderr bytes.Buffer
		exit := runCommand(t.Context(), "check", []string{"--root=" + root, "--format=json", "--deny=" + deny}, &stdout, &stderr, false)
		if exit != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
			t.Errorf("caught: missing-image command clean: deny=%s exit=%d stdout=%q stderr=%q", deny, exit, stdout.String(), stderr.String())
		}
	}
	invalidBaseline := filepath.Join(t.TempDir(), "invalid.jsonl")
	if err := os.WriteFile(invalidBaseline, []byte("not-json\n"), 0o600); err != nil {
		t.Fatalf("write invalid baseline: %v", err)
	}
	for _, args := range [][]string{{"--deny=link.broken.imag"}, {"--baseline=" + invalidBaseline}, {"NoSuchScope"}} {
		var stdout, stderr bytes.Buffer
		exit := runCommand(t.Context(), "check", append([]string{"--root=" + root, "--format=json"}, args...), &stdout, &stderr, false)
		if exit != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Errorf("caught: missing-image command refusal: args=%v exit=%d stdout=%q stderr=%q", args, exit, stdout.String(), stderr.String())
		}
	}
	writeFile(t, root, schema.ContractRelPath, "[malformed\n")
	for _, format := range []string{"json", "human", "md"} {
		var stdout, stderr bytes.Buffer
		exit := runCommand(t.Context(), "check", []string{"--root=" + root, "--format=" + format}, &stdout, &stderr, false)
		if exit != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "yomihon: privacy authority unavailable; agent-facing output disabled\n") {
			t.Errorf("caught: missing-image command authority: format=%s exit=%d stdout=%q stderr=%q", format, exit, stdout.String(), stderr.String())
		}
	}
}

func TestMissingImageCommandPrivacy(t *testing.T) {
	t.Parallel()
	absent, present := t.TempDir(), t.TempDir()
	for _, root := range []string{absent, present} {
		if err := os.CopyFS(root, os.DirFS("../../internal/judge/testdata/vault-missing-images")); err != nil {
			t.Fatalf("copy privacy fixture: %v", err)
		}
		writeFile(t, root, "Notes/Images.md", "![](public.png) ![[public-wiki.png]]\n![](/Private/secret.png) ![[Private/secret.png]]\n")
		writeFile(t, root, "Private/Author.md", "![](private-author.png) ![[private-author-wiki.png]]\n")
	}
	writeFile(t, present, "Private/secret.png", "held")
	var baselineBytes, baselineErrors bytes.Buffer
	if exit := runCommand(t.Context(), "check", []string{"--root=" + absent, "--format=json"}, &baselineBytes, &baselineErrors, false); exit != 0 || baselineErrors.Len() != 0 {
		t.Fatalf("privacy positive command: exit=%d stderr=%q", exit, baselineErrors.String())
	}
	for _, target := range []string{`"target":"Notes/public.png"`, `"target":"public-wiki.png"`} {
		if !bytes.Contains(baselineBytes.Bytes(), []byte(target)) {
			t.Fatalf("caught: missing-image command privacy positive producer: missing %s in %s", target, baselineBytes.String())
		}
	}
	baseline := filepath.Join(t.TempDir(), "baseline.jsonl")
	if err := os.WriteFile(baseline, baselineBytes.Bytes(), 0o600); err != nil {
		t.Fatalf("write privacy baseline: %v", err)
	}
	for _, format := range []string{"json", "human", "md"} {
		for _, scope := range [][]string{nil, {"--all"}, {"Notes"}} {
			for _, deny := range []string{"", "warn", "link.broken.image"} {
				for _, prior := range []string{"", baseline} {
					args := []string{"--format=" + format}
					args = append(args, scope...)
					if deny != "" {
						args = append(args, "--deny="+deny)
					}
					if prior != "" {
						args = append(args, "--baseline="+prior)
					}
					var a, b, aErrors, bErrors bytes.Buffer
					aExit := runCommand(t.Context(), "check", append([]string{"--root=" + absent}, args...), &a, &aErrors, false)
					bExit := runCommand(t.Context(), "check", append([]string{"--root=" + present}, args...), &b, &bErrors, false)
					t.Log("invoked: missing-image command denied destination")
					wantExit := 0
					if prior == "" && deny != "" {
						wantExit = 1
					}
					if aExit != wantExit || bExit != wantExit || aErrors.Len() != 0 || bErrors.Len() != 0 || !bytes.Equal(a.Bytes(), b.Bytes()) {
						t.Errorf("caught: missing-image command denied destination observable: format=%s scope=%v deny=%q baseline=%v exits=%d/%d want=%d stderr=%q/%q\nabsent=%s\npresent=%s", format, scope, deny, prior != "", aExit, bExit, wantExit, aErrors.String(), bErrors.String(), a.String(), b.String())
					}
					for _, forbidden := range []string{"Private", "secret.png", "private-author"} {
						if bytes.Contains(a.Bytes(), []byte(forbidden)) || bytes.Contains(b.Bytes(), []byte(forbidden)) {
							t.Errorf("caught: missing-image command denied destination observable: leaked %q", forbidden)
						}
					}
				}
			}
		}
	}
}

type missingImageFailingWriter struct {
	prefix []byte
}

func (w *missingImageFailingWriter) Write(p []byte) (int, error) {
	n := min(17, len(p))
	w.prefix = append(w.prefix, p[:n]...)
	return n, io.ErrClosedPipe
}

func TestMissingImageCommandOutputFailure(t *testing.T) {
	t.Parallel()
	root, want := missingImageCommandFixture(t)
	writer := &missingImageFailingWriter{}
	var stderr bytes.Buffer
	exit := runCommand(t.Context(), "check", []string{"--root=" + root, "--format=json"}, writer, &stderr, false)
	if exit != 2 || stderr.String() != "yomihon: write output: io: read/write on closed pipe\n" || !bytes.Equal(writer.prefix, want[:17]) {
		t.Errorf("caught: missing-image command output failure: exit=%d stderr=%q prefix=%q", exit, stderr.String(), writer.prefix)
	}
}

func TestMissingImageCommandPageAndHealth(t *testing.T) {
	t.Parallel()
	root, _ := missingImageCommandFixture(t)
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("close site: %v", closeErr)
		}
	})
	for _, target := range []string{"/notes/Notes/Images.md", "/health"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			site.ServeHTTP(recorder, siteRequest(t, http.MethodGet, target, nil))
			response := recorder.Result()
			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK {
				t.Fatalf("GET %s: status=%d read=%v close=%v", target, response.StatusCode, readErr, closeErr)
			}
			if target == "/health" {
				for _, wanted := range []string{"missing-wiki.png", "missing.pdf", "Go sync.Pool", "Ordinary note"} {
					if !bytes.Contains(body, []byte(wanted)) {
						t.Errorf("caught: missing-image Health wiki-only preservation: missing %q", wanted)
					}
				}
				for _, excluded := range []string{"Notes/missing.png", "nested.png", "missing-reference.png"} {
					if bytes.Contains(body, []byte(excluded)) {
						t.Errorf("caught: missing-image Health wiki-only preservation: added %q", excluded)
					}
				}
				return
			}
			for _, wanted := range []string{`src="/raw/Notes/missing.png"`, `src="/raw/Notes/held.png"`, "image-missing", "missing-wiki.png", "missing.pdf", "Go sync.Pool", "Ordinary note"} {
				if !bytes.Contains(body, []byte(wanted)) {
					t.Errorf("caught: missing-image command page preservation: missing %q", wanted)
				}
			}
		})
	}
}

func TestMissingImageBinary(t *testing.T) {
	t.Parallel()
	root, want := missingImageCommandFixture(t)
	binary := filepath.Join(t.TempDir(), "yomihon")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".") // #nosec G204 -- fixed tool and test-owned output
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build public binary: %v\n%s", err, output)
	}
	for _, tt := range []struct {
		name string
		args []string
		exit int
	}{
		{name: "pipe default"},
		{name: "explicit", args: []string{"--format=json"}},
		{name: "warn gate", args: []string{"--deny=warn"}, exit: 1},
		{name: "image gate", args: []string{"--deny=link.broken.image"}, exit: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"check", "--root=" + root}, tt.args...)
			command := exec.CommandContext(t.Context(), binary, args...) // #nosec G204 -- test-built binary and fixture arguments
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			exit := 0
			if err := command.Run(); err != nil {
				exitErr, ok := errors.AsType[*exec.ExitError](err)
				if !ok {
					t.Fatalf("run public binary: %v", err)
				}
				exit = exitErr.ExitCode()
			}
			t.Log("invoked: missing-image public binary")
			if exit != tt.exit || stderr.Len() != 0 {
				t.Errorf("caught: missing-image command contract: public exit=%d want=%d stderr=%q", exit, tt.exit, stderr.String())
			}
			if diff := cmp.Diff(string(want), stdout.String()); diff != "" {
				t.Errorf("caught: missing-image command contract public bytes (-want +got):\n%s", diff)
			}
		})
	}
}
