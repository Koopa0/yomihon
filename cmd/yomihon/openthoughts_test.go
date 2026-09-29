package main

import (
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

const openThoughtContract = `schema_version = "1"
[enums]
type = ["reflection", "other"]
[enums.status]
note = ["started", "revisit", "settled"]
[fields]
required = ["title", "type", "status"]
known = ["title", "type", "status", "updated", "based_on"]
[scan]
knowledge_dirs = ["Inside"]
[navigation]
path_types = []
map_types = []
answer_type = "reflection"
[artifacts]
non_instance_dirs = []
[privacy]
never_egress_dirs = []
[[lifecycle]]
status = "started"
applies_to = ["*"]
initial = true
from = []
owner = []
[[lifecycle]]
status = "revisit"
applies_to = ["*"]
initial = true
from = ["started"]
owner = []
[[lifecycle]]
status = "settled"
applies_to = ["*"]
initial = false
from = ["started", "revisit"]
owner = []
`

func TestOpenThoughtsComposesBothSourcesAndReachesOlderRows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, openThoughtContract)
	putOpenThoughtFile(t, root, "Source.md", "# Source\n\n## Chapter\n\nwords\n")
	for i := 1; i <= 6; i++ {
		putOpenThoughtFile(t, root, fmt.Sprintf("Inside/Thought%d.md", i), openThoughtNote(fmt.Sprintf("Thought %d", i), "reflection", "started", fmt.Sprintf("2026-09-%02d", i)))
	}
	putOpenThoughtFile(t, root, "Outside/Alternate.md", openThoughtNote("Alternative initial", "reflection", "revisit", "2026-09-08"))
	putOpenThoughtFile(t, root, "Inside/Done.md", openThoughtNote("Done note", "reflection", "settled", "2026-09-10"))
	putOpenThoughtFile(t, root, "Inside/WrongRole.md", openThoughtNote("Wrong role", "other", "started", "2026-09-11"))
	putOpenThoughtFile(t, root, "Inside/Invalid.md", openThoughtNote("Invalid status", "reflection", "unknown", "2026-09-12"))
	site, marks := openThoughtSite(t, root)
	_, err := marks.ToggleUncertainty(&mark.Uncertainty{RelPath: "Source.md", Anchor: "chapter", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"en", "zh-Hant"} {
		t.Run(lang, func(t *testing.T) {
			home := openThoughtPage(t, site, "/", lang)
			block := openThoughtBlock(t, home)
			if got := strings.Count(block, "data-desk-item"); got != 5 {
				t.Errorf("open desk rows = %d, want exactly five", got)
			}
			if !strings.Contains(block, `href="/open-thoughts"`) || !strings.Contains(block, `class="ui-navitem y-shelfall"`) {
				t.Error("open desk does not expose the existing All shelf link")
			}
			all := openThoughtPage(t, site, "/open-thoughts", lang)
			if got := strings.Count(all, "data-index-row"); got != 8 {
				t.Errorf("complete open shelf rows = %d, want eight", got)
			}
			for _, want := range []string{"Alternative initial", "Thought 1", "[[Source#Chapter]]", `href="/notes/Source.md#chapter"`} {
				if !strings.Contains(all, want) {
					t.Errorf("complete open shelf is missing %q", want)
				}
			}
			for _, unwanted := range []string{"Done note", "Wrong role", "Invalid status"} {
				if strings.Contains(all, unwanted) {
					t.Errorf("open shelf incorrectly includes %q", unwanted)
				}
			}
			if strings.Index(all, "Alternative initial") > strings.Index(all, `href="/notes/Source.md#chapter"`) || strings.Index(all, `href="/notes/Source.md#chapter"`) > strings.Index(all, "Thought 6") {
				t.Error("marks and notes are not interleaved newest first")
			}
			if lang == "en" && !strings.Contains(block, "What you left open") || lang == "zh-Hant" && !strings.Contains(block, "還沒說清楚的") {
				t.Error("open shelf title is not translated")
			}
		})
	}
}

func TestOpenThoughtsRechecksStatusBeforeTheNextScan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, openThoughtContract)
	rel := "Inside/Thought.md"
	putOpenThoughtFile(t, root, rel, openThoughtNote("A continuing thought", "reflection", "started", "2026-09-01"))
	site, _ := openThoughtSite(t, root)
	if !strings.Contains(openThoughtPage(t, site, "/open-thoughts", "en"), "A continuing thought") {
		t.Fatal("initial thought is absent")
	}
	for _, change := range []struct{ from, to string }{{"started", "revisit"}, {"revisit", "settled"}} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if err := site.writer.Flip(t.Context(), rel, change.from, change.to, vault.ContentIdentity(data)); err != nil {
			t.Fatal(err)
		}
		present := strings.Contains(openThoughtPage(t, site, "/open-thoughts", "en"), "A continuing thought")
		if present != (change.to == "revisit") {
			t.Errorf("open shelf after %s has thought = %t; initial-to-initial stays and settled leaves before scanning", change.to, present)
		}
	}
}

func TestOpenThoughtsKeepsMarksWithoutGovernanceAndReportsCorruption(t *testing.T) {
	t.Parallel()
	for _, contractText := range []string{"", "this is invalid TOML"} {
		t.Run(contractText, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if contractText != "" {
				putOpenThoughtFile(t, root, schema.ContractRelPath, contractText)
			}
			putOpenThoughtFile(t, root, "Source.md", "source\n")
			putOpenThoughtFile(t, root, "LooksLikeAnAnswer.md", openThoughtNote("Never guessed", "reflection", "started", "2026-09-01"))
			site, marks := openThoughtSite(t, root)
			kept := &mark.Uncertainty{RelPath: "Source.md", At: time.Now()}
			if _, err := marks.ToggleUncertainty(kept); err != nil {
				t.Fatal(err)
			}
			page := openThoughtPage(t, site, "/open-thoughts", "en")
			if strings.Count(page, "data-index-row") != 1 || strings.Contains(page, "Never guessed") {
				t.Error("ungoverned/invalid shelf must show marks without guessing an answer role")
			}
			if _, err := marks.ToggleUncertainty(kept); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(openThoughtPage(t, site, "/", "en"), `data-home-block="open-thoughts"`) {
				t.Error("an empty marks-only block displaced the ordinary folder entry")
			}
			for lang, sentence := range map[string]string{"en": `No locations marked "Not sure yet".`, "zh-Hant": "尚無「還不確定」的位置標記。"} {
				if !strings.Contains(openThoughtPage(t, site, "/open-thoughts", lang), sentence) {
					t.Errorf("marks-only empty state is missing %q", sentence)
				}
			}
			const corrupt = "{broken"
			if err := os.WriteFile(marks.UncertaintyPath(), []byte(corrupt), 0o600); err != nil {
				t.Fatal(err)
			}
			page = openThoughtPage(t, site, "/open-thoughts", "en")
			if !strings.Contains(page, "Marks cannot be read right now") || strings.Contains(page, "No locations marked") || strings.Contains(page, "0 items") {
				t.Error("corrupt marks were presented as an empty shelf rather than unavailable")
			}
			if strings.Contains(page, "Contract capabilities") || strings.Contains(page, "Some pages cannot be shown because of it") {
				t.Error("the reader's damaged mark file was mislabeled as a vault contract failure")
			}
			home := openThoughtPage(t, site, "/", "en")
			if !strings.Contains(home, "Marks cannot be read right now") || strings.Contains(openThoughtBlock(t, home), "No locations marked") {
				t.Error("Home hid unavailable marks behind an empty shelf")
			}
			data, err := os.ReadFile(marks.UncertaintyPath())
			if err != nil || string(data) != corrupt {
				t.Error("reading Home overwrote corrupt mark data")
			}
		})
	}
}

func TestOpenThoughtsEmptyNamesEveryInitialStage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, openThoughtContract)
	site, _ := openThoughtSite(t, root)
	for lang, sentence := range map[string]string{
		"en":      "No marked locations or notes of type reflection in any of these initial statuses: started, revisit.",
		"zh-Hant": "尚無標記位置，也沒有類型為 reflection、處於以下任一初始狀態的筆記：started、revisit。",
	} {
		if page := openThoughtPage(t, site, "/open-thoughts", lang); !strings.Contains(page, sentence) {
			t.Errorf("declared empty state missing %q", sentence)
		}
	}
}

func TestOpenThoughtsDoesNotTreatInferredInitialAsDeclared(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	legacy := strings.ReplaceAll(strings.ReplaceAll(openThoughtContract, "initial = true\n", ""), "initial = false\n", "")
	putOpenThoughtFile(t, root, schema.ContractRelPath, legacy)
	putOpenThoughtFile(t, root, "Inside/Legacy.md", openThoughtNote("Legacy capture", "reflection", "started", "2026-09-01"))
	site, _ := openThoughtSite(t, root)
	page := openThoughtPage(t, site, "/open-thoughts", "en")
	if strings.Contains(page, "Legacy capture") || !strings.Contains(page, "No locations marked") {
		t.Error("inferred initial status acquired an undeclared open-thought role")
	}
}

func TestOpenThoughtsRevokedContractKeepsOnlyMarks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, openThoughtContract)
	putOpenThoughtFile(t, root, "Inside/Thought.md", openThoughtNote("Withdrawn role", "reflection", "started", "2026-09-01"))
	putOpenThoughtFile(t, root, "Source.md", "source\n")
	site, marks := openThoughtSite(t, root)
	if _, err := marks.ToggleUncertainty(&mark.Uncertainty{RelPath: "Source.md", At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	putOpenThoughtFile(t, root, schema.ContractRelPath, "broken contract")
	page := openThoughtPage(t, site, "/open-thoughts", "en")
	if strings.Count(page, "data-index-row") != 1 || strings.Contains(page, "Withdrawn role") {
		t.Error("revoked contract kept an answer-role row or lost the independent mark")
	}
}

func TestOpenThoughtsReadFailureIsNotCompletion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, openThoughtContract)
	const rel = "Inside/Thought.md"
	putOpenThoughtFile(t, root, rel, openThoughtNote("Disappearing file", "reflection", "started", "2026-09-01"))
	site, _ := openThoughtSite(t, root)
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
	page := openThoughtPage(t, site, "/open-thoughts", "en")
	if !strings.Contains(page, "current statuses could not be read") || strings.Contains(page, "No marked locations") || strings.Contains(page, "0 items") {
		t.Error("failed live read was presented as completion or an empty complete shelf")
	}
}

func openThoughtSite(t *testing.T, root string) (*readingSite, *mark.File) {
	t.Helper()
	config := t.TempDir()
	site, err := newReadingSite(t.Context(), root, config, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := site.close(); err != nil {
			t.Error(err)
		}
	})
	// Keep one generation so the live-status assertion cannot pass because a
	// background scan happened between writing and reading.
	site.cancel()
	site.watchers.Wait()
	marks, err := mark.New(config, site.source.Name())
	if err != nil {
		t.Fatal(err)
	}
	return site, marks
}

func openThoughtPage(t *testing.T, site *readingSite, address, lang string) string {
	t.Helper()
	req := siteRequest(t, http.MethodGet, address, http.NoBody)
	req.AddCookie(&http.Cookie{Name: "yomihon_lang", Value: lang})
	recorder := httptest.NewRecorder()
	site.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d: %s", address, recorder.Code, recorder.Body.String())
	}
	return html.UnescapeString(recorder.Body.String())
}

func openThoughtBlock(t *testing.T, page string) string {
	t.Helper()
	_, rest, ok := strings.Cut(page, `data-home-block="open-thoughts"`)
	if !ok {
		t.Fatal("Home has no open-thoughts block")
	}
	block, _, ok := strings.Cut(rest, "</section>")
	if !ok {
		t.Fatal("open-thoughts block is not closed")
	}
	return block
}

func putOpenThoughtFile(t *testing.T, root, rel, content string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func openThoughtNote(title, kind, status, updated string) string {
	return fmt.Sprintf("---\ntitle: %q\ntype: %s\nstatus: %s\nupdated: %s\nbased_on: '[[Source#Chapter]]'\n---\nA thought.\n", title, kind, status, updated)
}
