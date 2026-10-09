package note_test

import (
	"context"
	"html"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/koopa0/yomihon/internal/nav"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/shell"
	"github.com/koopa0/yomihon/internal/status"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// The pair the folder below comes to hold: one name as a Linux keyboard writes
// it, and the same name as a macOS-to-Linux copy or a git checkout leaves it.
const (
	frozenComposed   = "Notes/\u304c.md"
	frozenDecomposed = "Notes/\u304b\u3099.md"
	frozenFresh      = "Notes/Fresh.md"
)

// runningFolder serves a folder the way the binary does: a snapshot store whose
// reconciliation loop is running, so the folder can change under the pages. The
// loop ticks every couple of seconds in real time, which is the cost of
// watching the whole chain from the file system to the page.
func runningFolder(t *testing.T, root string) *httptest.Server {
	t.Helper()
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	writer := openStatusWriter(t, source, nil, schema.Ungoverned())
	h := note.New(&note.Sources{
		Source:           source,
		VaultName:        shell.VaultName(source.Name()),
		Status:           writer.Authority,
		Snapshot:         store.Current,
		RequestReconcile: func() {},
		ObservedStatus:   writer.ObservedStatus,
		ConsumeReceipt:   writer.ConsumeReceipt,
		Continuation:     noMark,
		Log:              log,
	})
	mux := http.NewServeMux()
	h.Register(mux)
	status.NewHandler(writer, func() nav.Shell { return nav.Shell{} }, log).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		store.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return srv
}

// waitUntil polls a condition against the running loop. The loop scans every
// scanInterval, so the wait is bounded by a few of them rather than by a guess
// at how long one tick takes.
func waitUntil(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("gave up waiting until %s", what)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// railFootClaim is the number the foot of a page's rail states.
func railFootClaim(t *testing.T, page string) int {
	t.Helper()
	claim := railFootFindings.FindStringSubmatch(page)
	if claim == nil {
		t.Fatal("the page carries no rail foot")
	}
	count, err := strconv.Atoi(claim[1])
	if err != nil {
		t.Fatalf("the rail foot claims %q findings, which is no number: %v", claim[1], err)
	}
	return count
}

// railFootState is the state the foot's dot is drawn in, which the stylesheet
// colours and which is carried on the element beside the words.
var railFootState = regexp.MustCompile(`data-rail-foot data-health="(\w+)"`)

func railFootDot(t *testing.T, page string) string {
	t.Helper()
	state := railFootState.FindStringSubmatch(page)
	if state == nil {
		t.Fatal("the page carries no rail foot dot")
	}
	return state[1]
}

// noticeOn cuts the notice a page draws out of it: from the marker to the end of
// the section that holds it. The rail lists every file, so a name found
// anywhere on the page proves nothing; what is read here is the notice itself.
func noticeOn(t *testing.T, page, marker string) string {
	t.Helper()
	_, after, ok := strings.Cut(page, marker)
	if !ok {
		t.Fatalf("the page does not carry %s", marker)
	}
	notice, _, ok := strings.Cut(after, "</section>")
	if !ok {
		t.Fatalf("the notice after %s is not closed", marker)
	}
	return notice
}

// TestAFrozenFolderSaysSoOnTheDeskAndTheHealthPage follows the fault the way a
// reader met it. Two names with one canonical path stopped the scan for good:
// a note written beside them answered 404 indefinitely, and neither the desk
// nor the health page said anything. Both now say that the pages have stopped
// updating, name the two files, and say it in the language the reader chose;
// the rail's dot stops saying clear on every page; and taking one of the pair
// away brings the folder back with nothing restarted.
func TestAFrozenFolderSaysSoOnTheDeskAndTheHealthPage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for rel, body := range map[string]string{
		// Two notes that cite each other, so that the folder has no uncited note
		// and nothing against it: the foot can then read clear, and a freeze is
		// the only thing that could change it.
		"Notes/Existing.md": "---\ntitle: Existing\ntype: concept\n---\nsee [[" + strings.TrimSuffix(path.Base(frozenComposed), ".md") + "]]\n",
		frozenComposed:      "---\ntitle: Ga\ntype: concept\n---\nsee [[Existing]]\n",
	} {
		writeVaultNote(t, root, rel, body)
	}
	srv := runningFolder(t, root)
	client := srv.Client()
	quiet := getInLanguage(t, client, srv.URL+"/health", wording.En)
	if dot, claim := railFootDot(t, quiet), railFootClaim(t, quiet); dot != "clear" || claim != 0 {
		t.Fatalf("the folder before the pair has a %q dot and %d findings, so a freeze has nothing to change", dot, claim)
	}

	writeVaultNote(t, root, frozenDecomposed, "---\ntitle: Ga\ntype: concept\n---\nga\n")
	entries, err := os.ReadDir(filepath.Join(root, "Notes"))
	if err != nil {
		t.Fatalf("ReadDir(Notes): %v", err)
	}
	var listed []string
	for _, entry := range entries {
		listed = append(listed, entry.Name())
	}
	if !slices.Contains(listed, path.Base(frozenComposed)) || !slices.Contains(listed, path.Base(frozenDecomposed)) {
		t.Skip("this filesystem folds the two spellings into one file, so there is no pair to refuse")
	}
	writeVaultNote(t, root, frozenFresh, "---\ntitle: Fresh\ntype: concept\n---\nfresh\n")

	waitUntil(t, "the health page says the folder is frozen", func() bool {
		return strings.Contains(getInLanguage(t, client, srv.URL+"/health", wording.En), "data-health-notice")
	})

	if code, _ := get(t, client, srv.URL+"/notes/"+frozenFresh); code != http.StatusNotFound {
		t.Errorf("GET %s status = %d while the folder is frozen, want %d", frozenFresh, code, http.StatusNotFound)
	}
	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		pages := []struct {
			name   string
			url    string
			marker string
		}{
			{name: "the desk", url: srv.URL + "/", marker: `data-home-block="notice"`},
			{name: "the health page", url: srv.URL + "/health", marker: "data-health-notice"},
		}
		for _, p := range pages {
			page := getInLanguage(t, client, p.url, lang)
			seen := noticeOn(t, page, p.marker)
			for _, want := range []string{
				wording.NoticeNamesCollideTitle.In(lang),
				wording.NoticeNamesCollide.In(lang),
				html.EscapeString(vault.Spelled(frozenDecomposed)),
				html.EscapeString(vault.Spelled(frozenComposed)),
			} {
				if !strings.Contains(seen, want) {
					t.Errorf("%s in %s does not say %q; it says %q", p.name, lang, want, seen)
				}
			}
			if other := wording.NoticeNamesCollideTitle.In(lang.Other()); strings.Contains(seen, other) {
				t.Errorf("%s in %s carries the other language's title %q", p.name, lang, other)
			}
		}
	}
	// The rail's foot and the table are one instrument and state one number. A
	// notice is no row of the table, so the number leaves it out and the two
	// still agree while the folder is frozen; the dot is what stops saying clear.
	frozen := getInLanguage(t, client, srv.URL+"/health", wording.En)
	drawn := 0
	for _, row := range rowCount.FindAllStringSubmatch(frozen, -1) {
		count, err := strconv.Atoi(row[1])
		if err != nil {
			t.Fatalf("a findings row counts %q, which is no number: %v", row[1], err)
		}
		drawn += count
	}
	if claim := railFootClaim(t, frozen); claim != drawn {
		t.Errorf("the rail foot claims %d findings and the table's lines hold %d", claim, drawn)
	}
	for _, p := range []struct{ name, url string }{
		{"the health page", srv.URL + "/health"},
		{"a note", srv.URL + "/notes/Notes/Existing.md"},
	} {
		if dot := railFootDot(t, getInLanguage(t, client, p.url, wording.En)); dot != "findings" {
			t.Errorf("the rail foot on %s has a %q dot while the pages have stopped updating, want \"findings\"", p.name, dot)
		}
	}

	removeFile(t, root, frozenDecomposed)
	waitUntil(t, "the folder publishes again", func() bool {
		code, _ := get(t, client, srv.URL+"/notes/"+frozenFresh)
		return code == http.StatusOK
	})
	for _, p := range []struct{ name, url, marker string }{
		{"the desk", srv.URL + "/", `data-home-block="notice"`},
		{"the health page", srv.URL + "/health", "data-health-notice"},
	} {
		if page := getInLanguage(t, client, p.url, wording.En); strings.Contains(page, p.marker) {
			t.Errorf("%s still says the pages have stopped updating once one of the pair is gone", p.name)
		}
	}
}

func removeFile(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("remove %s: %v", rel, err)
	}
}
