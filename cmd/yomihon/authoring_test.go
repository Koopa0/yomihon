package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestNoContractRefusalNamesStarterAndHealth(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, command := range []string{"check", "coverage", "exists"} {
		for _, format := range []string{"json", "human", "md"} {
			t.Run(command+" "+format, func(t *testing.T) {
				t.Parallel()
				args := []string{"--root", root, "--format", format}
				if command == "exists" {
					args = append(args, "Start")
				}
				var stdout, stderr bytes.Buffer
				exit := runCommand(t.Context(), command, args, &stdout, &stderr, false)
				if exit != 2 || stdout.Len() != 0 {
					t.Fatalf("runCommand(%q, %q) = exit %d, stdout %q; want 2 and empty stdout", command, args, exit, stdout.String())
				}
				line, _, _ := strings.Cut(stderr.String(), "\n")
				if line != "yomihon: this folder has no vault contract, so there is nothing to judge notes against" {
					t.Fatalf("refusal first line = %q, want unchanged no-contract refusal", line)
				}
				t.Log("invoked: actual no-contract command refusal")
				for _, phrase := range []string{
					"examples/vault/System/schemas/vault-schema.toml",
					"https://github.com/koopa0/yomihon/releases",
					"source archive",
					"/health",
					"link findings",
				} {
					if !strings.Contains(stderr.String(), phrase) {
						t.Errorf("caught: no-contract refusal omits %q; stderr = %q", phrase, stderr.String())
					}
				}
			})
		}
	}
}

func TestAuthorGuideBuildsTheCourseItPromises(t *testing.T) {
	t.Parallel()
	for _, guide := range []struct{ readme, path string }{
		{"README.md", "docs/authoring.md"},
		{"README.zh-TW.md", "docs/authoring.zh-TW.md"},
	} {
		t.Run(guide.path, func(t *testing.T) {
			t.Parallel()
			readme := authorFile(t, guide.readme)
			if !strings.Contains(readme, "]("+guide.path+")") {
				t.Fatalf("caught: %s does not link its author guide", guide.readme)
			}
			doc := authorFile(t, guide.path)
			blocks := regexp.MustCompile("(?m)^### `([^`]+)`\\n\\n```markdown\\n([\\s\\S]*?)\\n```$").FindAllStringSubmatch(doc, -1)
			var paths []string
			for _, block := range blocks {
				paths = append(paths, block[1])
			}
			slices.Sort(paths)
			if !slices.Equal(paths, []string{"Lessons/Continue.md", "Lessons/Extra.md", "Lessons/Start.md", "Notes/First course.md"}) {
				t.Fatalf("caught: complete author-file set = %q, want all four named examples", paths)
			}
			root := t.TempDir()
			folders := regexp.MustCompile("`([A-Za-z]+/)`").FindAllStringSubmatch(doc, -1)
			var directories []string
			for _, folder := range folders {
				directories = append(directories, folder[1])
				if err := os.MkdirAll(filepath.Join(root, folder[1]), 0o750); err != nil {
					t.Fatalf("MkdirAll(%q) error = %v", folder[1], err)
				}
			}
			if !slices.Equal(directories, []string{"Concepts/", "Inbox/", "Lessons/", "Maps/", "Notes/"}) {
				t.Fatalf("caught: complete author folder instructions = %q, want all five starter directories", directories)
			}
			for _, block := range blocks {
				writeFile(t, root, block[1], block[2]+"\n")
			}
			writeFile(t, root, schema.ContractRelPath, authorFile(t, "examples/vault/System/schemas/vault-schema.toml"))
			t.Log("invoked: complete pasted author files and unchanged starter")
			var stdout, stderr bytes.Buffer
			exit := runCommand(t.Context(), "check", []string{"--root", root, "--format", "json"}, &stdout, &stderr, false)
			if exit != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("caught: pasted author guide check = exit %d, stdout %q, stderr %q; want 0 and no findings", exit, stdout.String(), stderr.String())
			}
			site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
			if err != nil {
				t.Fatalf("newReadingSite() error = %v", err)
			}
			t.Cleanup(func() {
				if err := site.close(); err != nil {
					t.Errorf("readingSite.close() error = %v", err)
				}
			})
			server := httptest.NewServer(site)
			t.Cleanup(server.Close)
			pathsModel := site.snapshots.Current().Capture().Navigation().Paths()
			if len(pathsModel) != 1 || pathsModel[0].Planned != 2 || pathsModel[0].Branched != 1 || len(pathsModel[0].Diagnostics) != 0 {
				t.Fatalf("caught: pasted author guide does not yield exactly two main lessons and one side lesson: %+v", pathsModel)
			}
			for _, lang := range []wording.Lang{wording.En, wording.ZhHant} {
				index := authorPage(t, server, "/paths", lang)
				courseURL := onlyLink(t, "author course index", index, "data-index-row", "First course")
				course := authorPage(t, server, courseURL, lang)
				startURL := onlyLink(t, "author course", course, "y-lesson", "Start")
				start := authorPage(t, server, startURL, lang)
				if next := onlyLink(t, "author first lesson", start, `rel="next"`, "Continue"); next != "/notes/Lessons/Continue.md" {
					t.Errorf("caught: Start's next href = %q, want Continue on the main line", next)
				}
				_ = authorPage(t, server, "/notes/Lessons/Extra.md", lang)
			}
			t.Log("invoked: both complete author fences through actual judge and composed reading routes")
		})
	}
}

func authorPage(t *testing.T, server *httptest.Server, path string, lang wording.Lang) string {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+path, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequestWithContext(%q) error = %v", path, err)
	}
	request.Header.Set("Cookie", "yomihon_lang="+string(lang))
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("GET %q error = %v", path, err)
	}
	data, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("GET %q = status %d, read error %v, close error %v; want 200 without errors", path, response.StatusCode, readErr, closeErr)
	}
	if !strings.Contains(string(data), `<html lang="`+string(lang)+`"`) {
		t.Fatalf("caught: GET %q did not render the requested interface language %q", path, lang)
	}
	return string(data)
}

func authorFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", path)) // #nosec G304 -- callers name fixed tracked documentation and starter paths
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}
