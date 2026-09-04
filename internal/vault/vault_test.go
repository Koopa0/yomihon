package vault_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

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

// TestACarriageReturnOnlyFileHasNoFrontmatter pins the line discipline of the
// frontmatter split: lines end at a line feed, alone or after a carriage
// return, and nothing else. A file whose lines end in bare carriage returns
// has no recognizable fence, so the whole file is body — read charity extends
// to CRLF and a byte-order mark, and stops there.
func TestACarriageReturnOnlyFileHasNoFrontmatter(t *testing.T) {
	t.Parallel()

	const content = "---\rtitle: CR\r---\rbody"
	n := vault.Parse("note.md", []byte(content))
	if n.Frontmatter != nil || n.FMDiagnostic != "" {
		t.Errorf("Parse() frontmatter = %#v, diagnostic %q, want none of either", n.Frontmatter, n.FMDiagnostic)
	}
	if n.Body != content {
		t.Errorf("Parse() body = %q, want the whole file", n.Body)
	}
	if n.BodyLine != 1 {
		t.Errorf("Parse() body line = %d, want 1", n.BodyLine)
	}
	if got := n.Title(); got != "note" {
		t.Errorf("Title() = %q, want the filename stem", got)
	}
}

// TestNonMappingYAMLIsInvalidYAML pins that a frontmatter block holding
// well-formed YAML of the wrong shape — a list, a bare scalar — earns the
// same diagnostic as broken YAML. Frontmatter is a mapping by definition;
// tolerating another shape would leave every field lookup silently empty
// while the note claimed to have frontmatter.
func TestNonMappingYAMLIsInvalidYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "a list is not frontmatter", content: "---\n- a\n- b\n---\nbody\n"},
		{name: "a bare scalar is not frontmatter", content: "---\nhello\n---\nbody\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse("note.md", []byte(tt.content))
			if n.FMDiagnostic == "" {
				t.Error("Parse() diagnostic empty, want the invalid-YAML diagnostic")
			}
			if n.Frontmatter != nil {
				t.Errorf("Parse() frontmatter = %#v, want none", n.Frontmatter)
			}
			if n.Body != "body\n" {
				t.Errorf("Parse() body = %q, want %q", n.Body, "body\n")
			}
		})
	}
}

// TestAnEmptyFrontmatterBlockReadsAsNoFrontmatter pins the current
// equivalence: a note opening with an empty fence pair carries no fields, no
// diagnostic, and answers every frontmatter question exactly as a note with
// no block at all. The one trace the block leaves is the body's file line,
// which still counts the fences.
func TestAnEmptyFrontmatterBlockReadsAsNoFrontmatter(t *testing.T) {
	t.Parallel()

	n := vault.Parse("note.md", []byte("---\n---\nbody\n"))
	if n.Frontmatter != nil || n.FMDiagnostic != "" {
		t.Errorf("Parse() frontmatter = %#v, diagnostic %q, want none of either", n.Frontmatter, n.FMDiagnostic)
	}
	if n.Body != "body\n" {
		t.Errorf("Parse() body = %q, want %q", n.Body, "body\n")
	}
	if n.BodyLine != 3 {
		t.Errorf("Parse() body line = %d, want 3 — the empty block still occupies two file lines", n.BodyLine)
	}
	if got := n.Title(); got != "note" {
		t.Errorf("Title() = %q, want the filename stem", got)
	}
}

// TestTitleStripsOnlyTheFinalExtension pins the fallback title of a note
// with a dotted name: exactly one extension comes off the stem, so a
// language-tagged name keeps its tag.
func TestTitleStripsOnlyTheFinalExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		relPath string
		want    string
	}{
		{relPath: "Writing/note.ja.md", want: "note.ja"},
		{relPath: "Writing/note.md", want: "note"},
		{relPath: "Writing/archive.tar.gz", want: "archive.tar"},
	}
	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse(tt.relPath, []byte("body\n"))
			if got := n.Title(); got != tt.want {
				t.Errorf("Title() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestYAMLDiagnosticNamesFileLinesNotBlockLines holds the frontmatter
// diagnostic to the discipline the body side already obeys: line numbers in
// a fault message count over the file, not the block. The yaml parser
// numbers lines from the first byte it is handed, so the block arrives
// prefixed with the newlines that precede it in the file and the parser
// counts in the file's own geometry — where it places a mark exactly, as it
// does for a decoding fault, the number is the very line an editor shows.
func TestYAMLDiagnosticNamesFileLinesNotBlockLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			// The duplicate sits on file line 4, first defined on file line 3.
			name:    "duplicate key cites both file lines",
			content: "---\ntitle: ok\ndup: 1\ndup: 2\n---\nbody\n",
			want:    `line 4: mapping key "dup" already defined at line 3`,
		},
		{
			// The stray mapping value sits on file line 3.
			name:    "parser fault cites its file line",
			content: "---\na: b\n c: d\n---\nbody\n",
			want:    "line 3: mapping values are not allowed in this context",
		},
		{
			// A byte-order mark and carriage returns do not shift the count.
			name:    "mark and carriage returns leave the count alone",
			content: "\xef\xbb\xbf---\r\ntitle: ok\r\ndup: 1\r\ndup: 2\r\n---\r\nbody\r\n",
			want:    `line 4: mapping key "dup" already defined at line 3`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse("note.md", []byte(tt.content))
			if !strings.Contains(n.FMDiagnostic, tt.want) {
				t.Errorf("Parse() diagnostic = %q, missing %q", n.FMDiagnostic, tt.want)
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

// TestIsMarkdownAcceptsOnlyTheExactLowercaseFinalExtension pins the one test
// every reader splits note from resource on: the path ends in ".md", those
// exact bytes. Any case variant, a missing dot, or a longer final extension
// names a resource.
func TestIsMarkdownAcceptsOnlyTheExactLowercaseFinalExtension(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		{"note.md", true},
		{"Concepts/golang/slices.md", true},
		{"note.ja.md", true},
		{"Note.MD", false},
		{"note.Md", false},
		{"note.mD", false},
		{"note", false},
		{"notemd", false},
		{"note.mdx", false},
		{"note.md.bak", false},
		{"diagram.canvas", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := vault.IsMarkdown(tt.path); got != tt.want {
			t.Errorf("IsMarkdown(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// TestUpdated pins the charitable reading of the declared update date. YAML
// hands an unquoted date over as a time and a quoted one as text; both read,
// as does a full timestamp in either shape. Everything else — absence, prose,
// a shape that is not a scalar — is the zero time, so the caller falls back
// to the file's own recorded change rather than showing a guess.
func TestUpdated(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want time.Time
	}{
		{
			name: "unquoted date",
			raw:  "---\nupdated: 2026-07-12\n---\nbody\n",
			want: time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "quoted date",
			raw:  "---\nupdated: \"2026-07-12\"\n---\nbody\n",
			want: time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "unquoted timestamp keeps its clock",
			raw:  "---\nupdated: 2026-07-12T10:30:00Z\n---\nbody\n",
			want: time.Date(2026, time.July, 12, 10, 30, 0, 0, time.UTC),
		},
		{
			name: "quoted timestamp keeps its clock",
			raw:  "---\nupdated: \"2026-07-12T10:30:00Z\"\n---\nbody\n",
			want: time.Date(2026, time.July, 12, 10, 30, 0, 0, time.UTC),
		},
		{
			name: "absent is the zero time",
			raw:  "---\ntitle: A\n---\nbody\n",
		},
		{
			name: "prose is the zero time",
			raw:  "---\nupdated: last week\n---\nbody\n",
		},
		{
			name: "a list is the zero time",
			raw:  "---\nupdated:\n  - 2026-07-12\n---\nbody\n",
		},
		{
			name: "no frontmatter is the zero time",
			raw:  "body only\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := vault.Parse("A.md", []byte(tt.raw)).Updated()
			if !got.Equal(tt.want) {
				t.Errorf("Updated() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNoteTextAndStrings pins the two frontmatter readers the typed
// accessors are built on: what counts as text, what counts as a list of text,
// and what a shape that is neither costs. A malformed field costs that field
// and never the build, so every row asking for a shape the note did not write
// answers empty rather than an error. The nil-against-empty split in the list
// reader is deliberate and pinned here: nothing at all when the value is not a
// list, an empty list when it is a list holding no text.
func TestNoteTextAndStrings(t *testing.T) {
	t.Parallel()

	const content = `---
title: A note
declared_empty: ""
count: 7
when: 2026-09-02
topics: [alpha, beta]
mixed:
  - alpha
  - 7
  - beta
nested:
  - [alpha]
none: []
---
body
`
	n := vault.Parse("Notes/one.md", []byte(content))

	t.Run("text", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name   string
			key    string
			want   string
			wantOK bool
		}{
			{name: "text answers text", key: "title", want: "A note", wantOK: true},
			{name: "text declared empty is still text", key: "declared_empty", want: "", wantOK: true},
			{name: "a number is not text", key: "count"},
			{name: "a date is not text", key: "when"},
			{name: "a list is not text", key: "topics"},
			{name: "a key the note never wrote is not text", key: "absent"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				got, ok := n.Text(tt.key)
				if got != tt.want || ok != tt.wantOK {
					t.Errorf("Text(%q) = (%q, %t), want (%q, %t)", tt.key, got, ok, tt.want, tt.wantOK)
				}
			})
		}
	})

	t.Run("list of text", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name string
			key  string
			want []string
		}{
			{name: "a list of text answers all of it in order", key: "topics", want: []string{"alpha", "beta"}},
			{name: "a list drops the members that are not text", key: "mixed", want: []string{"alpha", "beta"}},
			{name: "a list of lists holds no text", key: "nested", want: []string{}},
			{name: "an empty list is a list holding no text", key: "none", want: []string{}},
			{name: "text is not a list of text", key: "title", want: nil},
			{name: "a key the note never wrote is not a list of text", key: "absent", want: nil},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				if diff := cmp.Diff(tt.want, n.Strings(tt.key)); diff != "" {
					t.Errorf("Strings(%q) mismatch (-want +got):\n%s", tt.key, diff)
				}
			})
		}
	})
}
