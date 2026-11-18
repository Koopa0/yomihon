package note_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	htmlnode "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/koopa0/yomihon/internal/mark"
	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestReadingIsTheWordsOfThePageAndTheIdentityOfTheBytes(t *testing.T) {
	t.Parallel()

	handler, root := thoughtRouteHandler(t, false, "")
	page, ok := handler.Reading("Source.md", wording.ZhHant)
	if !ok {
		t.Fatal("a note the vault holds is not readable")
	}
	if !strings.Contains(page.Text, "PROMPT AND EXPLANATION MUST NOT BE COPIED") || !strings.Contains(page.Text, "Chapter one") {
		t.Errorf("the words of the page are %q, want its heading and its paragraph", page.Text)
	}
	if strings.Contains(page.Text, "type:") || strings.Contains(page.Text, "---") {
		t.Errorf("the words of the page are %q, which carry the frontmatter", page.Text)
	}
	data, err := os.ReadFile(filepath.Join(root, "Source.md")) // #nosec G304 -- a file this test wrote under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(data); page.Identity != hex.EncodeToString(sum[:]) {
		t.Errorf("identity = %s, want the SHA-256 of every byte of the file, %x", page.Identity, sum)
	}
}

func TestReadingRefusesWhatNoReaderOpens(t *testing.T) {
	t.Parallel()

	handler, _ := thoughtRouteHandler(t, false, "")
	for _, rel := range []string{"Missing.md", "../Source.md", "", "Source.txt", "/Source.md"} {
		if page, ok := handler.Reading(rel, wording.En); ok {
			t.Errorf("Reading(%q) = %+v, want a note that is not there", rel, page)
		}
	}
}

// TestEveryWordAPageShowsIsFoundWhereItIs holds what the mark routes depend on:
// a reader who selects the words of one text node of a page, in any note of the
// example vault, has chosen words Locate finds exactly. A rule of TextContent
// that dropped or changed words would show here as a highlight the server
// refuses.
func TestEveryWordAPageShowsIsFoundWhereItIs(t *testing.T) {
	t.Parallel()

	rules := readingLibraryContract(t)
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, readingLibraryRoot, log, rules, rules.Governance())
	handler := note.New(&note.Sources{
		Source: source, Contract: rules, Snapshot: store.Current, Bookmark: noMark, Log: log,
		Authority: func() schema.Authority { return schema.CaptureAuthority(rules, rules.Governance()) },
	})
	snap := store.Current().Capture()

	notes, nodes := 0, 0
	err := filepath.WalkDir(readingLibraryRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		rel, err := filepath.Rel(readingLibraryRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		page, ok := handler.Reading(rel, wording.ZhHant)
		if !ok {
			return nil
		}
		notes++
		reading, _ := snap.Note(rel)
		for _, words := range textNodes(t, snap.Render(rel, reading.Body, wording.ZhHant).HTML) {
			nodes++
			m := &mark.Mark{Kind: mark.Highlight, RelPath: rel, Quote: words}
			if found := mark.Locate(page.Text, nil, m); found.Status != mark.Found || !found.Exact {
				t.Errorf("%s: the words %q of a text node are not found exactly in the page's words (status %v)", rel, words, found.Status)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if notes < 10 || nodes < 100 {
		t.Fatalf("the walk read %d notes and %d text nodes, so it looked at too little to hold anything", notes, nodes)
	}
}

// textNodes is the words of each text node of an HTML fragment that a reader
// can select, one string each, with the white space of each run made one space
// and nothing but white space left out.
func textNodes(t *testing.T, fragment string) []string {
	t.Helper()
	root, err := htmlnode.ParseFragment(strings.NewReader(fragment), &htmlnode.Node{Type: htmlnode.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		t.Fatal(err)
	}
	var words []string
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode && slices.Contains([]string{"script", "style", "template", "rp"}, n.Data) {
			return
		}
		if n.Type == htmlnode.TextNode {
			if text := strings.Join(strings.Fields(n.Data), " "); text != "" {
				words = append(words, text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	for _, n := range root {
		walk(n)
	}
	return words
}
