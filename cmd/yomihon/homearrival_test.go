package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

const readingClubNote = "# Reading club\n\nBring a notebook.\n\nUmbrellaRule: If it rains, meet inside.\n"

// homeBlockMarker matches the ways in the desk draws. The search row carries
// the same attribute but is not a way in, so it is left out.
var homeBlockMarker = regexp.MustCompile(`data-home-block="(folders|paths|maps|reports)"`)

// homeBlockOrder is the order the desk draws its ways in.
func homeBlockOrder(page string) []string {
	var order []string
	for _, m := range homeBlockMarker.FindAllStringSubmatch(page, -1) {
		order = append(order, m[1])
	}
	return order
}

// homeSite composes the production site over a temporary vault holding files.
func homeSite(t *testing.T, files map[string]string) http.Handler {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	site, err := newReadingSite(t.Context(), root, t.TempDir(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("newReadingSite: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := site.close(); closeErr != nil {
			t.Errorf("readingSite.close() error = %v", closeErr)
		}
	})
	return site
}

// TestPlainFolderHomeLeadsWithFoldersAndStaysReadable follows a populated
// folder that has no contract from Home to a note, to search, and to the three
// organised modes.
func TestPlainFolderHomeLeadsWithFoldersAndStaysReadable(t *testing.T) {
	t.Parallel()
	site := homeSite(t, map[string]string{
		"Reading club.md": readingClubNote,
		"Weather.md":      "# Weather\n\nRead the club plan for rain.\n",
	})
	home := readingPage(t, site, "/")

	if got, want := homeBlockOrder(home), []string{"folders", "paths", "maps", "reports"}; !slices.Equal(got, want) {
		t.Errorf("plain folder Home blocks = %v, want %v", got, want)
	}

	folders := deskBlockMarkup(t, home, "folders")
	const noteHref = "/notes/Reading%20club.md"
	if !strings.Contains(folders, `href="`+noteHref+`" data-desk-item`) {
		t.Fatalf("the folders block offers no desk link to %s: %s", noteHref, folders)
	}
	if note := readingPage(t, site, noteHref); !strings.Contains(note, "If it rains, meet inside.") {
		t.Errorf("%s does not carry the note body", noteHref)
	}

	if !strings.Contains(home, `<form class="y-homesearch" method="get" action="/search">`) {
		t.Error("Home lacks the native GET form to /search")
	}
	results := readingPage(t, site, "/search?q=UmbrellaRule")
	if !strings.Contains(results, `href="`+noteHref) {
		t.Errorf("searching Home's term does not reach %s", noteHref)
	}

	for _, mode := range []string{"paths", "maps", "reports"} {
		if !strings.Contains(deskBlockMarkup(t, home, mode), `href="/`+mode+`"`) {
			t.Errorf("the %s block does not link to /%s", mode, mode)
		}
		readingPage(t, site, "/"+mode)
	}
}

// TestEmptyPlainFolderGuidesWithoutAContract holds the empty-folder next step
// in both languages on Home and on the folder index.
func TestEmptyPlainFolderGuidesWithoutAContract(t *testing.T) {
	t.Parallel()
	site := homeSite(t, nil)
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		want := wording.FolderIndexUngovernedNext.In(lang)
		if lang == wording.ZhHant && want != "用你的編輯器在這個資料夾裡新增 .md 檔，就能開始閱讀；不需要契約。" {
			t.Fatalf("zh-Hant guidance = %q", want)
		}
		if lang == wording.En && want != "Use your editor to add a .md file to this folder and start reading; no contract is needed." {
			t.Fatalf("en guidance = %q", want)
		}
		if got := deskBlockMarkup(t, readingPageIn(t, site, "/", lang), "folders"); !strings.Contains(got, want) {
			t.Errorf("%s Home folders block lacks %q", lang, want)
		}
		if got := readingPageIn(t, site, "/folders", lang); !strings.Contains(got, want) {
			t.Errorf("%s folder index lacks %q", lang, want)
		}
	}
}

// TestGovernedHomeKeepsItsOrder holds the order a present contract already had.
func TestGovernedHomeKeepsItsOrder(t *testing.T) {
	t.Parallel()
	contract, err := os.ReadFile(filepath.Join("..", "..", "internal", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatalf("read schema fixture: %v", err)
	}
	site := homeSite(t, map[string]string{
		schema.ContractRelPath: string(contract),
		"Reading club.md":      readingClubNote,
	})
	if got, want := homeBlockOrder(readingPage(t, site, "/")), []string{"paths", "maps", "reports", "folders"}; !slices.Equal(got, want) {
		t.Errorf("governed Home blocks = %v, want %v", got, want)
	}
}
