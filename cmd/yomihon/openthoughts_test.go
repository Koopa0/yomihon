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

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

const openThoughtContract = `schema_version = "1"
[enums]
type = ["reflection", "other"]
[enums.status]
note = ["started", "revisit", "settled"]
[fields]
required = ["title", "type"]
known = ["title", "type", "status", "draft", "updated", "based_on"]
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
		putOpenThoughtFile(t, root, fmt.Sprintf("Inside/Thought%d.md", i), openThoughtNote(fmt.Sprintf("Thought %d", i), "reflection", draftFlag, fmt.Sprintf("2026-09-%02d", i)))
	}
	// A thought outside the knowledge layer is still the reader's own, and
	// only the flag decides: false, absent and a value that is not a boolean
	// are all finished, and a draft of another type is no thought at all.
	putOpenThoughtFile(t, root, "Outside/Alternate.md", openThoughtNote("Outside the layer", "reflection", draftFlag, "2026-09-08"))
	putOpenThoughtFile(t, root, "Inside/Done.md", openThoughtNote("Done note", "reflection", "draft: false\n", "2026-09-10"))
	putOpenThoughtFile(t, root, "Inside/Unflagged.md", openThoughtNote("Unflagged note", "reflection", "", "2026-09-13"))
	putOpenThoughtFile(t, root, "Inside/WrongRole.md", openThoughtNote("Wrong role", "other", draftFlag, "2026-09-11"))
	putOpenThoughtFile(t, root, "Inside/Invalid.md", openThoughtNote("Invalid flag", "reflection", "draft: \"true\"\n", "2026-09-12"))
	site, marks := openThoughtSite(t, root)
	_, err := marks.ToggleUncertainty(&mark.Uncertainty{RelPath: "Source.md", Anchor: "chapter", At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)}, admitMark)
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"en", "zh-Hant"} {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			home := openThoughtPage(t, site, "/", lang)
			block := openThoughtBlock(t, home)
			if got := strings.Count(block, "data-desk-item"); got != 5 {
				t.Errorf("open desk rows = %d, want exactly five", got)
			}
			if got := strings.Count(block, "ui-navitem--wrap"); got != 5 {
				t.Errorf("open desk rows preserving their source location = %d, want five", got)
			}
			if !strings.Contains(block, `href="/open-thoughts"`) || !strings.Contains(block, `class="ui-navitem y-shelfall"`) {
				t.Error("open desk does not expose the existing All shelf link")
			}
			all := openThoughtPage(t, site, "/open-thoughts", lang)
			if got := strings.Count(all, "data-index-row"); got != 8 {
				t.Errorf("complete open shelf rows = %d, want eight", got)
			}
			for _, want := range []string{"Outside the layer", "Thought 1", "[[Source#Chapter]]", `href="/notes/Source.md#chapter"`} {
				if !strings.Contains(all, want) {
					t.Errorf("complete open shelf is missing %q", want)
				}
			}
			for _, unwanted := range []string{"Done note", "Unflagged note", "Wrong role", "Invalid flag"} {
				if strings.Contains(all, unwanted) {
					t.Errorf("open shelf incorrectly includes %q", unwanted)
				}
			}
			if strings.Index(all, "Outside the layer") > strings.Index(all, `href="/notes/Source.md#chapter"`) || strings.Index(all, `href="/notes/Source.md#chapter"`) > strings.Index(all, "Thought 6") {
				t.Error("marks and notes are not interleaved newest first")
			}
			if lang == "en" && !strings.Contains(block, "What you left open") || lang == "zh-Hant" && !strings.Contains(block, "還沒想通的") {
				t.Error("open shelf title is not translated")
			}
		})
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
			putOpenThoughtFile(t, root, "LooksLikeAnAnswer.md", openThoughtNote("Never guessed", "reflection", draftFlag, "2026-09-01"))
			site, marks := openThoughtSite(t, root)
			kept := &mark.Uncertainty{RelPath: "Source.md", At: time.Now()}
			if _, err := marks.ToggleUncertainty(kept, admitMark); err != nil {
				t.Fatal(err)
			}
			page := openThoughtPage(t, site, "/open-thoughts", "en")
			if strings.Count(page, "data-index-row") != 1 || strings.Contains(page, "Never guessed") {
				t.Error("ungoverned/invalid shelf must show marks without guessing an answer role")
			}
			if _, err := marks.ToggleUncertainty(kept, admitMark); err != nil {
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

// TestOpenThoughtsEmptySaysWhatTheShelfHolds pins the empty shelf of a vault
// that declares a thought type: the lede names both sources, the count says
// none, and the marks-only sentence — which would leave the thoughts out — is
// not said.
func TestOpenThoughtsEmptySaysWhatTheShelfHolds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, openThoughtContract)
	site, _ := openThoughtSite(t, root)
	for _, lang := range []wording.Lang{wording.En, wording.ZhHant} {
		page := openThoughtPage(t, site, "/open-thoughts", lang.Tag())
		if !strings.Contains(page, wording.OpenThoughtsLede.In(lang)) {
			t.Errorf("%s: the lede does not say the shelf holds marks and unfinished thoughts", lang.Tag())
		}
		if !strings.Contains(page, fmt.Sprintf(wording.OpenThoughtsCount.In(lang), 0)) {
			t.Errorf("%s: the empty shelf does not count none", lang.Tag())
		}
		if strings.Contains(page, wording.OpenThoughtsMarksEmpty.In(lang)) {
			t.Errorf("%s: the empty shelf speaks of marks alone in a vault that declares thoughts", lang.Tag())
		}
	}
}

func TestOpenThoughtsSkipsDeclaredNonInstances(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, strings.Replace(openThoughtContract, "non_instance_dirs = []", `non_instance_dirs = ["Templates"]`, 1))
	putOpenThoughtFile(t, root, "Inside/Real.md", openThoughtNote("A real thought", "reflection", draftFlag, "2026-09-01"))
	putOpenThoughtFile(t, root, "Templates/Template.md", openThoughtNote("A template row", "reflection", draftFlag, "2026-09-02"))
	site, _ := openThoughtSite(t, root)
	for _, address := range []string{"/", "/open-thoughts"} {
		page := openThoughtPage(t, site, address, "en")
		if !strings.Contains(page, "A real thought") {
			t.Errorf("GET %s lost the real thought", address)
		}
		if strings.Contains(page, "A template row") {
			t.Errorf("GET %s lists a declared non-instance as an open thought", address)
		}
	}
	if !strings.Contains(openThoughtPage(t, site, "/open-thoughts", "en"), "1 item") {
		t.Error("the count includes the non-instance")
	}
}

// The #996 ruling explicitly reverses the former omission oracle: stored paths
// remain visible as inert text even when this generation has no matching note.
// Catches openMarkRow's early return and Home's no-Href filter.
func TestOpenThoughtsKeepsAMarkWhoseNoteIsNotInTheSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, openThoughtContract)
	putOpenThoughtFile(t, root, "Source.md", "# Source\n")
	site, marks := openThoughtSite(t, root)
	const stray = "Visit example dot com for free prizes.md"
	for _, rel := range []string{stray, "Source.md"} {
		if _, err := marks.ToggleUncertainty(&mark.Uncertainty{RelPath: rel, At: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)}, admitMark); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name    string
		lang    string
		missing string
		count   string
	}{
		{name: "english", lang: "en", missing: "Note not found", count: "2 items"},
		{name: "traditional chinese", lang: "zh-Hant", missing: "找不到這篇筆記", count: "2 筆"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, address := range []string{"/", "/open-thoughts"} {
				page := openThoughtPage(t, site, address, tt.lang)
				marker := "data-index-row"
				if address == "/" {
					page = openThoughtBlock(t, page)
					marker = "data-desk-item"
				}
				got := struct {
					Rows                                            int
					StoredPath, Missing, Count, RealMark, FalseLink bool
				}{
					Rows: strings.Count(page, marker), StoredPath: strings.Contains(page, stray),
					Missing: strings.Contains(page, tt.missing), Count: strings.Contains(page, tt.count),
					RealMark: strings.Contains(page, `href="/notes/Source.md"`), FalseLink: strings.Contains(page, `href="/notes/Visit`),
				}
				want := struct {
					Rows                                            int
					StoredPath, Missing, Count, RealMark, FalseLink bool
				}{Rows: 2, StoredPath: true, Missing: true, Count: true, RealMark: true}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("caught: GET %s lost-mark projection mismatch (-want +got):\n%s", address, diff)
				}
			}
		})
	}
}

// TestOpenThoughtsHomeShowsFiveAndCountsEveryOne pins that the desk's block is
// a window onto the shelf rather than a second selection: it draws the five
// rows it has room for and counts every thought the shelf holds.
func TestOpenThoughtsHomeShowsFiveAndCountsEveryOne(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	putOpenThoughtFile(t, root, schema.ContractRelPath, openThoughtContract)
	for i := 1; i <= 7; i++ {
		putOpenThoughtFile(t, root, fmt.Sprintf("Inside/Thought%d.md", i), openThoughtNote(fmt.Sprintf("Thought %d", i), "reflection", draftFlag, fmt.Sprintf("2026-09-%02d", i)))
	}
	site, _ := openThoughtSite(t, root)
	home := openThoughtPage(t, site, "/", "en")
	if got := strings.Count(openThoughtBlock(t, home), "data-desk-item"); got != 5 {
		t.Errorf("Home rows = %d, want five", got)
	}
	if !strings.Contains(openThoughtBlock(t, home), "7 items") {
		t.Error("Home's count is not the shelf's whole count")
	}
	if got := strings.Count(openThoughtPage(t, site, "/open-thoughts", "en"), "data-index-row"); got != 7 {
		t.Errorf("complete shelf rows = %d, want seven", got)
	}
}

func admitMark(*mark.Uncertainty) bool { return true }

func openThoughtSite(t *testing.T, root string) (*readingSite, *mark.File) {
	t.Helper()
	config := t.TempDir()
	site, err := newReadingSite(t.Context(), root, config, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	// Keep one generation so no assertion can pass because a background scan
	// happened between writing and reading.
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
	req.AddCookie(&http.Cookie{Name: "yomihon_lang", Value: lang, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
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

// draftFlag is the frontmatter line that leaves a thought open.
const draftFlag = "draft: true\n"

// openThoughtNote writes a note of kind; flag is a whole frontmatter line, or
// empty for a note that writes none.
func openThoughtNote(title, kind, flag, updated string) string {
	return fmt.Sprintf("---\ntitle: %q\ntype: %s\n%supdated: %s\nbased_on: '[[Source#Chapter]]'\n---\nA thought.\n", title, kind, flag, updated)
}
