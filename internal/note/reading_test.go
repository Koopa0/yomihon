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
// example vault, has chosen words Locate finds exactly, or, where the page has
// them at several places and the selection says nothing of what stands around
// them, words it does not place. A rule of TextContent that dropped or changed
// words would show here as a highlight the server refuses.
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
		squeezed := strings.Join(strings.Fields(page.Text), " ")
		for _, words := range textNodes(t, snap.Render(rel, reading.Body, wording.ZhHant).HTML) {
			m := &mark.Mark{Kind: mark.Highlight, RelPath: rel, Quote: words}
			found := mark.Locate(page.Text, nil, m)
			if places(squeezed, words) > 1 {
				if found.Status != mark.Changed {
					t.Errorf("%s: the words %q of a text node are at several places of the page's words, and Locate answers status %v", rel, words, found.Status)
				}
				continue
			}
			nodes++
			if found.Status != mark.Found || !found.Exact {
				t.Errorf("%s: the words %q of a text node are not found exactly in the page's words (status %v)", rel, words, found.Status)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if notes < 10 || nodes < 100 {
		t.Fatalf("the walk read %d notes and %d text nodes it could place, so it looked at too little to hold anything", notes, nodes)
	}
}

// TestAQuoteOfTheBaseTextAcrossRubyIsFoundWhereItIs holds what a reader who
// selects across a ruby has chosen: the base text, the reading being no part of
// what is selected. Locate finds those words exactly, at the sentence they are
// in, whether the author wrote the reading's brackets or not and however many
// pairs of base and reading the ruby holds.
func TestAQuoteOfTheBaseTextAcrossRubyIsFoundWhereItIs(t *testing.T) {
	t.Parallel()

	const rel = "Notes/Snow.md"
	root := writeNotes(t, map[string]string{rel: "---\ntype: note\n---\n# Snow\n\n" +
		"国境の長いトンネルを抜けると<ruby>雪国<rp>(</rp><rt>ゆきぐに</rt><rp>)</rp></ruby>であった。夜の底が白くなった。\n\n" +
		"<ruby>東<rt>とう</rt>京<rt>きょう</rt></ruby>と<ruby>仮名</ruby>を読む。\n"})
	log := slog.New(slog.DiscardHandler)
	store, source := newSnapshotStore(t, root, log, nil, schema.Ungoverned())
	handler := note.New(&note.Sources{
		Source: source, Snapshot: store.Current, Bookmark: noMark, Log: log,
		Authority: func() schema.Authority { return schema.CaptureAuthority(nil, schema.Ungoverned()) },
	})
	page, ok := handler.Reading(rel, wording.En)
	if !ok {
		t.Fatalf("Reading(%q) is not a note a reader can open", rel)
	}

	for _, tt := range []struct {
		name, quote string
		want        mark.Status
	}{
		{"across a ruby that has brackets", "国境の長いトンネルを抜けると雪国であった。", mark.Found},
		{"the base alone", "雪国", mark.Found},
		{"from inside a base to past it", "国であった。夜の底", mark.Found},
		{"across a ruby of two pairs", "東京と", mark.Found},
		{"across a ruby that has no reading", "と仮名を読む。", mark.Found},
		{"a reading the page does not select", "雪国ゆきぐにであった", mark.Changed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mark.Locate(page.Text, nil, &mark.Mark{Kind: mark.Highlight, RelPath: rel, Quote: tt.quote})
			if got.Status != tt.want {
				t.Fatalf("Locate(%q) status = %v, want %v in %q", tt.quote, got.Status, tt.want, page.Text)
			}
			if tt.want != mark.Found {
				return
			}
			if !got.Exact || page.Text[got.Start:got.End] != tt.quote {
				t.Errorf("Locate(%q) = exact %v at %q, want the words themselves", tt.quote, got.Exact, page.Text[got.Start:got.End])
			}
		})
	}
}

// places is how many places words begin in text, a place that overlaps another
// counting as one more.
func places(text, words string) int {
	n := 0
	for from := 0; ; from++ {
		at := strings.Index(text[from:], words)
		if at < 0 {
			return n
		}
		n++
		from += at
	}
}

// textNodes is the words of each text node of an HTML fragment that a reader
// can select, one string each, with the white space of each run made one space
// and nothing but white space left out. A ruby reading is not selected with
// its base, so it is not among them.
func textNodes(t *testing.T, fragment string) []string {
	t.Helper()
	root, err := htmlnode.ParseFragment(strings.NewReader(fragment), &htmlnode.Node{Type: htmlnode.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		t.Fatal(err)
	}
	var words []string
	var walk func(*htmlnode.Node)
	walk = func(n *htmlnode.Node) {
		if n.Type == htmlnode.ElementNode && slices.Contains([]string{"script", "style", "template", "rp", "rt", "rtc"}, n.Data) {
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
