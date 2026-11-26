package note_test

import (
	"log/slog"
	neturl "net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/koopa0/yomihon/internal/note"
	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/schema"
	"github.com/koopa0/yomihon/internal/snapshot"
	"github.com/koopa0/yomihon/internal/vault"
	"github.com/koopa0/yomihon/internal/wording"
)

// pageHTML is the renderer the page draws with, over a one-note vault: what
// Cite is handed so that it asks the product, not a stand-in, what a quotation
// block draws.
func pageHTML(tb testing.TB) func(markdown string) string {
	tb.Helper()
	root := tb.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Notes"), 0o750); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Notes", "Page.md"), []byte("# Page\n\nwords\n"), 0o600); err != nil {
		tb.Fatal(err)
	}
	reader, err := vault.Open(root)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if closeErr := reader.Close(); closeErr != nil {
			tb.Errorf("Reader.Close() error = %v", closeErr)
		}
	})
	store, err := snapshot.New(tb.Context(), reader, slog.New(slog.DiscardHandler), nil, schema.Ungoverned())
	if err != nil {
		tb.Fatal(err)
	}
	snap := store.Current()
	return func(markdown string) string { return snap.Render("Notes/Page.md", markdown, wording.En).HTML }
}

func TestCiteWritesTheOneShape(t *testing.T) {
	t.Parallel()
	html := pageHTML(t)
	for _, tt := range []struct {
		name string
		in   note.Citation
		lang wording.Lang
		want string
	}{
		{
			name: "a passage of a section, and nothing the reader wrote",
			in:   note.Citation{Title: "Pipeline", Path: "Notes/go/Pipeline.md", Section: "Two signals", Quote: "cancel() does not wait for the work to end."},
			lang: wording.En,
			want: "> cancel() does not wait for the work to end.\n\n— [Pipeline](Notes/go/Pipeline.md), Two signals",
		},
		{
			name: "a whole section is its source line alone",
			in:   note.Citation{Title: "Pipeline", Path: "Notes/go/Pipeline.md", Section: "Two signals"},
			lang: wording.En,
			want: "— [Pipeline](Notes/go/Pipeline.md), Two signals",
		},
		{
			name: "before the first heading there is no section to name",
			in:   note.Citation{Title: "Pipeline", Path: "Notes/go/Pipeline.md", Quote: "Intro words."},
			lang: wording.En,
			want: "> Intro words.\n\n— [Pipeline](Notes/go/Pipeline.md)",
		},
		{
			name: "the reader's words follow after a blank line",
			in:   note.Citation{Title: "Pipeline", Path: "Notes/go/Pipeline.md", Section: "Two signals", Quote: "Intro words.", Words: "\n  my first line\nmy second\n\n"},
			lang: wording.En,
			want: "> Intro words.\n\n— [Pipeline](Notes/go/Pipeline.md), Two signals\n\n  my first line\nmy second",
		},
		{
			name: "a path with a space and parentheses is written between angle brackets",
			in:   note.Citation{Title: "Spaced", Path: "Notes/My notes (draft)/A b.md", Quote: "x"},
			lang: wording.En,
			want: "> x\n\n— [Spaced](<Notes/My notes (draft)/A b.md>)",
		},
		{
			name: "the section is joined as the language writes a comma",
			in:   note.Citation{Title: "可以取消的管線", Path: "Notes/go/可以取消的管線.md", Section: "兩個訊號，各管一件事", Quote: "cancel() 本身不等待工作結束。"},
			lang: wording.ZhHant,
			want: "> cancel() 本身不等待工作結束。\n\n— [可以取消的管線](Notes/go/可以取消的管線.md)，兩個訊號，各管一件事",
		},
		{
			name: "a note with no title is linked by its file name",
			in:   note.Citation{Path: "Notes/go/pipeline-notes.md", Quote: "x"},
			lang: wording.En,
			want: "> x\n\n— [pipeline-notes](Notes/go/pipeline-notes.md)",
		},
		{
			name: "marks in a title and a section are written so that they stay words",
			in:   note.Citation{Title: "A [b] *c* `d` <e> &f\nand g", Path: "N.md", Section: "x_y_z", Quote: "q"},
			lang: wording.En,
			want: "> q\n\n— [A \\[b\\] \\*c\\* \\`d\\` \\<e\\> \\&f and g](N.md), x\\_y\\_z",
		},
		{
			name: "a backslash ends no link, and an unbalanced parenthesis ends no destination",
			in:   note.Citation{Title: `back\`, Path: "Notes/a(b.md", Section: `c\`, Quote: "x"},
			lang: wording.En,
			want: "> x\n\n— [back\\\\](<Notes/a(b.md>), c\\\\",
		},
		{
			name: "a quote that wraps over lines, or ends them with a carriage return, is one block",
			in:   note.Citation{Title: "T", Path: "N.md", Quote: "first line\r\nsecond line\rthird line"},
			lang: wording.En,
			want: "> first line\n> second line\n> third line\n\n— [T](N.md)",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, note.Cite(&tt.in, tt.lang, html)); diff != "" {
				t.Errorf("Cite() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// A quote is the words the reader saw. Where the page would draw a quotation
// block of them as something else, they are put in a fence, which draws what
// is in it.
func TestCiteFencesWhatAQuotationBlockWouldDrawAsSomethingElse(t *testing.T) {
	t.Parallel()
	html := pageHTML(t)
	for _, tt := range []struct {
		name  string
		quote string
		code  bool
		lang  string
		want  string
	}{
		{name: "a numbered line", quote: "1. first\n2. second", want: "> ```\n> 1. first\n> 2. second\n> ```"},
		{name: "a line that reads as a heading", quote: "# not a heading", want: "> ```\n> # not a heading\n> ```"},
		{name: "asterisks", quote: "a *b* c", want: "> ```\n> a *b* c\n> ```"},
		{name: "a callout marker", quote: "[!note] words", want: "> ```\n> [!note] words\n> ```"},
		{name: "raw HTML a consumer that renders HTML would draw", quote: "<img src=x onerror=1>", want: "> ```\n> <img src=x onerror=1>\n> ```"},
		{name: "a link written as HTML", quote: "<a href=\"http://evil.example\">go</a>", want: "> ```\n> <a href=\"http://evil.example\">go</a>\n> ```"},
		{name: "an HTML comment", quote: "before <!-- after", want: "> ```\n> before <!-- after\n> ```"},
		{name: "a reference label a reader's words may define", quote: "see [a] now", want: "> ```\n> see [a] now\n> ```"},
		{name: "an opening bracket alone", quote: "a [b c", want: "> ```\n> a [b c\n> ```"},
		{name: "an ampersand alone", quote: "AT&T", want: "> ```\n> AT&T\n> ```"},
		{name: "a closing bracket alone", quote: "a] b", want: "> ```\n> a] b\n> ```"},
		{name: "a character reference", quote: "AT&amp;T", want: "> ```\n> AT&amp;T\n> ```"},
		{name: "a block of code, whatever it holds", quote: "x := 1", code: true, lang: "go", want: "> ```go\n> x := 1\n> ```"},
		{name: "a fence is longer than any run of backticks in the quote", quote: "a ````` b\n```", code: true, want: "> ``````\n> a ````` b\n> ```\n> ``````"},
		{name: "a language that is not one word is not written", quote: "x", code: true, lang: "go`\n# [x](y)", want: "> ```\n> x\n> ```"},
		{name: "a language of a language's own characters", quote: "x", code: true, lang: "c++", want: "> ```c++\n> x\n> ```"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := note.Citation{Title: "T", Path: "N.md", Quote: tt.quote, Code: tt.code, Language: tt.lang}
			if diff := cmp.Diff(tt.want+"\n\n— [T](N.md)", note.Cite(&in, wording.En, html)); diff != "" {
				t.Errorf("Cite() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// A path is a place in the vault, and the link to it is never a link to
// anywhere else: a path another computer wrote may start with something a
// Markdown reader takes for a scheme or a host.
func TestCiteWritesAPathAsAPlaceInTheVaultAndNoOther(t *testing.T) {
	t.Parallel()
	html := pageHTML(t)
	for _, tt := range []struct{ name, path, want string }{
		{"a scheme", "javascript:alert(1).md", "<./javascript:alert(1).md>"},
		{"a scheme with one slash", "http:/x/y.md", "./http:/x/y.md"},
		{"a colon in the first name", "a:b/c.md", "./a:b/c.md"},
		{"a colon after the first name is no scheme", "Notes/a:b.md", "Notes/a:b.md"},
		{"a host", "//evil.example/x.md", ".///evil.example/x.md"},
		{"the root", "/etc/x.md", ".//etc/x.md"},
		{"a backslash a browser reads as a slash", "\\\\evil.example\\x.md", "<./\\\\\\\\evil.example\\\\x.md>"},
		{"a number sign is part of the name, not a fragment", "Notes/C#.md", "Notes/C%23.md"},
		{"a question mark is part of the name, not a query", "Notes/Why?.md", "Notes/Why%3F.md"},
		{"a percent sign is itself and not the start of an escape", "Notes/100%.md", "Notes/100%25.md"},
		{"a name that looks like an escape is not one", "Notes/%41.md", "Notes/%2541.md"},
		{"a path inside the vault is left as it is", "Notes/go/Pipeline.md", "Notes/go/Pipeline.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := note.Citation{Title: "T", Path: tt.path}
			if diff := cmp.Diff("— [T]("+tt.want+")", note.Cite(&in, wording.En, html)); diff != "" {
				t.Errorf("Cite() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// What a quote draws is asked of the page's own renderer, which reads more than
// CommonMark does: a wikilink, a highlight, a block address, a comment.
func TestCiteAsksThePagesRendererAndNotAnotherOne(t *testing.T) {
	t.Parallel()
	html := pageHTML(t)
	for _, quote := range []string{"see [[Page]] now", "a ==marked== word", "text ^block-id", "kept %%hidden%% kept", "![[Page]]"} {
		in := note.Citation{Title: "T", Path: "N.md", Quote: quote}
		got := note.Cite(&in, wording.En, html)
		if !strings.HasPrefix(got, "> ```\n") {
			t.Errorf("Cite(%q) = %q, want the quote fenced: the page draws something other than those words", quote, got)
		}
	}
}

// A renderer that draws a block as written leaves the quote in it, and one that
// does not leaves nothing to decide the fence by, so there must be one.
func TestCiteNeedsARenderer(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("Cite(nil renderer) did not panic")
		}
	}()
	note.Cite(&note.Citation{Path: "N.md", Quote: "x"}, wording.En, nil)
}

// parsed is the Markdown as CommonMark with the GFM extensions reads it.
func parsedCitation(out string) (doc ast.Node, src []byte) {
	src = []byte(out)
	return goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser().Parse(text.NewReader(src)), src
}

func blocksOf(n ast.Node) []ast.Node {
	var out []ast.Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		out = append(out, c)
	}
	return out
}

func linkText(n ast.Node, src []byte) string {
	var words []byte
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the callback returns no error
		if t, ok := c.(*ast.Text); ok && entering {
			words = append(words, util.UnescapePunctuations(t.Segment.Value(src))...)
		}
		return ast.WalkContinue, nil
	})
	return string(words)
}

// FuzzCite holds the one thing that must hold of whatever a vault and a reader
// can put into a mark: the citation is one quotation block, if there is a
// quote, and one line that is one link. Nothing in the quote, the title, the
// path or the section ends the block, begins another or adds a link. The
// reader's words are Markdown of their own, written after a blank line, so
// they are checked for what they must be and not for what they say.
func FuzzCite(f *testing.F) {
	type seed struct {
		quote, title, path, section, words string
		code                               bool
		language                           string
	}
	for _, s := range []seed{
		{"cancel() does not wait", "Pipeline", "Notes/go/Pipeline.md", "Two signals", "", false, ""},
		{"```\n— [x](y)\n```", "T", "N.md", "S", "", true, "go"},
		{"````\n```\n# h", "T", "N.md", "S", "", false, ""},
		{"a\rb\r\n```\r\nc", "T", "N.md", "", "words", true, "```"},
		{"~~~\n> x\n---\n", "T", "N.md", "", "", true, ""},
		{"line\n\n> nested\n\n- list\n1. one", "]( <a> )[", "a b/(c)/<d>.md", "`code` *s*", "", false, ""},
		{"x", "T\n\n# h\n\n```", "N\n.md", "S\r\n---", "", false, ""},
		{"x", "T", "\\<n>&amp;.md", "&lt;", "", false, ""},
		{"x", "back\\", "a(b.md", "c\\", "", false, "go"},
		{"x", "T", "a)b.md", "S", "", false, "x y"},
		{"\x00\x1b[31m" + string(rune(0x2028)) + string(rune(0x202e)) + " x", "\x00", "\x1b.md", string(rune(0x2029)), "", false, "go c"},
		{"   \n \t\n", "T", "N.md", "S", "w", false, ""},
		{"x", "T", "javascript:alert(1).md", "S", "", false, ""},
		{"x", "T", "http:/x/y.md", "S", "", false, ""},
		{"x", "T", "//evil.example/x.md", "S", "", false, ""},
		{"x", "T", "Notes/C#.md", "S", "", false, ""},
		{"x", "T", "Notes/Why?.md", "S", "", false, ""},
		{"x", "T", "Notes/100%.md", "S", "", false, ""},
		{"x", "T", "Notes/%41#a?b.md", "S", "", false, ""},
		{"x", "T", "\\\\evil.example/x.md", "S", "", false, ""},
		{"<img src=x onerror=1> <!-- [a] &#60;", "T", "N.md", "S", "", false, ""},
	} {
		f.Add(s.quote, s.title, s.path, s.section, s.words, s.code, s.language)
	}
	html := pageHTML(f)
	f.Fuzz(func(t *testing.T, quote, title, path, section, words string, code bool, language string) {
		for _, field := range []string{quote, title, path, section, words, language} {
			if !utf8.ValidString(field) {
				// A mark is held to UTF-8 on the way in and on the way out, and a
				// title or a path the vault holds is read as UTF-8.
				t.Skip("not UTF-8")
			}
		}
		in := note.Citation{Title: title, Path: path, Section: section, Quote: quote, Code: code, Language: language}
		out := note.Cite(&in, wording.En, html)
		if strings.ContainsRune(out, '\r') {
			t.Fatalf("Cite(%q) = %q holds a carriage return, which Markdown reads as a line end", quote, out)
		}
		hasQuote := strings.TrimSpace(quote) != ""
		doc, src := parsedCitation(out)
		top := blocksOf(doc)
		want := 1
		if hasQuote {
			want = 2
		}
		if len(top) != want {
			t.Fatalf("Cite(%q, title %q, path %q, section %q) = %q is %d blocks, want %d", quote, title, path, section, out, len(top), want)
		}
		if hasQuote {
			checkQuotation(t, top[0], src, &in, out, html)
		}
		para, ok := top[want-1].(*ast.Paragraph)
		if !ok {
			t.Fatalf("Cite() = %q does not end in the source line", out)
		}
		var links []*ast.Link
		_ = ast.Walk(para, func(n ast.Node, entering bool) (ast.WalkStatus, error) { //nolint:errcheck // the callback returns no error
			if l, ok := n.(*ast.Link); ok && entering {
				links = append(links, l)
			}
			return ast.WalkContinue, nil
		})
		if len(links) != 1 {
			t.Fatalf("Cite(title %q, path %q, section %q) = %q has %d links in the source line, want 1", title, path, section, out, len(links))
		}
		if got, want := string(util.UnescapePunctuations(links[0].Destination)), inVault(controlsEscaped(path)); got != want {
			t.Fatalf("Cite(path %q) = %q links to %q, want %q", path, out, got, want)
		}
		// Read as a reference, the destination names the path of the note, whole:
		// no part of the name is a query, a fragment or an escape.
		if u, err := neturl.Parse(string(util.UnescapePunctuations(links[0].Destination))); err != nil {
			t.Fatalf("Cite(path %q) = %q links to what does not parse as a reference: %v", path, out, err)
		} else if u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || u.Path != inVault(path) {
			t.Fatalf("Cite(path %q) = %q links to the path %q with query %q and fragment %q, want the path alone", path, out, u.Path, u.RawQuery, u.Fragment)
		}
		if dest := string(util.UnescapePunctuations(links[0].Destination)); linksElsewhere(dest) {
			t.Fatalf("Cite(path %q) = %q links to %q, which a Markdown reader takes for another place than the vault", path, out, dest)
		}
		if got := linkText(links[0], src); got != wantTitle(title, path) {
			t.Fatalf("Cite(title %q, path %q) = %q reads the link as %q, want %q", title, path, out, got, wantTitle(title, path))
		}

		// The reader's words come after, and change nothing before them.
		withWords := note.Cite(&note.Citation{Title: title, Path: path, Section: section, Quote: quote, Code: code, Language: language, Words: words}, wording.En, html)
		if !strings.HasPrefix(withWords, out) {
			t.Fatalf("Cite(words %q) = %q does not begin with the citation without them, %q", words, withWords, out)
		}
	})
}

// checkQuotation holds the quotation block of a citation to its one shape: a
// block, with either a fence that holds the quote's lines and nothing else or
// the words the page draws.
func checkQuotation(t *testing.T, top ast.Node, src []byte, in *note.Citation, out string, html func(string) string) {
	t.Helper()
	quote := in.Quote
	block, ok := top.(*ast.Blockquote)
	if !ok {
		t.Fatalf("Cite(%q) = %q does not open with a quotation block", quote, out)
	}
	var fence *ast.FencedCodeBlock
	if inside := blocksOf(block); len(inside) == 1 {
		if single, isFence := inside[0].(*ast.FencedCodeBlock); isFence {
			fence = single
		}
	}
	switch {
	case fence != nil:
		var got []byte
		lines := fence.Lines()
		for i := range lines.Len() {
			seg := lines.At(i)
			got = append(got, seg.Value(src)...)
		}
		if wantLines := strings.Join(trimmedLines(quote), "\n") + "\n"; string(got) != wantLines {
			t.Fatalf("Cite(%q) = %q fences %q, want the quote's own lines %q", quote, out, got, wantLines)
		}
	case in.Code:
		t.Fatalf("Cite(%q, code) = %q is not a fence", quote, out)
	default:
		// Not fenced, so the page draws these words and no others, and nothing
		// in them is read as HTML, a reference or a character reference by a
		// consumer that reads more than the page does.
		if strings.ContainsAny(quote, "<[]&") {
			t.Fatalf("Cite(%q) = %q is not fenced and holds markup a consumer may read", quote, out)
		}
		drawn := render.TextContent(html(out[:strings.LastIndex(out, "\n\n— [")]))
		if noSpace(drawn) != noSpace(strings.Join(trimmedLines(quote), " ")) {
			t.Fatalf("Cite(%q) = %q is not fenced and the page draws %q", quote, out, drawn)
		}
	}
}

func noSpace(words string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, words)
}

// trimmedLines is the lines of a quote that Cite writes: line ends of every
// kind are one, and the blank lines at either end are not part of it.
func trimmedLines(quote string) []string {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(quote, "\r\n", "\n"), "\r", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// controlsEscaped is a path as a link holds it: a control character and the
// three that would start a query, a fragment or an escape are percent escapes,
// and nothing else is changed.
func controlsEscaped(path string) string {
	var b strings.Builder
	for _, r := range path {
		if unicode.IsControl(r) || r == '#' || r == '?' || r == '%' {
			for _, by := range []byte(string(r)) {
				b.WriteString("%" + strings.ToUpper(string("0123456789abcdef"[by>>4])+string("0123456789abcdef"[by&15])))
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// inVault is a link destination as the vault's own: one that starts with
// something a reader takes for a scheme, a host or the root gets "./".
func inVault(dest string) string {
	if linksElsewhere(dest) {
		return "./" + dest
	}
	return dest
}

// linksElsewhere says a destination, read as a relative reference, is not one:
// it has a scheme, starts at a host or at the root, or starts with a backslash,
// which a browser reads as a slash.
func linksElsewhere(dest string) bool {
	if strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "\\") {
		return true
	}
	first, _, _ := strings.Cut(dest, "/")
	if strings.Contains(first, ":") {
		return true
	}
	if u, err := neturl.Parse(dest); err == nil && (u.Scheme != "" || u.Host != "") {
		return true
	}
	return false
}

// wantTitle is the words of the link: the title on one line, or the file's name.
func wantTitle(title, rel string) string {
	oneLine := func(text string) string {
		return strings.Join(strings.FieldsFunc(text, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
	}
	if one := oneLine(title); one != "" {
		return one
	}
	return oneLine(strings.TrimSuffix(pathpkg.Base(rel), pathpkg.Ext(rel)))
}
