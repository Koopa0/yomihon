package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
)

func TestCommandExistsPathSuffix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for rel, body := range map[string]string{
		"Notes/Atlas/Page.md":     "---\ntitle: Page\n---\nbody\n",
		"Diary/Private/Hidden.md": "---\ntitle: Private\n---\nbody\n",
	} {
		suffixCommandWrite(t, root, rel, body)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data) + "\n[privacy]\nnever_egress_dirs = [\"Diary\"]\n"
	suffixCommandWrite(t, root, schema.ContractRelPath, text)
	tests := []struct {
		query, want string
		exit        int
	}{
		{"Atlas/Page", "{\"query\":\"Atlas/Page\",\"matches\":[{\"path\":\"Notes/Atlas/Page.md\",\"field\":\"path\",\"value\":\"Notes/Atlas/Page.md\"}]}\n", 0},
		{"Nope/Page", "{\"query\":\"Nope/Page\",\"matches\":[]}\n", 1},
		{"Private/Hidden", "{\"query\":\"Private/Hidden\",\"matches\":[],\"withheld\":true}\n", 0},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()
			var out, errs bytes.Buffer
			exit := runCommand(t.Context(), "exists", []string{"--root", root, "--format=json", tt.query}, &out, &errs, false)
			t.Log("invoked: actual suffix resolution contract")
			if exit != tt.exit || out.String() != tt.want || errs.Len() != 0 {
				t.Errorf("caught: composed suffix CLI changed: stdout=%q stderr=%q exit=%d", out.String(), errs.String(), exit)
			}
			if strings.Contains(out.String(), "Diary/") {
				t.Errorf("private location escaped: %q", out.String())
			}
		})
	}
}

func suffixCommandWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil { // #nosec G703 -- every caller supplies a testing.TempDir root and fixture-relative literal
		t.Fatal(err)
	}
}
