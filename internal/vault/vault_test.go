package vault_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/koopa0/yomihon/internal/vault"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		wantType   string
		wantDraft  any
		wantSlug   string
		wantDiag   bool
		wantInBody string
	}{
		{
			name:       "lesson slug is read as the join key",
			content:    "---\ntype: lesson\ndraft: true\nslug: jp-minna-l01\n---\n\nbody\n",
			wantType:   "lesson",
			wantDraft:  true,
			wantSlug:   "jp-minna-l01",
			wantInBody: "body",
		},
		{
			name:       "frontmatter and body",
			content:    "---\ntype: concept\ndraft: false\n---\n\n# 本文\n",
			wantType:   "concept",
			wantDraft:  false,
			wantInBody: "# 本文",
		},
		{
			name:       "no frontmatter is legal",
			content:    "# 假名 quiz\n",
			wantInBody: "# 假名 quiz",
		},
		{
			name:       "broken yaml yields diagnostic not error",
			content:    "---\ntype: [broken\n---\nbody\n",
			wantDiag:   true,
			wantInBody: "body",
		},
		{
			name:       "wikilink value survives the split",
			content:    "---\ntype: t\nbased_on: \"[[大家的日本語 第11課]]\"\n---\nbody\n",
			wantType:   "t",
			wantInBody: "body",
		},
		{
			// The concept-resolver corruption guard, at the layer where the
			// guarantee lives: a lesson's frontmatter based_on holds
			// [[wikilinks]]; because the split happens here, before any body
			// preprocessing, a later concept resolver rewriting [[...]] to a
			// trigger can never reach the YAML and empty the meta (which would
			// drop the draft flag and the slug and hide the lesson). Both must
			// survive intact alongside the wikilink-valued field.
			name:       "concept resolver cannot corrupt frontmatter fields",
			content:    "---\ntype: lesson\ndraft: false\nbased_on: \"[[大家的日本語 第1課]]\"\nslug: jp-minna-l00\n---\n\nSee [[は]] and [[です]].\n",
			wantType:   "lesson",
			wantDraft:  false,
			wantSlug:   "jp-minna-l00",
			wantInBody: "[[は]]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse("Concepts/note.md", []byte(tt.content))
			if got := n.Type(); got != tt.wantType {
				t.Errorf("Type() = %q, want %q", got, tt.wantType)
			}
			if got := n.Frontmatter["draft"]; got != tt.wantDraft {
				t.Errorf("Frontmatter[draft] = %#v, want %#v", got, tt.wantDraft)
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
		name     string
		content  string
		wantType string
		wantLang string
		wantBody string
	}{
		{name: "LF dash closer", content: "---\ntype: LF\nlang: ja\n---\nbody\n", wantType: "LF", wantLang: "ja", wantBody: "body\n"},
		{name: "CRLF dash closer", content: "---\r\ntype: CRLF\r\nlang: en\r\n---\r\nbody\r\n", wantType: "CRLF", wantLang: "en", wantBody: "body\r\n"},
		{name: "YAML document closer", content: "---\ntype: Dots\nlang: ja\n...\nbody\n", wantType: "Dots", wantLang: "ja", wantBody: "body\n"},
		{name: "closer at EOF", content: "---\ntype: EOF\nlang: en\n---", wantType: "EOF", wantLang: "en", wantBody: ""},
		{name: "unterminated is body", content: "---\ntype: Open\nlang: ja\nbody\n", wantType: "", wantLang: "", wantBody: "---\ntype: Open\nlang: ja\nbody\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse("note.md", []byte(tt.content))
			if got := n.Type(); got != tt.wantType {
				t.Errorf("Type() = %q, want %q", got, tt.wantType)
			}
			if got, _ := n.Text("lang"); got != tt.wantLang {
				t.Errorf("Text(lang) = %q, want %q", got, tt.wantLang)
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
	if n.HasFrontmatter || n.Frontmatter != nil || n.FMDiagnostic != "" {
		t.Errorf("Parse() HasFrontmatter = %v frontmatter = %#v, diagnostic %q, want none of either", n.HasFrontmatter, n.Frontmatter, n.FMDiagnostic)
	}
	if n.Body != content {
		t.Errorf("Parse() body = %q, want the whole file", n.Body)
	}
	if n.BodyLine != 1 {
		t.Errorf("Parse() body line = %d, want 1", n.BodyLine)
	}
	if got := n.Type(); got != "" {
		t.Errorf("Type() = %q, want none", got)
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

// parseWithoutPanic runs vault.Parse and turns a panic into a failure of the
// calling subtest, so a parser that dies reports which input killed it instead
// of aborting every test beside it.
func parseWithoutPanic(t *testing.T, content string) *vault.Note {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Parse(%q) panicked: %v", content, r)
		}
	}()
	return vault.Parse("note.md", []byte(content))
}

// TestAFrontmatterMergeKeyNeverPanicsTheParser pins that a merge key (<<) in
// frontmatter costs the note, never the process. Parse reads whatever bytes a
// vault holds, on every scan and every save, and a panic there ends the
// server: a YAML merge beside a key that is a list or a mapping used to reach
// an unhashable map key inside the decoder. Such a note is reported as invalid
// YAML, and the merge forms that are valid keep decoding as they always did,
// so the guard cannot be met by refusing merge keys altogether.
func TestAFrontmatterMergeKeyNeverPanicsTheParser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		wantDiag   bool
		wantFields map[string]any
	}{
		{
			name:     "a merge key beside a list key",
			content:  "---\n<<: {a: 1}\n? [1, 2]\n: 3\n---\n# hi\n",
			wantDiag: true,
		},
		{
			name:     "a merge key after a mapping key",
			content:  "---\n? {x: 1}\n: 3\n<<: {a: 1}\n---\n# hi\n",
			wantDiag: true,
		},
		{
			name:     "a merged alias beside a list key",
			content:  "---\nbase: &base {a: 1}\n<<: *base\n? [1, 2]\n: 3\n---\n# hi\n",
			wantDiag: true,
		},
		{
			name:     "a merged list of mappings beside a list key",
			content:  "---\n<<: [{a: 1}, {b: 2}]\n? [1, 2]\n: 3\n---\n# hi\n",
			wantDiag: true,
		},
		{
			name:     "a merge key holding a scalar beside a list key",
			content:  "---\n<<: 1\n? [1, 2]\n: 3\n---\n# hi\n",
			wantDiag: true,
		},
		{
			name:     "a merge key holding a scalar",
			content:  "---\n<<: 1\nb: 2\n---\n# hi\n",
			wantDiag: true,
		},
		{
			name:     "a merged list holding a scalar",
			content:  "---\n<<: [1]\nb: 2\n---\n# hi\n",
			wantDiag: true,
		},
		{
			name:       "a merged mapping",
			content:    "---\n<<: {a: 1}\nb: 2\n---\n# hi\n",
			wantFields: map[string]any{"a": 1, "b": 2},
		},
		{
			name:       "a merged alias",
			content:    "---\nbase: &base {a: 1}\n<<: *base\nb: 2\n---\n# hi\n",
			wantFields: map[string]any{"base": map[string]any{"a": 1}, "a": 1, "b": 2},
		},
		{
			name:       "a merged list of mappings",
			content:    "---\n<<: [{a: 1}, {c: 3}]\nb: 2\n---\n# hi\n",
			wantFields: map[string]any{"a": 1, "b": 2, "c": 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := parseWithoutPanic(t, tt.content)
			if !n.HasFrontmatter {
				t.Error("Parse() HasFrontmatter = false, want the block counted as present")
			}
			if n.Body != "# hi\n" {
				t.Errorf("Parse() body = %q, want %q", n.Body, "# hi\n")
			}
			if gotDiag := n.FMDiagnostic != ""; gotDiag != tt.wantDiag {
				t.Errorf("Parse() diagnostic = %q, want a diagnostic: %v", n.FMDiagnostic, tt.wantDiag)
			}
			if diff := cmp.Diff(tt.wantFields, n.Frontmatter); diff != "" {
				t.Errorf("Parse() frontmatter mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestAnEmptyFrontmatterBlockIsPresentWithoutFields pins the two facts an
// empty fence pair leaves: the block is there, and it decoded to no fields.
// Field lookups still match a file with no block; HasFrontmatter does not.
func TestAnEmptyFrontmatterBlockIsPresentWithoutFields(t *testing.T) {
	t.Parallel()

	n := vault.Parse("note.md", []byte("---\n---\nbody\n"))
	if !n.HasFrontmatter {
		t.Error("Parse() HasFrontmatter = false, want the empty block counted as present")
	}
	if n.Frontmatter != nil || n.FMDiagnostic != "" {
		t.Errorf("Parse() frontmatter = %#v, diagnostic %q, want none of either", n.Frontmatter, n.FMDiagnostic)
	}
	if n.Body != "body\n" {
		t.Errorf("Parse() body = %q, want %q", n.Body, "body\n")
	}
	if n.BodyLine != 3 {
		t.Errorf("Parse() body line = %d, want 3 — the empty block still occupies two file lines", n.BodyLine)
	}
	if got := n.Type(); got != "" {
		t.Errorf("Type() = %q, want none", got)
	}
}

// TestParseReportsWhetherAFrontmatterBlockWasThere holds block presence to the
// split, not to whether YAML produced a map. An empty fence pair and a
// malformed block are present; a file with no delimiters is not.
func TestParseReportsWhetherAFrontmatterBlockWasThere(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "no delimiters", data: "# body\n", want: false},
		{name: "empty fence pair", data: "---\n---\nbody\n", want: true},
		{name: "fields", data: "---\ntitle: Note\n---\nbody\n", want: true},
		{name: "malformed YAML", data: "---\ntitle: [broken\n---\nbody\n", want: true},
		{name: "unterminated is body", data: "---\ntitle: Open\nbody\n", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			n := vault.Parse("note.md", []byte(tt.data))
			if n.HasFrontmatter != tt.want {
				t.Errorf("Parse(%q) HasFrontmatter = %v, want %v", tt.data, n.HasFrontmatter, tt.want)
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
// mark does not move the offsets measured against the original bytes.
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
			name: "mark with no fence precedes the body", data: mark + "just body\n",
			wantFound: false, wantContent: "", wantBody: "just body\n",
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
			// were handed in, not in a trimmed copy of them.
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

// TestCreated pins the charitable reading of the declared creation date. YAML
// hands an unquoted date over as a time and a quoted one as text; both read,
// as does a full timestamp in either shape. Everything else — absence, prose,
// a shape that is not a scalar, a date written under another key — is the zero
// time, so no caller shows a guess.
func TestCreated(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  string
		want time.Time
	}{
		{
			name: "unquoted date",
			raw:  "---\ncreated: 2026-07-12\n---\nbody\n",
			want: time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "quoted date",
			raw:  "---\ncreated: \"2026-07-12\"\n---\nbody\n",
			want: time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "unquoted timestamp keeps its clock",
			raw:  "---\ncreated: 2026-07-12T10:30:00Z\n---\nbody\n",
			want: time.Date(2026, time.July, 12, 10, 30, 0, 0, time.UTC),
		},
		{
			name: "quoted timestamp keeps its clock",
			raw:  "---\ncreated: \"2026-07-12T10:30:00Z\"\n---\nbody\n",
			want: time.Date(2026, time.July, 12, 10, 30, 0, 0, time.UTC),
		},
		{
			name: "absent is the zero time",
			raw:  "---\ntype: concept\n---\nbody\n",
		},
		{
			name: "an update date is not a creation date",
			raw:  "---\nupdated: 2026-07-12\n---\nbody\n",
		},
		{
			name: "prose is the zero time",
			raw:  "---\ncreated: last week\n---\nbody\n",
		},
		{
			name: "a number is the zero time",
			raw:  "---\ncreated: 20260712\n---\nbody\n",
		},
		{
			name: "a list is the zero time",
			raw:  "---\ncreated:\n  - 2026-07-12\n---\nbody\n",
		},
		{
			name: "no frontmatter is the zero time",
			raw:  "body only\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := vault.Parse("A.md", []byte(tt.raw)).Created()
			if !got.Equal(tt.want) {
				t.Errorf("Created() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestVerifiedOnIsTheDayTheAuthorWroteAndNothingElseStandsInForIt holds the one
// date the reading page shows as the note's: read as created is, from an
// unquoted or a quoted day, and the zero time for anything else — a creation
// date, an update date, prose, or a list do not stand in for it.
func TestVerifiedOnIsTheDayTheAuthorWroteAndNothingElseStandsInForIt(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		raw  string
		want time.Time
	}{
		{name: "an unquoted day", raw: "---\nverified_on: 2026-07-20\n---\nbody\n", want: day},
		{name: "a quoted day", raw: "---\nverified_on: \"2026-07-20\"\n---\nbody\n", want: day},
		{name: "a creation date is not a checked day", raw: "---\ncreated: 2026-07-20\n---\nbody\n"},
		{name: "an update date is not a checked day", raw: "---\nupdated: 2026-07-20\n---\nbody\n"},
		{name: "prose is the zero time", raw: "---\nverified_on: last week\n---\nbody\n"},
		{name: "a list is the zero time", raw: "---\nverified_on:\n  - 2026-07-20\n---\nbody\n"},
		{name: "no frontmatter is the zero time", raw: "body only\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := vault.Parse("A.md", []byte(tt.raw)).VerifiedOn()
			if !got.Equal(tt.want) {
				t.Errorf("VerifiedOn() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseLeavesTheDomainEmpty pins that a domain is never read off the note:
// a frontmatter domain key and a folder that looks like a domain both leave it
// empty, because only a reader holding the contract knows which folders name
// domains.
func TestParseLeavesTheDomainEmpty(t *testing.T) {
	t.Parallel()

	n := vault.Parse("Concepts/golang/slices.md", []byte("---\ntype: concept\ndomain: golang\n---\n# Slices\n"))
	if n.Domain != "" {
		t.Errorf("Parse().Domain = %q, want empty", n.Domain)
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
