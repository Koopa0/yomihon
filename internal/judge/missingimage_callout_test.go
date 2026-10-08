package judge_test

import (
	"bytes"
	"encoding/json"
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

func TestMissingImagesRespectLiteralCalloutTitles(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, body           string
		titleLine, imageLine int
		titleFingerprint     string
	}{
		{name: "known", body: "> [!note] ![](title.png)\n", titleLine: 1, titleFingerprint: "v1:d5c9757222181477"},
		{name: "unknown", body: "> [!unlisted] ![](title.png)\n", titleLine: 1, titleFingerprint: "v1:d5c9757222181477"},
		{name: "open", body: "> [!note]+ ![](title.png)\n", titleLine: 1, titleFingerprint: "v1:d5c9757222181477"},
		{name: "closed", body: "> [!note]- ![](title.png)\n", titleLine: 1, titleFingerprint: "v1:d5c9757222181477"},
		{name: "nested", body: "> [!note] Outer\n> > [!note] ![](title.png)\n"},
		{name: "comment-prefix", body: "%% hidden %%> [!note] ![](title.png)\n"},
		{name: "body-image", body: "> [!note] ![](title.png)\n> ![](body.png)\n", titleLine: 1, titleFingerprint: "v1:d5c9757222181477", imageLine: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/vault-missing-images")); err != nil {
				t.Fatalf("copy callout fixture: %v", err)
			}
			files, err := os.OpenRoot(root)
			if err != nil {
				t.Fatalf("open callout fixture: %v", err)
			}
			t.Cleanup(func() {
				if err := files.Close(); err != nil {
					t.Errorf("close callout fixture: %v", err)
				}
			})
			if err := files.WriteFile("Notes/Images.md", []byte(tt.body), 0o600); err != nil {
				t.Fatalf("write callout subject: %v", err)
			}
			source, err := vault.Open(root)
			if err != nil {
				t.Fatalf("open callout source: %v", err)
			}
			t.Cleanup(func() {
				if err := source.Close(); err != nil {
					t.Errorf("close callout source: %v", err)
				}
			})
			contract, err := schema.LoadReader(t.Context(), source)
			if err != nil {
				t.Fatalf("load callout authority: %v", err)
			}
			store, err := snapshot.New(t.Context(), source, slog.New(slog.DiscardHandler), contract, contract.Governance())
			if err != nil {
				t.Fatalf("build callout snapshot: %v", err)
			}
			var wantDiagnostics []render.Diagnostic
			if tt.name == "unknown" {
				wantDiagnostics = []render.Diagnostic{{Kind: render.DiagUnknownCallout, Target: "unlisted", Message: `unknown callout type "unlisted"; rendered as a neutral callout`}}
			}
			if tt.imageLine != 0 {
				wantDiagnostics = []render.Diagnostic{{Kind: render.DiagImageMissing, Target: "Notes/body.png", Message: `the vault holds no file at "Notes/body.png", so this picture cannot be shown`}}
			}
			for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
				page := store.Current().Render("Notes/Images.md", tt.body, lang)
				if diff := cmp.Diff(wantDiagnostics, page.Diagnostics); diff != "" {
					t.Errorf("caught: literal callout image page rail lang=%s (-want +got):\n%s", lang, diff)
				}
				if !bytes.Contains([]byte(page.HTML), []byte("![](title.png)")) {
					t.Error("caught: literal callout image title was lost")
				}
				if tt.imageLine != 0 && !bytes.Contains([]byte(page.HTML), []byte(`src="/raw/Notes/body.png"`)) {
					t.Error("caught: real callout body image was lost")
				}
				if bytes.Contains([]byte(page.HTML), []byte(`src="/raw/Notes/title.png"`)) {
					t.Error("caught: literal callout image title became an image")
				}
			}
			var want []judge.Finding
			if tt.titleLine != 0 {
				want = append(want, judge.Finding{RuleID: "callout.title_markup", Severity: judge.SeverityInfo, Path: "Notes/Images.md", Line: new(tt.titleLine), Message: "this callout title carries markup the page escapes as visible text", Evidence: "the title is written as ![](title.png) on a recognised callout's opening line", SuggestedAction: "move the markup into the callout body, or write the title as plain text", SourceRule: "yomihon", Target: new("![](title.png)"), Fingerprint: tt.titleFingerprint})
			}
			if tt.imageLine != 0 {
				want = append(want, judge.Finding{RuleID: "link.broken.image", Severity: judge.SeverityWarn, Path: "Notes/Images.md", Line: new(tt.imageLine), Message: "image to Notes/body.png resolves to no file", Evidence: "no file matches the image target", SuggestedAction: "restore the target file, change the image to an existing file, or remove it", SourceRule: "yomihon", Target: new("Notes/body.png"), Fingerprint: "v1:c593cf50b70ebb78"})
			}
			got, err := judge.Check(t.Context(), root)
			if err != nil {
				t.Fatalf("Check callout subject: %v", err)
			}
			t.Log("invoked: complete literal callout image Check and public deny")
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("caught: literal callout image complete findings (-want +got):\n%s", diff)
			}
			var wantWire bytes.Buffer
			for _, finding := range want {
				line, err := json.Marshal(finding)
				if err != nil {
					t.Fatalf("marshal literal finding: %v", err)
				}
				wantWire.Write(line)
				wantWire.WriteByte('\n')
			}
			payload, exit, err := judge.RunCheck(t.Context(), &judge.CheckOptions{Root: root, Format: judge.FormatJSON, Deny: []string{"link.broken.image"}})
			wantExit := 0
			if tt.imageLine != 0 {
				wantExit = 1
			}
			if err != nil || exit != wantExit || !bytes.Equal(wantWire.Bytes(), payload) {
				t.Errorf("caught: literal callout image public deny: error=%v exit=%d want=%d\nwant=%s\ngot=%s", err, exit, wantExit, wantWire.Bytes(), payload)
			}
		})
	}
}
