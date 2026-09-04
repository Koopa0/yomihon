package render

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/graph"
	"github.com/koopa0/yomihon/internal/wording"
)

// noBodies and anyTitle stand in for the two collaborators a pipeline requires
// but the callout tests never exercise.
type noBodies struct{}

func (noBodies) Transclusion(string) (string, bool) { return "", false }

type anyTitle struct{}

func (anyTitle) TitledBy(string) []string { return nil }

// TestACalloutBodyClosesEveryBlockAnEmptyLineDoesNotEnd enumerates the ways a
// callout's body can end in the middle of something. A callout is read as part
// of the note it is written in, so its body's source is laid into the note's
// own; a block the body opened and never closed would go on reading whatever
// follows it as its own content — the rest of the note, and the markup this
// renderer writes to close the callout with it. What reaches the reader then is
// the two failures this whole area exists to forbid: the note's remaining
// prose as program text with its footnote references spelled out as "[^h]",
// and a "<!--yomihon-block:1-->" of the renderer's own on the page, with the
// callout's elements never closed.
//
// The set is CommonMark's: a fenced code block, and the HTML blocks whose end
// is a particular string rather than an empty line. The other two HTML blocks
// are ended by an empty line, and one stands after every callout body. Each
// case is written twice, once at the body's own level and once inside a list
// item, because a close has to be written from inside the container that opened
// it — at the margin it ends the list instead and opens a block of its own.
func TestACalloutBodyClosesEveryBlockAnEmptyLineDoesNotEnd(t *testing.T) {
	t.Parallel()
	r := New(graph.BuildFromNotes(nil, nil), noBodies{}, anyTitle{}, holdsEverything{})

	cases := []struct {
		name   string
		opener string
		// ends names the entry of htmlBlockKinds a case stands for, so the
		// completeness check below can see which are spoken for. A fence has no
		// entry there and leaves it empty.
		ends string
	}{
		{"a backtick fence", "```", ""},
		{"a longer backtick fence", "````", ""},
		{"a tilde fence", "~~~", ""},
		{"a longer tilde fence", "~~~~", ""},
		{"a fence carrying an info string", "```go", ""},
		{"a pre element", "<pre>", "</pre>"},
		{"a script element", "<script>", "</script>"},
		{"a style element", "<style>", "</style>"},
		{"a textarea element", "<textarea>", "</textarea>"},
		{"a comment", "<!-- unfinished", "-->"},
		{"a processing instruction", "<?php", "?>"},
		{"a character-data section", "<![CDATA[", "]]>"},
		{"a declaration", "<!DOCTYPE html", ">"},
	}

	containers := []struct {
		name  string
		lines func(opener string) []string
	}{
		{"at the callout body's own level", func(opener string) []string {
			return []string{"> [!note] Aside", "> " + opener, "> swallowed"}
		}},
		{"inside a list item in the callout", func(opener string) []string {
			return []string{"> [!note] Aside", "> - item", ">   " + opener, ">   swallowed"}
		}},
	}

	for _, tc := range cases {
		for _, container := range containers {
			t.Run(tc.name+" "+container.name, func(t *testing.T) {
				t.Parallel()
				body := strings.Join(append(container.lines(tc.opener),
					"", "Host paragraph after.[^h]", "", "[^h]: The host's definition."), "\n")
				got := r.HTML("Notes/Runaway.md", "", body, wording.ZhHant).HTML

				if strings.Contains(got, "yomihon-block") {
					t.Errorf("a marker the renderer plants for its own markup reached the page as text:\n%s", got)
				}
				if strings.Contains(got, "[^") {
					t.Errorf("footnote syntax reached the page as text, so the note after the callout was read as content of the block inside it:\n%s", got)
				}
				host := strings.Index(got, "<p>Host paragraph after.")
				if host < 0 {
					t.Fatalf("the note after the callout is not on the page as prose:\n%s", got)
				}
				if closed := strings.Index(got, "</div></div>"); closed < 0 || closed > host {
					t.Errorf("the callout does not close before the note continues:\n%s", got)
				}
				if open, shut := strings.Count(got, "<div"), strings.Count(got, "</div>"); open != shut {
					t.Errorf("the page ships %d opened and %d closed divs; unbalanced markup reaches the layout around it:\n%s", open, shut, got)
				}
			})
		}
	}

	// The cases above are only an enumeration if every member of the set has
	// one. A kind added to htmlBlockKinds with no case here would be closed by
	// code nothing exercises.
	t.Run("every kind of block has a case", func(t *testing.T) {
		t.Parallel()
		spokenFor := map[string]bool{}
		for _, tc := range cases {
			spokenFor[tc.ends] = true
		}
		for i := range htmlBlockKinds {
			if ends := htmlBlockKinds[i].ends; !spokenFor[ends] {
				t.Errorf("no case opens a block ending at %q, so nothing here proves a callout closes it", ends)
			}
		}
	})
}

// TestEveryBlockKindIsRecognisedByTheScanThatMustCloseIt locks the recognition
// itself, because two members of the set cannot be told apart from the outside.
// A comment and a declaration are terminated today by the very marker this
// renderer plants to close a callout, which is spelled with "-->" and so carries
// both their end strings; a page therefore looks correct for those two whether
// or not anything recognised them. That is a property of how the marker happens
// to be written, and respelling it would silently reopen them. Asking the scan
// directly what it would write is the only check that does not inherit the
// coincidence.
func TestEveryBlockKindIsRecognisedByTheScanThatMustCloseIt(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		line  string
		close string
	}{
		{"a pre element", "<pre>", "</pre>"},
		{"a script element", "<script src=\"x\">", "</script>"},
		{"a style element", "<style>", "</style>"},
		{"a textarea element", "<textarea>", "</textarea>"},
		{"a comment", "<!-- unfinished", "-->"},
		{"a processing instruction", "<?php", "?>"},
		{"a character-data section", "<![CDATA[", "]]>"},
		{"a declaration", "<!DOCTYPE html", ">"},
		{"an element opened inside a list item", "  <pre>", "  </pre>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			st := &preprocessState{}
			st.trackHTMLBlock(tc.line)
			if st.pendingClose != tc.close {
				t.Errorf("after %q the scan would write %q to close what it is inside, want %q",
					tc.line, st.pendingClose, tc.close)
			}
		})
	}

	// A block whose end stands on the line that opened it leaves nothing open.
	// Every read-aloud marker an author writes is one of these, as is every
	// marker this renderer plants, so treating them as open would put a stray
	// closing line into a note that is not in the middle of anything.
	for _, line := range []string{
		"<!-- read-aloud: ja -->", blockPlaceholder(3),
		"<pre>done</pre>", "<!DOCTYPE html>", "<?php ?>", "<![CDATA[x]]>",
	} {
		st := &preprocessState{}
		st.trackHTMLBlock(line)
		if st.pendingClose != "" {
			t.Errorf("the line %q ends the block it opens, yet the scan would still write %q after it",
				line, st.pendingClose)
		}
	}
}

// TestACalloutShowsAnIndentedWikilinkAsWritten holds the reading a callout's
// body makes of its own lines. A callout is scanned by a scan of its own, over
// the body's lines with the quotation stripped off them, so which of those lines
// an indented code block hands to the reader as written is a question that scan
// has to answer for itself — the note's own answer was worked out over the
// note's lines and does not describe the body laid inside it. Left unanswered,
// every line of the body reads as prose, and a wikilink an author indented in
// order to show becomes a link the reader can follow: the renderer turning an
// example of a citation into a citation, which is editing the note.
func TestACalloutShowsAnIndentedWikilinkAsWritten(t *testing.T) {
	t.Parallel()
	r := New(graph.BuildFromNotes([]graph.NoteInput{{RelPath: "Notes/Real Note.md"}}, nil), noBodies{}, anyTitle{}, holdsEverything{})

	body := strings.Join([]string{
		"> [!note] Aside",
		"> A sentence citing [[Real Note]].",
		">",
		"> How that citation is written:",
		">",
		">     [[Real Note]]",
	}, "\n")
	got := r.HTML("Notes/Shown.md", "", body, wording.ZhHant).HTML

	// The prose citation resolves, so the callout does convert links and a
	// literal below is a decision rather than a pass that never ran.
	if !strings.Contains(got, `<a href="/notes/Notes/Real%20Note.md" class="wikilink">Real Note</a>`) {
		t.Fatalf("the sentence's own citation is not a link, so nothing here says anything about the indented one:\n%s", got)
	}
	if n := strings.Count(got, `class="wikilink"`); n != 1 {
		t.Errorf("the page carries %d links where one was written; the wikilink an author indented to show was followed instead:\n%s", n, got)
	}
	if !strings.Contains(got, "<pre><code>[[Real Note]]") {
		t.Errorf("the indented wikilink is not on the page as the characters its author typed:\n%s", got)
	}
}

// TestAnIndentedCodeLineOpensNoBlock is the measurement the scan's own reading
// of a line rests on. CommonMark starts an indented code block at four columns
// and recognises an HTML block only within three, so a line handed to the reader
// as written can never also be the start of a block. That is why the scan can
// read a line for an opener at all. Loosening any of these anchors, or adding a
// kind that matches deeper, breaks the arithmetic rather than the syntax, and
// nothing about the resulting page would say so.
func TestAnIndentedCodeLineOpensNoBlock(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		"    <pre>", "    <script>", "    <style>", "    <textarea>",
		"    <!-- x", "    <?php", "    <![CDATA[", "    <!DOCTYPE html",
		"\t<pre>", "        <pre>",
	} {
		for i := range htmlBlockKinds {
			if htmlBlockKinds[i].opens.MatchString(line) {
				t.Errorf("the block ending at %q claims to be opened by %q, which is a line an indented code block shows as written",
					htmlBlockKinds[i].ends, line)
			}
		}
	}
}

// TestAnUnclosedBlockAtTheEndOfANoteIsLeftAlone is the other side of the rule
// above. A close is written because more of the note follows the callout's
// body; nothing follows the note's own last line, so writing one there would
// put a line its author never typed into their last block — visibly, since a
// closing tag outside the block it closes is shown as text, and a fence at the
// margin below an indented one opens an empty block rather than closing it.
func TestAnUnclosedBlockAtTheEndOfANoteIsLeftAlone(t *testing.T) {
	t.Parallel()
	r := New(graph.BuildFromNotes(nil, nil), noBodies{}, anyTitle{}, holdsEverything{})

	for _, tc := range []struct{ name, body string }{
		{"a fence inside a list item", "- item\n  ```go\n  code"},
		{"a fence at the top level", "```go\ncode"},
		{"a pre element", "<pre>\nraw"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := r.HTML("Notes/Trailing.md", "", tc.body, wording.ZhHant).HTML
			if n := strings.Count(got, "<code></code>"); n != 0 {
				t.Errorf("the note ends with %d empty code blocks its author never wrote:\n%s", n, got)
			}
			for _, written := range []string{"&lt;/pre&gt;", "&lt;/script&gt;"} {
				if strings.Contains(got, written) {
					t.Errorf("a closing tag this scan wrote is shown to the reader as %s:\n%s", written, got)
				}
			}
		})
	}
}

// TestTheCalloutVocabularyNamesEachTypeOnce replaces a check the compiler used
// to make. The vocabulary was a switch, and a type written into two case lists
// was a compile error; as a table it is a silent first-match-wins, so moving
// "summary" into the Danger group would build, vet clean, and quietly keep
// rendering it as a note. The coverage lock beside it cannot see that either,
// because it ranges over buckets rather than types.
//
// There is deliberately no assertion on how many types there are. A count is a
// number the next person bumps without reading it, and it would fail for the
// one edit that is always legitimate — adding a type Obsidian added.
func TestTheCalloutVocabularyNamesEachTypeOnce(t *testing.T) {
	t.Parallel()

	group := map[string]string{}
	for _, vocabulary := range calloutVocabulary {
		if vocabulary.title == "" {
			t.Errorf("the %q group carries no default title, so a callout written with no title of its own renders headless", vocabulary.types)
		}
		if len(vocabulary.types) == 0 {
			t.Error("a callout group names no types, so nothing can ever render as it")
		}
		for _, typ := range vocabulary.types {
			if previous, taken := group[typ]; taken {
				t.Errorf("[!%s] is named by both the %q and the %q groups; the first one wins silently, "+
					"so the second is dead and that callout renders as something its author did not ask for",
					typ, previous, vocabulary.title)
				continue
			}
			group[typ] = vocabulary.title
			// calloutStart lowercases what the author wrote before anything
			// looks it up, so an entry carrying a capital is unreachable: it
			// sits in the table and no callout can ever match it.
			if typ != strings.ToLower(typ) {
				t.Errorf("[!%s] is spelled with a capital in the vocabulary, and a callout's type is "+
					"lowercased before the lookup, so nothing will ever match this entry", typ)
			}
			if bucket, _ := calloutBucketOf(typ); bucket == bucketUnknown {
				t.Errorf("[!%s] is in the vocabulary and classifies as unknown, so it renders as a plain blockquote", typ)
			}
		}
	}
}

// strangeBucket is one past the last declared bucket: the value somebody
// creates by adding a member to the block and stopping there.
const strangeBucket calloutBucket = 4

// TestABucketNamesItself covers the four the vocabulary sorts types into, and
// the answer for a value nothing declared — a number, because a name invented
// for it would report a look this renderer never chose.
func TestABucketNamesItself(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		bucket calloutBucket
		want   string
	}{
		{bucketUnknown, "unknown"},
		{bucketNote, "note"},
		{bucketWarning, "warning"},
		{bucketQuote, "quote"},
		{strangeBucket, "4"},
	} {
		if got := tc.bucket.String(); got != tc.want {
			t.Errorf("calloutBucket(%d).String() = %q, want %q", tc.bucket, got, tc.want)
		}
	}
}

// TestAnUnrecognizedCalloutTypeKeepsTheNoteLook holds the half that must not
// move. A callout type outside the vocabulary is classified as bucketUnknown
// and turned back into a plain blockquote before a look is chosen, so nothing
// asks these two about it today — but that filter sits a long way from here,
// and a reader who wrote "> [!speculation]" must never meet a stopped page for
// it. The look bucketUnknown gets if it ever arrives is the note's, which is
// what it got when both of these ended in a default.
func TestAnUnrecognizedCalloutTypeKeepsTheNoteLook(t *testing.T) {
	t.Parallel()

	if got, want := calloutIcon(bucketUnknown), "ℹ"; got != want {
		t.Errorf("calloutIcon(bucketUnknown) = %q, want %q", got, want)
	}
	if got, want := calloutClass(bucketUnknown), "note"; got != want {
		t.Errorf("calloutClass(bucketUnknown) = %q, want %q", got, want)
	}
}

// TestABucketNobodyGaveALookToStops holds the arm that runs when a member is
// added to the block and neither of these is taught what it looks like. Both
// used to end in a default that handed back the note's glyph and class, so the
// new bucket rendered as a note and every page kept working — the reader is
// shown a callout that looks like something the author did not write, and
// nothing anywhere says so.
func TestABucketNobodyGaveALookToStops(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		look func(calloutBucket) string
	}{
		{"calloutIcon", calloutIcon},
		{"calloutClass", calloutClass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				recovered := recover()
				text, isText := recovered.(string)
				if !isText || !strings.Contains(text, strangeBucket.String()) {
					t.Errorf("panic = %v, want a message naming the bucket %s", recovered, strangeBucket)
				}
			}()
			_ = tc.look(strangeBucket)
			t.Errorf("%s(%s) returned instead of panicking", tc.name, strangeBucket)
		})
	}
}
