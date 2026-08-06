package vault_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/vault"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		wantTitle  string
		wantStatus string
		wantSlug   string
		wantDiag   bool
		wantInBody string
	}{
		{
			name:       "lesson slug is read as the join key",
			content:    "---\ntitle: L01\ntype: lesson\nstatus: draft\nslug: jp-minna-l01\n---\n\nbody\n",
			wantTitle:  "L01",
			wantStatus: "draft",
			wantSlug:   "jp-minna-l01",
			wantInBody: "body",
		},
		{
			name:       "frontmatter and body",
			content:    "---\ntitle: 數量詞の位置\ntype: concept\nstatus: seedling\n---\n\n# 本文\n",
			wantTitle:  "數量詞の位置",
			wantStatus: "seedling",
			wantInBody: "# 本文",
		},
		{
			name:       "no frontmatter is legal",
			content:    "# 假名 quiz\n",
			wantTitle:  "note",
			wantInBody: "# 假名 quiz",
		},
		{
			name:       "broken yaml yields diagnostic not error",
			content:    "---\ntitle: [broken\n---\nbody\n",
			wantTitle:  "note",
			wantDiag:   true,
			wantInBody: "body",
		},
		{
			name:       "wikilink value survives the split",
			content:    "---\ntitle: t\nbased_on: \"[[大家的日本語 第11課]]\"\n---\nbody\n",
			wantTitle:  "t",
			wantInBody: "body",
		},
		{
			// The concept-resolver corruption guard, at the layer where the
			// guarantee lives: a lesson's frontmatter based_on holds
			// [[wikilinks]]; because the split happens here, before any body
			// preprocessing, a later concept resolver rewriting [[...]] to a
			// trigger can never reach the YAML and empty the meta (which would
			// drop status:ready and hide the lesson). The status must survive
			// intact alongside the wikilink-valued field.
			name:       "concept resolver cannot corrupt frontmatter status",
			content:    "---\ntitle: L00 テスト課\ntype: lesson\nstatus: ready\nbased_on: \"[[大家的日本語 第1課]]\"\nslug: jp-minna-l00\n---\n\nSee [[は]] and [[です]].\n",
			wantTitle:  "L00 テスト課",
			wantStatus: schema.SealStatus,
			wantSlug:   "jp-minna-l00",
			wantInBody: "[[は]]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse("Concepts/note.md", []byte(tt.content))
			if got := n.Title(); got != tt.wantTitle {
				t.Errorf("Title() = %q, want %q", got, tt.wantTitle)
			}
			if got := n.Status(); got != tt.wantStatus {
				t.Errorf("Status() = %q, want %q", got, tt.wantStatus)
			}
			if got := n.Slug(); got != tt.wantSlug {
				t.Errorf("Slug() = %q, want %q", got, tt.wantSlug)
			}
			if (n.FMDiagnostic != "") != tt.wantDiag {
				t.Errorf("FMDiagnostic = %q, want diagnostic: %v", n.FMDiagnostic, tt.wantDiag)
			}
			if !strings.Contains(n.Body, tt.wantInBody) {
				t.Errorf("Body does not contain %q:\n%s", tt.wantInBody, n.Body)
			}
		})
	}
}

func TestParseFrontmatterBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		content    string
		wantTitle  string
		wantStatus string
		wantBody   string
	}{
		{name: "LF dash closer", content: "---\ntitle: LF\nstatus: draft\n---\nbody\n", wantTitle: "LF", wantStatus: "draft", wantBody: "body\n"},
		{name: "CRLF dash closer", content: "---\r\ntitle: CRLF\r\nstatus: ready\r\n---\r\nbody\r\n", wantTitle: "CRLF", wantStatus: "ready", wantBody: "body\r\n"},
		{name: "YAML document closer", content: "---\ntitle: Dots\nstatus: draft\n...\nbody\n", wantTitle: "Dots", wantStatus: "draft", wantBody: "body\n"},
		{name: "closer at EOF", content: "---\ntitle: EOF\nstatus: ready\n---", wantTitle: "EOF", wantStatus: "ready", wantBody: ""},
		{name: "unterminated is body", content: "---\ntitle: Open\nstatus: draft\nbody\n", wantTitle: "note", wantStatus: "", wantBody: "---\ntitle: Open\nstatus: draft\nbody\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse("note.md", []byte(tt.content))
			if got := n.Title(); got != tt.wantTitle {
				t.Errorf("Title() = %q, want %q", got, tt.wantTitle)
			}
			if got := n.Status(); got != tt.wantStatus {
				t.Errorf("Status() = %q, want %q", got, tt.wantStatus)
			}
			if n.Body != tt.wantBody {
				t.Errorf("Body = %q, want %q", n.Body, tt.wantBody)
			}
		})
	}
}

// TestSplitFrontmatterStepsOverAByteOrderMark asserts a note whose first bytes
// are a byte-order mark is read as the note it is, and that stepping over the
// mark does not move the offsets the status writer splices against.
//
// Some editors write the mark. A note carrying one looks ordinary to a reader
// and to Obsidian, but its opening fence stopped being recognized here, so its
// whole frontmatter block was read as body: the title, the type and the place
// in the lifecycle all vanished at once, and every face agreed the note simply
// had none — which is a legal state, so nothing anywhere reported a fault.
func TestSplitFrontmatterStepsOverAByteOrderMark(t *testing.T) {
	t.Parallel()
	const mark = "\xef\xbb\xbf"
	tests := []struct {
		name             string
		data             string
		wantFound        bool
		wantContent      string
		wantBody         string
		wantContentStart int
		wantBodyLine     int
	}{
		{
			name: "mark before an ordinary fence", data: mark + "---\ntitle: A\n---\nbody\n",
			wantFound: true, wantContent: "title: A\n", wantBody: "body\n",
			wantContentStart: 7, wantBodyLine: 4,
		},
		{
			name: "mark before a carriage-return fence", data: mark + "---\r\ntitle: A\r\n---\r\nbody\r\n",
			wantFound: true, wantContent: "title: A\r\n", wantBody: "body\r\n",
			wantContentStart: 8, wantBodyLine: 4,
		},
		{
			name: "mark with no fence after it stays body", data: mark + "just body\n",
			wantFound: false, wantContent: "", wantBody: mark + "just body\n",
			wantContentStart: 0, wantBodyLine: 1,
		},
		{
			name: "a mark that is not first is not a mark", data: "x" + mark + "---\ntitle: A\n---\n",
			wantFound: false, wantContent: "", wantBody: "x" + mark + "---\ntitle: A\n---\n",
			wantContentStart: 0, wantBodyLine: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, found := vault.SplitFrontmatter([]byte(tt.data))
			if found != tt.wantFound {
				t.Fatalf("SplitFrontmatter(%q) found = %t, want %t", tt.data, found, tt.wantFound)
			}
			if string(got.Content) != tt.wantContent {
				t.Errorf("SplitFrontmatter(%q) content = %q, want %q", tt.data, got.Content, tt.wantContent)
			}
			if string(got.Body) != tt.wantBody {
				t.Errorf("SplitFrontmatter(%q) body = %q, want %q", tt.data, got.Body, tt.wantBody)
			}
			if got.ContentStart != tt.wantContentStart {
				t.Errorf("SplitFrontmatter(%q) content start = %d, want %d", tt.data, got.ContentStart, tt.wantContentStart)
			}
			if got.BodyStartLine != tt.wantBodyLine {
				t.Errorf("SplitFrontmatter(%q) body start line = %d, want %d", tt.data, got.BodyStartLine, tt.wantBodyLine)
			}
			// The offset has to name the frontmatter's place in the bytes that
			// were handed in, not in a trimmed copy of them: the status writer
			// slices the original file at it.
			if found && string([]byte(tt.data)[got.ContentStart:got.ContentStart+len(got.Content)]) != tt.wantContent {
				t.Errorf("SplitFrontmatter(%q) content start %d does not locate the content in the original bytes", tt.data, got.ContentStart)
			}
		})
	}
}
