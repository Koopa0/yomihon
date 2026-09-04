package render

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// holdsEverything answers that nothing a note asks for is missing, so the
// rewrite can be exercised without the missing-file mark landing on every row.
// The two are separate questions and these tests keep them apart.
type holdsEverything struct{}

func (holdsEverything) MissingFile(string) bool { return false }

// holdsNothing is the other end: whatever a note reached for, the vault looked
// and has nothing there.
type holdsNothing struct{}

func (holdsNothing) MissingFile(string) bool { return true }

// TestResolveAssetHrefs covers the rewrite and, just as importantly, everything
// it must leave alone. A markdown image is written relative to its own note,
// which the reading route would resolve into another reading page — the reader
// sees a broken image and no diagnostic anywhere says why.
func TestResolveAssetHrefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		note string
		src  string
		want string
	}{
		{
			name: "climbs out of the note directory",
			note: "notes/deep/note.md",
			src:  "../../images/diagram.png",
			want: "/raw/images/diagram.png",
		},
		{
			name: "root-relative names the vault root",
			note: "notes/deep/note.md",
			src:  "/images/diagram.png",
			want: "/raw/images/diagram.png",
		},
		{
			name: "beside the note",
			note: "notes/deep/note.md",
			src:  "./local.png",
			want: "/raw/notes/deep/local.png",
		},
		{
			name: "bare name beside a note at the vault root",
			note: "note.md",
			src:  "cover.png",
			want: "/raw/cover.png",
		},
		{
			name: "a space survives as one path segment",
			note: "notes/note.md",
			src:  "my%20image.png",
			want: "/raw/notes/my%20image.png",
		},
		{
			name: "a fragment stays attached",
			note: "note.md",
			src:  "d.svg#layer",
			want: "/raw/d.svg#layer",
		},
		{
			// Left alone: climbing past the root is not a vault asset, and
			// answering it here would invent a path the routes never serve.
			name: "escaping the vault is untouched",
			note: "note.md",
			src:  "../../../etc/passwd",
			want: "../../../etc/passwd",
		},
		{
			name: "remote is untouched",
			note: "note.md",
			src:  "https://example.com/x.png",
			want: "https://example.com/x.png",
		},
		{
			name: "protocol-relative is untouched",
			note: "note.md",
			src:  "//example.com/x.png",
			want: "//example.com/x.png",
		},
		{
			name: "self-contained data url is untouched",
			note: "note.md",
			src:  "data:image/png;base64,AAAA",
			want: "data:image/png;base64,AAAA",
		},
		{
			name: "an already-routed source is not rewritten twice",
			note: "note.md",
			src:  "/raw/images/diagram.png",
			want: "/raw/images/diagram.png",
		},
		{
			name: "the app's own assets are untouched",
			note: "note.md",
			src:  "/static/yomihon-mark.svg",
			want: "/static/yomihon-mark.svg",
		},
		{
			// The source arrives as attribute text, so the name "a&b.png"
			// reaches here spelled "a&amp;b.png" and has to be read back as
			// the one byte it stands for before it can be resolved.
			name: "an ampersand in the file name resolves to the byte it names",
			note: "notes/note.md",
			src:  "a&amp;b.png",
			want: "/raw/notes/a&amp;b.png",
		},
		{
			name: "a name that reads as a character reference stays a name",
			note: "notes/note.md",
			src:  "a&amp;copy.png",
			want: "/raw/notes/a&amp;copy.png",
		},
		{
			name: "a remote query keeps both of its parameters",
			note: "note.md",
			src:  "https://example.com/x.png?a=1&amp;b=2",
			want: "https://example.com/x.png?a=1&amp;b=2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := `<p><img src="` + tt.src + `" alt="x"></p>`
			var diags []Diagnostic
			got := resolveAssetHrefs(in, tt.note, holdsEverything{}, wording.En, &diags)
			if len(diags) != 0 {
				t.Errorf("a picture the vault holds drew %d diagnostics, want none", len(diags))
			}
			want := `<p><img src="` + tt.want + `" alt="x"></p>`
			if got != want {
				t.Errorf("resolveAssetHrefs(%q, %q) = %q, want %q", tt.src, tt.note, got, want)
			}
		})
	}
}

// TestResolveAssetHrefsLeavesEverythingElseAlone guards the blast radius: this
// runs over rendered HTML, so anything that is not a local image source must
// come through byte-identical.
func TestResolveAssetHrefsLeavesEverythingElseAlone(t *testing.T) {
	t.Parallel()

	const in = `<h2 id="x">標題</h2>` +
		`<p><a href="../other.md">link</a></p>` +
		`<p><code>&lt;img src="a.png"&gt;</code></p>` +
		`<pre class="chroma"><span>img src="b.png"</span></pre>`
	var diags []Diagnostic
	if got := resolveAssetHrefs(in, "notes/note.md", holdsEverything{}, wording.En, &diags); got != in {
		t.Errorf("resolveAssetHrefs rewrote content that is not an image source:\n got %q\nwant %q", got, in)
	}
}

// TestResolveAssetHrefsRewritesEveryImageOnThePage catches a rewrite that stops
// after the first match, which a page with one working and one broken image
// would otherwise hide.
func TestResolveAssetHrefsRewritesEveryImageOnThePage(t *testing.T) {
	t.Parallel()

	in := `<img src="a.png" alt=""><p>text</p><img src="sub/b.png" alt="">`
	var diags []Diagnostic
	got := resolveAssetHrefs(in, "notes/note.md", holdsEverything{}, wording.En, &diags)
	for _, want := range []string{`src="/raw/notes/a.png"`, `src="/raw/notes/sub/b.png"`} {
		if !strings.Contains(got, want) {
			t.Errorf("resolveAssetHrefs output %q is missing %q", got, want)
		}
	}
}

// TestResolveAssetHrefsDoesNotEscapeTheMarkupItRead names the defect directly:
// reading the attribute text as if it were the URL sends the ";" of "&amp;"
// into the path as "%3B", and the browser then asks the raw route for a file
// nobody has. The check is the bytes' absence, because a rewrite that puts them
// back would still route somewhere and still fail only in a browser.
func TestResolveAssetHrefsDoesNotEscapeTheMarkupItRead(t *testing.T) {
	t.Parallel()

	var diags []Diagnostic
	got := resolveAssetHrefs(`<p><img src="a&amp;b.png" alt="x"></p>`, "notes/note.md", holdsEverything{}, wording.En, &diags)
	for _, absent := range []string{"%3B", "&amp;amp;"} {
		if strings.Contains(got, absent) {
			t.Errorf("resolveAssetHrefs re-encoded the markup it read: %q appears in %q", absent, got)
		}
	}
}

// TestAMissingPictureIsMarkedWhereTheAuthorPutIt is the whole point of asking
// the vault what it holds. A note showing a picture the vault does not have
// rewrote to a /raw address that answers 404, and nothing on the page — not the
// image, not the conditions block beside it, not /health — said so, while a
// broken [[wikilink]] two lines above said it in place and in the reader's own
// language. The two are the same absence and the page now describes them the
// same way, with the sentence that already exists for a name with a file's
// extension and nothing behind it.
func TestAMissingPictureIsMarkedWhereTheAuthorPutIt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		note   string
		src    string
		holds  Files
		marked bool
		target string
	}{
		{
			name:   "the vault does not hold it",
			note:   "Notes/note.md",
			src:    "./missing.png",
			holds:  holdsNothing{},
			marked: true,
			target: "Notes/missing.png",
		},
		{
			name:  "the vault holds it",
			note:  "Notes/note.md",
			src:   "./there.png",
			holds: holdsEverything{},
		},
		{
			// An address that is not a vault path was never rewritten either,
			// and this side has nothing to say about what another host holds.
			name:  "an address off the machine",
			note:  "Notes/note.md",
			src:   "https://example.invalid/x.png",
			holds: holdsNothing{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var diags []Diagnostic
			got := resolveAssetHrefs(`<p><img src="`+tt.src+`" alt="x"></p>`, tt.note, tt.holds, wording.En, &diags)
			marked := strings.Contains(got, `class="image-missing"`)
			if marked != tt.marked {
				t.Errorf("marked = %v, want %v; output %q", marked, tt.marked, got)
			}
			if !tt.marked {
				if len(diags) != 0 {
					t.Errorf("drew %d diagnostics, want none: %+v", len(diags), diags)
				}
				return
			}
			if len(diags) != 1 || diags[0].Kind != DiagImageMissing || diags[0].Target != tt.target {
				t.Fatalf("diagnostics = %+v, want one %v naming %q", diags, DiagImageMissing, tt.target)
			}
			// The sentence reaches both a pointer and a voice, which is what
			// the rest of this family promises.
			if !strings.Contains(got, `title="`) || !strings.Contains(got, offscreenNoteClass) {
				t.Errorf("the mark carries no title or no offscreen sentence: %q", got)
			}
			if !strings.Contains(got, `src="/raw/`+tt.target+`"`) {
				t.Errorf("the address the author wrote was not kept: %q", got)
			}
		})
	}
}
