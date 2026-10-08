package judge_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/judge"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestMissingImagePageParity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/vault-missing-images")); err != nil {
		t.Fatalf("copy page fixture: %v", err)
	}
	files, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("open fixture root: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := files.Close(); closeErr != nil {
			t.Errorf("close fixture root: %v", closeErr)
		}
	})
	before, err := files.ReadFile("Notes/Images.md")
	if err != nil {
		t.Fatalf("read page source: %v", err)
	}
	source, err := vault.Open(root)
	if err != nil {
		t.Fatalf("vault.Open: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := source.Close(); closeErr != nil {
			t.Errorf("close source: %v", closeErr)
		}
	})
	contract, err := schema.LoadReader(t.Context(), source)
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	store, err := snapshot.New(t.Context(), source, slog.New(slog.DiscardHandler), contract, contract.Governance())
	if err != nil {
		t.Fatalf("snapshot.New: %v", err)
	}
	generation := store.Current()
	if generation == nil {
		t.Fatal("snapshot has no generation")
	}
	page := generation.Render("Notes/Images.md", string(before), wording.En)
	var markdown, wiki []string
	for _, d := range page.Diagnostics {
		if d.Kind == render.DiagImageMissing {
			markdown = append(markdown, d.Target)
		} else if d.Kind == render.DiagWikilinkBroken {
			if d.Target == "missing-wiki.png" || d.Target == "missing.pdf" {
				wiki = append(wiki, d.Target)
			}
		}
	}
	wantMarkdown := []string{"Notes/missing.png", "Notes/missing.png", "Notes/nested.png", "missing-reference.png"}
	wantWiki := []string{"missing-wiki.png", "missing.pdf"}
	if diff := cmp.Diff(wantMarkdown, markdown); diff != "" {
		t.Fatalf("page Markdown producer differs from literal nonempty occurrence set (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantWiki, wiki); diff != "" {
		t.Fatalf("page wiki producer differs from literal nonempty occurrence set (-want +got):\n%s", diff)
	}
	findings, err := judge.Check(t.Context(), root)
	if err != nil {
		t.Fatalf("judge.Check: %v", err)
	}
	t.Log("invoked: missing-image page and check")
	var checkMarkdown, checkWiki []string
	for _, f := range findings {
		if f.RuleID != "link.broken.image" {
			continue
		}
		if f.Target == nil || f.Line == nil {
			t.Fatalf("caught: missing-image page parity: target or line absent: %+v", f)
		}
		if *f.Line == 9 {
			checkWiki = append(checkWiki, *f.Target)
		} else {
			checkMarkdown = append(checkMarkdown, *f.Target)
		}
	}
	if diff := cmp.Diff(wantMarkdown, checkMarkdown); diff != "" {
		t.Errorf("caught: missing-image page parity Markdown (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantWiki, checkWiki); diff != "" {
		t.Errorf("caught: missing-image page parity wiki (-want +got):\n%s", diff)
	}
	var healthTargets []string
	for _, link := range generation.Health().Unwritten {
		healthTargets = append(healthTargets, link.Target)
	}
	wantHealth := []string{"Go sync.Pool", "missing.pdf", "missing-wiki.png", "Ordinary note"}
	if diff := cmp.Diff(wantHealth, healthTargets); diff != "" {
		t.Errorf("caught: missing-image Health wiki-only preservation (-want +got):\n%s", diff)
	}
	after, err := files.ReadFile("Notes/Images.md")
	if err != nil {
		t.Fatalf("read source after projections: %v", err)
	}
	if diff := cmp.Diff(before, after); diff != "" {
		t.Errorf("caught: missing-image source preservation (-want +got):\n%s", diff)
	}
}
