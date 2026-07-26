//go:build realvault

package render_test

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/vault"
)

// TestRealVaultRendersWithoutFaults checks that every markdown note in an
// explicitly selected private vault can be read and rendered without a panic
// or blank page. Paths and note contents never enter the test output.
func TestRealVaultRendersWithoutFaults(t *testing.T) {
	t.Parallel()
	testRealVaultRendersWithoutFaults(t)
}

func testRealVaultRendersWithoutFaults(t *testing.T) {
	t.Helper()
	defer redactRealVaultPanic(t)

	root := requireRealVaultRoot(t)
	reader, err := vault.Open(root)
	if err != nil {
		t.Fatal("open configured real vault failed")
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error("close configured real vault failed")
		}
	})

	scan, err := reader.ScanComplete(t.Context())
	if err != nil {
		t.Fatal("scan configured real vault failed")
	}

	entries := scan.Files()
	notes := make([]*vault.Note, 0, len(entries))
	resources := make([]string, 0, len(entries))
	bodies := make(transclusions)
	for _, entry := range entries {
		if path.Ext(entry.Path()) != ".md" {
			resources = append(resources, entry.Path())
			continue
		}
		data, err := reader.ReadFile(t.Context(), entry)
		if err != nil {
			t.Fatal("capture configured real-vault note failed")
		}
		note := vault.Parse(entry.Path(), data)
		notes = append(notes, note)
		bodies[entry.Path()] = note.Body
	}
	r := render.New(graph.New(notes, resources), bodies)

	mdCount := 0
	for _, entry := range entries {
		if path.Ext(entry.Path()) != ".md" {
			continue
		}
		ordinal := mdCount
		mdCount++
		t.Run(fmt.Sprintf("note-%d", ordinal), func(t *testing.T) {
			t.Parallel()

			switch renderRealVaultNote(bodies[entry.Path()], r) {
			case renderOK:
			case renderPanic:
				t.Error("real-vault note render panicked")
			case renderBlank:
				t.Error("real-vault note rendered a blank page")
			default:
				t.Error("real-vault note returned an unknown render result")
			}
		})
	}

	if mdCount == 0 {
		t.Error("real-vault sweep found zero markdown notes")
	}
	t.Logf("real-vault aggregate: markdown_notes=%d", mdCount)
}

func redactRealVaultPanic(t *testing.T) {
	t.Helper()

	if recover() != nil {
		t.Fatal("real-vault render verification panicked")
	}
}

type renderResult uint8

const (
	renderOK renderResult = iota
	renderPanic
	renderBlank
)

func renderRealVaultNote(body string, r *render.Pipeline) (result renderResult) {
	defer func() {
		if recover() != nil {
			result = renderPanic
		}
	}()

	if r.HTML("note.md", body).HTML == "" {
		return renderBlank
	}
	return renderOK
}

func requireRealVaultRoot(t *testing.T) string {
	t.Helper()

	root := os.Getenv("YOMIHON_ROOT")
	if root == "" {
		t.Skip("YOMIHON_ROOT is required for real-vault verification")
	}
	if _, err := os.Stat(root); err != nil { // #nosec G703 -- an explicit test-only opt-in selects the private root
		t.Fatal("configured real vault is unavailable")
	}
	return root
}
