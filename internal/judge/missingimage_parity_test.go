package judge_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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
	body, err := os.ReadFile(filepath.Join(root, "Notes", "Images.md")) // #nosec G304 -- a fixed path under this test's own temp directory
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
	page := generation.Render("Notes/Images.md", string(body), wording.En)
	var markdown, wiki []string
	for _, d := range page.Diagnostics {
		switch d.Kind {
		case render.DiagImageMissing:
			markdown = append(markdown, d.Target)
		case render.DiagWikilinkBroken:
			if d.Target == "missing-wiki.png" || d.Target == "missing.pdf" {
				wiki = append(wiki, d.Target)
			}
		default:
			// Every other diagnostic is about something other than a file
			// the page could not show, so neither rule compares it.
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
	payload, _, err := judge.RunCheck(t.Context(), &judge.CheckOptions{Root: root, Format: judge.FormatJSON})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	var out bytes.Buffer
	if _, err := payload.WriteTo(&out); err != nil {
		t.Fatalf("write check payload: %v", err)
	}
	var checkMarkdown, checkWiki []string
	for line := range bytes.Lines(out.Bytes()) {
		var f struct {
			RuleID judge.RuleID `json:"rule_id"`
			Target *string      `json:"target"`
			Line   *int         `json:"line"`
		}
		if err := json.Unmarshal(line, &f); err != nil {
			t.Fatalf("check wrote a line that is not a finding: %q: %v", line, err)
		}
		if f.RuleID != judge.RuleLinkBrokenImage {
			continue
		}
		if f.Target == nil || f.Line == nil {
			t.Fatalf("link.broken.image finding without target or line: %+v", f)
		}
		if slices.Contains(wantWiki, *f.Target) {
			checkWiki = append(checkWiki, *f.Target)
		} else {
			checkMarkdown = append(checkMarkdown, *f.Target)
		}
	}
	if diff := cmp.Diff(wantMarkdown, checkMarkdown); diff != "" {
		t.Errorf("check and page disagree on Markdown images (-page +check):\n%s", diff)
	}
	if diff := cmp.Diff(wantWiki, checkWiki); diff != "" {
		t.Errorf("check and page disagree on wiki embeds (-page +check):\n%s", diff)
	}
}
