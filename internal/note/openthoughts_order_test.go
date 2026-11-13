package note_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
)

// shelfVault is two notes that were read and two thoughts written from them,
// one of each pair marked on a different day, so that every order puts the four
// rows somewhere different. The file names sort against the dates.
func shelfVault() map[string]string {
	return map[string]string{
		"Notes/Source.md": "---\ntype: note\n---\n# Source\n\n## Cancel\n\nCancelling is a signal, not a stop. It needs help.\n\n" +
			"## Wait\n\n<ruby>辞書<rt>じしょ</rt></ruby>はここにあります。次の文。\n\n## Bare\n",
		"Notes/Other.md":   "---\ntype: note\n---\n# Other\n\n## Part\n\nOther's opening. And more.\n",
		"Writing/Zed.md":   "---\ntype: writing\ndraft: true\ncreated: 2026-03-01\nbased_on: \"[[Other]]\"\n---\n# Thought on other\n\nThought one. More.\n",
		"Writing/Alpha.md": "---\ntype: writing\ndraft: true\ncreated: 2026-03-03\nbased_on: \"[[Source#Cancel]]\"\n---\n# Thought on cancel\n\nThought two. More.\n",
	}
}

func shelfMarks() []mark.Uncertainty {
	return []mark.Uncertainty{
		{RelPath: "Notes/Source.md", Anchor: "cancel", At: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)},
		{RelPath: "Notes/Other.md", Anchor: "part", At: time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)},
	}
}

// shelfServer serves the shelf over one vault, with the thought role held by the
// writing type. Rebuild reads the folder again into a new generation, which is
// what happens when an author saves a note.
type shelfServer struct {
	mux     http.Handler
	root    string
	rebuild func(t *testing.T)
}

func newShelfServer(t *testing.T, files map[string]string, marks []mark.Uncertainty) *shelfServer {
	t.Helper()
	root := writeNotes(t, files)
	data, err := os.ReadFile(filepath.Join("..", "schema", "testdata", "contract.toml"))
	if err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(t.TempDir(), "vault-schema.toml")
	if err = os.WriteFile(contractPath, []byte(writingHoldsTheThoughtRole(t, string(data))), 0o600); err != nil { // #nosec G703 -- a temporary file this test just named
		t.Fatal(err)
	}
	contract, err := schema.LoadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, contract, contract.Governance())
	var current atomic.Pointer[snapshot.Generation]
	current.Store(store.Current())
	handler := note.New(&note.Sources{
		Source: source, Contract: contract,
		Authority:     func() schema.Authority { return schema.CaptureAuthority(contract, contract.Governance()) },
		Snapshot:      current.Load,
		Continuation:  noMark,
		Uncertainties: func() ([]mark.Uncertainty, error) { return marks, nil },
		Log:           log,
	})
	mux := http.NewServeMux()
	handler.Register(mux)
	return &shelfServer{mux: mux, root: root, rebuild: func(t *testing.T) {
		t.Helper()
		rebuilt, rebuildErr := snapshot.New(t.Context(), source, log, contract, contract.Governance())
		if rebuildErr != nil {
			t.Fatal(rebuildErr)
		}
		current.Store(rebuilt.Current())
	}}
}

func (s *shelfServer) page(t *testing.T, address, lang string) string {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, address, http.NoBody)
	req.AddCookie(&http.Cookie{Name: "yomihon_lang", Value: lang}) // #nosec G124 -- synthetic incoming Cookie header
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	body, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d: %s", address, w.Code, body)
	}
	return string(body)
}

// TestTheShelfIsOrderedNewestFirstByDefaultAndByTheThreeChoices holds the three
// orders over one fixture: newest first, oldest first, and by source — the
// places marked in a note and the thoughts written from it standing together,
// the groups by the source's path and each group newest first. The shelf opens
// newest first however it was left, and a word outside the set changes nothing.
func TestTheShelfIsOrderedNewestFirstByDefaultAndByTheThreeChoices(t *testing.T) {
	t.Parallel()

	shelf := newShelfServer(t, shelfVault(), shelfMarks())
	newest := []string{"Other › Part", "Thought on cancel", "Source › Cancel", "Thought on other"}
	for address, want := range map[string][]string{
		"/open-thoughts":                newest,
		"/open-thoughts?order=newest":   newest,
		"/open-thoughts?order=oldest":   {"Thought on other", "Source › Cancel", "Thought on cancel", "Other › Part"},
		"/open-thoughts?order=source":   {"Other › Part", "Thought on other", "Thought on cancel", "Source › Cancel"},
		"/open-thoughts?order=sideways": newest,
		"/open-thoughts?order=":         newest,
	} {
		if diff := cmp.Diff(want, openNames(t, shelf.page(t, address, "en"))); diff != "" {
			t.Errorf("GET %s order (-want +got):\n%s", address, diff)
		}
	}
	for address, want := range map[string]string{
		"/open-thoughts": "Newest first", "/open-thoughts?order=source": "By source",
		"/open-thoughts?order=oldest": "Oldest first", "/open-thoughts?order=nonsense": "Newest first",
	} {
		page := shelf.page(t, address, "en")
		_, rest, ok := strings.Cut(page, `aria-pressed="true">`)
		if !ok || strings.Count(page, `aria-pressed="true"`) != 1 {
			t.Fatalf("GET %s does not press exactly one order", address)
		}
		if got, _, _ := strings.Cut(rest, "<"); got != want {
			t.Errorf("GET %s presses %q, want %q", address, got, want)
		}
	}
	if page := shelf.page(t, "/open-thoughts?order=source", "zh-Hant"); !strings.Contains(page, ">排序</span>") || !strings.Contains(page, `value="source" aria-pressed="true">依來源<`) {
		t.Errorf("the order control is not said in the reader's language")
	}
}

// TestADatelessThoughtStaysAfterTheDatedOnesWhicheverWayTheDatesRun holds that a
// thought with no day is neither the earliest nor the latest.
func TestADatelessThoughtStaysAfterTheDatedOnesWhicheverWayTheDatesRun(t *testing.T) {
	t.Parallel()

	files := shelfVault()
	files["Writing/Undated.md"] = "---\ntype: writing\ndraft: true\n---\n# Undated thought\n\nNo day.\n"
	shelf := newShelfServer(t, files, shelfMarks())
	for _, order := range []string{"newest", "oldest"} {
		names := openNames(t, shelf.page(t, "/open-thoughts?order="+order, "en"))
		if got := names[len(names)-1]; got != "Undated thought" {
			t.Errorf("in order %q the undated thought stands at %q, want last: %v", order, got, names)
		}
	}
}

// openingsOf is what each row of the shelf says under its name, keyed by the
// name; an empty string is a row that says nothing there.
func openingsOf(t *testing.T, page string) map[string]string {
	t.Helper()
	root, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	classed := func(node *html.Node, class string) bool {
		return node.Type == html.ElementNode && slices.ContainsFunc(node.Attr, func(a html.Attribute) bool {
			return a.Key == "class" && slices.Contains(strings.Fields(a.Val), class)
		})
	}
	var gather func(*html.Node, *strings.Builder)
	gather = func(node *html.Node, out *strings.Builder) {
		if node.Type == html.TextNode {
			out.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			gather(child, out)
		}
	}
	text := func(node *html.Node) string {
		var out strings.Builder
		gather(node, &out)
		return strings.Join(strings.Fields(out.String()), " ")
	}
	openings := map[string]string{}
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if classed(node, "y-openitem") {
			var name, opening string
			var find func(*html.Node)
			find = func(n *html.Node) {
				switch {
				case classed(n, "y-openitem__name"):
					name = text(n)
				case classed(n, "y-openitem__opening"):
					opening = text(n)
				}
				for child := n.FirstChild; child != nil; child = child.NextSibling {
					find(child)
				}
			}
			find(node)
			openings[name] = opening
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return openings
}

// TestEachRowCarriesTheSentenceOfItsPlaceReadAsTheNoteIsNow holds what a row
// says under its name: the first sentence of the section it names, of the
// thought for a thought, and nothing for a mark on the whole note or a section
// that holds no prose. The note is read when the page is drawn, so a sentence
// the author rewrote since the mark is the rewritten one, and a Japanese
// sentence is drawn without its readings.
func TestEachRowCarriesTheSentenceOfItsPlaceReadAsTheNoteIsNow(t *testing.T) {
	t.Parallel()

	marks := append(shelfMarks(),
		mark.Uncertainty{RelPath: "Notes/Source.md", Anchor: "wait", At: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)},
		mark.Uncertainty{RelPath: "Notes/Source.md", Anchor: "bare", At: time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)},
		mark.Uncertainty{RelPath: "Notes/Source.md", At: time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)},
	)
	shelf := newShelfServer(t, shelfVault(), marks)
	want := map[string]string{
		"Source › Cancel":   "Cancelling is a signal, not a stop.",
		"Source › Wait":     "辞書はここにあります。",
		"Source › Bare":     "",
		"Source":            "",
		"Other › Part":      "Other's opening.",
		"Thought on cancel": "Thought two.",
		"Thought on other":  "Thought one.",
	}
	if diff := cmp.Diff(want, openingsOf(t, shelf.page(t, "/open-thoughts", "en"))); diff != "" {
		t.Errorf("the sentence under each row (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(want, openingsOf(t, shelf.page(t, "/open-thoughts", "zh-Hant"))); diff != "" {
		t.Errorf("the sentence under each row in the other language (-want +got):\n%s", diff)
	}

	rewritten := "---\ntype: note\n---\n# Source\n\n## Cancel\n\nRewritten since the mark. Then more.\n\n## Wait\n\nWait.\n\n## Bare\n"
	if err := os.WriteFile(filepath.Join(shelf.root, "Notes", "Source.md"), []byte(rewritten), 0o600); err != nil {
		t.Fatal(err)
	}
	shelf.rebuild(t)
	got := openingsOf(t, shelf.page(t, "/open-thoughts", "en"))
	if got["Source › Cancel"] != "Rewritten since the mark." || got["Source › Wait"] != "Wait." {
		t.Errorf("the sentence is not the note as it stands now: %v", got)
	}
}

// TestAThoughtIsLabelledAndNamesTheNotesItCameFrom holds the label a reader's
// own note carries, the kind it is told from a mark by, and the line of notes it
// declares: each a link to the place the author named.
func TestAThoughtIsLabelledAndNamesTheNotesItCameFrom(t *testing.T) {
	t.Parallel()

	shelf := newShelfServer(t, shelfVault(), shelfMarks())
	for lang, labels := range map[string][]string{
		"en":      {">Thought</span>", ">Draft</span>", "<span>Declared sources</span>"},
		"zh-Hant": {">想法</span>", ">草稿</span>", "<span>聲明的來源</span>"},
	} {
		page := shelf.page(t, "/open-thoughts", lang)
		for _, want := range labels {
			if !strings.Contains(page, want) {
				t.Errorf("the %s shelf lacks %q", lang, want)
			}
		}
		for _, want := range []string{
			`<a href="/notes/Notes/Source.md#cancel">Source › Cancel</a>`,
			`<a href="/notes/Notes/Other.md">Other</a>`,
		} {
			if !strings.Contains(page, want) {
				t.Errorf("the %s shelf does not link the declared source %q", lang, want)
			}
		}
		if strings.Contains(page, "[[") {
			t.Errorf("the %s shelf prints a declaration as the author wrote it, brackets and all", lang)
		}
	}
}
