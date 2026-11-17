package egress_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/egress"
)

// bodyNames is a small vault for Body to match link targets against: one note
// that is withheld, one that is readable, and one the walk left out.
func bodyNames() names {
	return names{
		byName: map[string]string{
			"Secret Plan":            "Private/Secret Plan.md",
			"Private/Secret Plan.md": "Private/Secret Plan.md",
			"Plain":                  "Notes/Plain.md",
			"Notes/Plain.md":         "Notes/Plain.md",
			"Ignored":                "Notes/Ignored.md",
			"Notes/Ignored.md":       "Notes/Ignored.md",
		},
		outside: map[string]bool{"Notes/Ignored.md": true},
	}
}

const masked = egress.Masked

// bodyOf is what Body leaves of text, in the note rel.
func bodyOf(gate *egress.Gate, rel, text string) string {
	got, _ := gate.Body(rel, text, bodyNames())
	return got
}

func TestBodyReplacesAWithheldLinkWithOneToken(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, rulesFile(t, `"Private"`))
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{"a wikilink", "see [[Secret Plan]] now", "see " + masked + " now"},
		{"a wikilink with an alias", "see [[Secret Plan|the plan]] now", "see " + masked + " now"},
		{"a wikilink with a heading", "see [[Secret Plan#Goals]].", "see " + masked + "."},
		{"a wikilink with a block", "see [[Secret Plan#^abc123]].", "see " + masked + "."},
		{"an embed, the bang with it", "![[Secret Plan]]\n", masked + "\n"},
		{"an embed with a size", "![[Secret Plan|300]]\n", masked + "\n"},
		{"two links side by side stay two", "[[Secret Plan]][[Secret Plan]]", masked + masked},
		{"a wikilink by path", "[[Private/Secret Plan.md]]", masked},
		{"an inline link, its text with it", "see [the plan](Private/Unknown.md) now", "see " + masked + " now"},
		{"an inline link with a title", `[the plan](Private/Unknown.md "Secret Plan")`, masked},
		{"an inline link in angle brackets", "[the plan](<Private/Secret Plan.md>)", masked},
		{"an inline link, percent-encoded", "[x](Private%2FSecret%20Plan.md)", masked},
		{"an inline link in another case", "[x](private/x.md)", masked},
		{"an inline link with the slash as a character reference", "[x](Private&#47;Unknown.md)", masked},
		{"an inline link with a zero-width character in the folder", "[x](Priv\u200bate/Unknown.md)", masked},
		{"an inline link to a note by name", "[x](Secret Plan.md)", "[x](Secret Plan.md)"},
		{"an image", "![diagram](Private/fig.png)", masked},
		{"an image inside a readable link", "[![a](Private/fig.png)](Notes/Plain.md)", "[" + masked + "](Notes/Plain.md)"},
		{"a link whose text holds brackets", "[a [b] c](Private/x.md)", masked},
		{"a reference link and its definition", "[the plan][sp] and more\n\n[sp]: Private/x.md\n", masked + " and more\n\n" + masked + "\n"},
		{"a collapsed reference", "[sp][] here\n\n[sp]: Private/x.md", masked + " here\n\n" + masked},
		{"a shortcut reference", "[sp] here\n\n[sp]: Private/x.md", masked + " here\n\n" + masked},
		{"a reference label in another case", "[SP] here\n\n[sp]: Private/x.md", masked + " here\n\n" + masked},
		{"an image reference", "![alt][sp]\n\n[sp]: Private/x.png", masked + "\n\n" + masked},
		{"an autolink", "<file:///vault/Private/x.md> end", masked + " end"},
		{"an anchor, its words with it", `see <a href="Private/x.md">the plan</a> now`, "see " + masked + " now"},
		{"an anchor, single-quoted", `<A class=y HREF='Private/x.md'>the plan</A>`, masked},
		{"an element with a src", `<img src="Private/x.png" alt="Secret Plan">`, masked},
		{"a link the page would not render, in a code span", "`[[Secret Plan]]`", "`" + masked + "`"},
		{"a link in a code fence", "```\n[[Secret Plan]]\n```\n", "```\n" + masked + "\n```\n"},
		{"a link in an Obsidian comment", "%% [[Secret Plan]] %%", "%% " + masked + " %%"},
		{"a link in an HTML comment", "<!-- [[Secret Plan]] -->", "<!-- " + masked + " -->"},
		{"a link in a table cell", "| a | [[Secret Plan\\|plan]] |\n", "| a | " + masked + " |\n"},
		{"a link in a heading", "## About [[Secret Plan]]\n", "## About " + masked + "\n"},
		{"a note the walk left out", "see [[Ignored]]", "see " + masked},
		{"words around it, in another script", "標題 [[Secret Plan]] 結尾", "標題 " + masked + " 結尾"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := bodyOf(gate, "Notes/Citing.md", tt.body); got != tt.want {
				t.Errorf("Body(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

// TestBodyLeavesEveryOtherByteAsItWas holds the other half: what is not a link
// to withheld ground is not touched, so a reader sees the note they would have
// seen, with only the links taken out.
func TestBodyLeavesEveryOtherByteAsItWas(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, rulesFile(t, `"Private"`))
	for _, tt := range []struct {
		name string
		body string
	}{
		{"no links", "# Heading\n\nSome *prose* and a list:\n\n- one\n- two\n"},
		{"a wikilink to a readable note", "see [[Plain]] and [[Plain|the note]] and ![[Plain]]"},
		{"a wikilink to a note that is nowhere", "see [[Nowhere]]"},
		{"an inline link to a readable note", "see [the note](Notes/Plain.md) and ![fig](Notes/fig.png)"},
		{"an external link", "see [site](https://example.com/a?b=c#d) and <https://example.com>"},
		{"a reference to a readable note", "[x][n]\n\n[n]: Notes/Plain.md\n"},
		{"an anchor to a readable page", `<a href="Notes/Plain.md">the note</a>`},
		{"link text that spells a withheld folder", "see [Private/ notes](Notes/Plain.md)"},
		{"a bare mention of a withheld note", "The Secret Plan is in Private/ somewhere."},
		{"a link opened and never closed", "see [[Secret Plan and more text"},
		{"a link broken across lines", "see [[Secret\nPlan]]"},
		{"an escaped link", `see \[\[Secret Plan\]\]`},
		{"brackets that are not links", "an array [[1, 2], [3, 4]] and [x] and (y)"},
		{"a checkbox", "- [x] done\n- [ ] not\n"},
		{"a readable note in a table cell", "| a | [[Plain\\|the note]] |\n"},
		{"empty", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := bodyOf(gate, "Notes/Citing.md", tt.body); got != tt.body {
				t.Errorf("Body(%q) = %q, want it unchanged", tt.body, got)
			}
		})
	}
}

// TestBodyReadsAFileDestinationFromTheNoteThatWroteIt holds that a link to a
// file is read where the note that wrote it sits: a readable note reached by a
// path that climbs is still readable, and one that climbs into withheld ground,
// or out of the vault, is not.
func TestBodyReadsAFileDestinationFromTheNoteThatWroteIt(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, rulesFile(t, `"Private"`))
	const note = "Notes/Sub/Citing.md"
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{"a readable note one folder up", "[x](../Plain.md)", "[x](../Plain.md)"},
		{"a readable note by a path from the root", "[x](/Notes/Plain.md)", "[x](/Notes/Plain.md)"},
		{"a readable note two folders up", "[x](../../Notes/Plain.md)", "[x](../../Notes/Plain.md)"},
		{"a file beside the note", "[x](./Plain.md)", "[x](./Plain.md)"},
		{"an image one folder up", "![x](../fig.png)", "![x](../fig.png)"},
		{"a reference one folder up", "[x][r]\n\n[r]: ../Plain.md", "[x][r]\n\n[r]: ../Plain.md"},
		{"withheld ground by a path that climbs", "[x](../../Private/Unknown.md)", masked},
		{"withheld ground by a path from the root", "[x](</Private/Secret Plan.md>)", masked},
		{"a note the walk left out, by a path that climbs", "[x](../Ignored.md)", masked},
		{"a path that climbs out of the vault", "[x](../../../Plain.md)", masked},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := bodyOf(gate, note, tt.body); got != tt.want {
				t.Errorf("Body(%q, %q) = %q, want %q", note, tt.body, got, tt.want)
			}
		})
	}
}

// TestBodyReadsEveryLinkOnALineWithTheTextReaderAlone holds that the plain-text
// reader does not stop at the first link of a line: the extractor reads nothing
// here, as it does for a link in a code span or a comment, so this is the only
// reader there is.
func TestBodyReadsEveryLinkOnALineWithTheTextReaderAlone(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "System/schemas/vault-schema.toml", rulesFile(t, `"Private"`))
	gate, err := egress.Open(t.Context(), openFolder(t, root), func(string) []egress.Link { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for body, want := range map[string]string{
		"[[Plain]] [[Secret Plan]]":                     "[[Plain]] " + masked,
		"[[Secret Plan]] [[Plain]] [[Secret Plan]]":     masked + " [[Plain]] " + masked,
		"[[Plain]] and [x](Private/a.md) and [[Plain]]": "[[Plain]] and " + masked + " and [[Plain]]",
		"[[ [[Secret Plan]]":                            "[[ " + masked,
	} {
		if got := bodyOf(gate, "Notes/Citing.md", body); got != want {
			t.Errorf("Body(%q) = %q, want %q", body, got, want)
		}
	}
}

// TestBodyCountsWhatItMasked holds the number the caller says before the body, so
// a reader can tell the token a note contains from one that was written here.
func TestBodyCountsWhatItMasked(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, rulesFile(t, `"Private"`))
	for _, tt := range []struct {
		name string
		body string
		want int
	}{
		{"nothing to mask", "see [[Plain]]", 0},
		{"a note that holds the token itself", "the token is " + masked + " here", 0},
		{"one link", "see [[Secret Plan]]", 1},
		{"one link read by both readers", "[[Secret Plan|the plan]]", 1},
		{"two links side by side", "[[Secret Plan]][[Secret Plan]]", 2},
		{"a link inside a link's text", "[![a](Private/fig.png)](Private/x.md)", 1},
		{"a definition and its use", "[sp] here\n\n[sp]: Private/x.md", 2},
		{"a note that holds the token and a link", masked + " [[Secret Plan]]", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, got := gate.Body("Notes/Citing.md", tt.body, bodyNames()); got != tt.want {
				t.Errorf("Body(%q) masked %d places, want %d", tt.body, got, tt.want)
			}
		})
	}
	if got, places := gate.Body("Notes/Citing.md", "body", names{}); got != "body" || places != 0 {
		t.Errorf("Body(no links) = %q, %d", got, places)
	}
}

// TestBodyOfAFolderThatWithholdsNothingIsTheBody holds that with no rules file
// there is nothing to take out, whatever the links say.
func TestBodyOfAFolderThatWithholdsNothingIsTheBody(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, "")
	body := "[[Secret Plan]] [x](Private/x.md) ![[Plain]]"
	if got := bodyOf(gate, "Notes/Citing.md", body); got != body {
		t.Errorf("Body(%q) = %q, want it unchanged", body, got)
	}
}

// TestBodyReadsLinksWithTheExtractorTheGateWasOpenedWith holds that the
// extractor's links are masked where it says they are: a link only it can see,
// since nothing in the text spells one.
func TestBodyReadsLinksWithTheExtractorTheGateWasOpenedWith(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "System/schemas/vault-schema.toml", rulesFile(t, `"Private"`))
	gate, err := egress.Open(t.Context(), openFolder(t, root), func(string) []egress.Link {
		return []egress.Link{{Target: "Secret Plan", Start: 4, End: 10}, {Target: "Plain", Start: 11, End: 16}}
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := bodyOf(gate, "Notes/Citing.md", "see HIDDEN shown there"), "see "+masked+" shown there"; got != want {
		t.Errorf("Body() = %q, want %q: the extractor places a link no text spells", got, want)
	}
}

// TestBodyShowsNothingWhenAWithheldLinkCannotBePlaced holds the one case where
// the whole body goes: an extractor that reports a withheld link with a span
// that is not in the text. What it addressed must not be shown, and it cannot be
// found to be taken out.
func TestBodyShowsNothingWhenAWithheldLinkCannotBePlaced(t *testing.T) {
	t.Parallel()

	for _, span := range [][2]int{{-1, 4}, {5, 5}, {3, 2}, {0, 99}} {
		root := t.TempDir()
		writeFile(t, root, "System/schemas/vault-schema.toml", rulesFile(t, `"Private"`))
		gate, err := egress.Open(t.Context(), openFolder(t, root), func(string) []egress.Link {
			return []egress.Link{{Target: "Secret Plan", Start: span[0], End: span[1]}}
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := bodyOf(gate, "Notes/Citing.md", "body text"); got != masked {
			t.Errorf("Body() with a link at %v = %q, want %q", span, got, masked)
		}
	}
}

// TestBodyLeavesNoWithheldAddressInAnyLink is the one end-to-end reading of the
// promise: a body that cites one withheld note in every syntax, and what is left
// after Body holds neither its name, its path, nor a display text that said it.
func TestBodyLeavesNoWithheldAddressInAnyLink(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, rulesFile(t, `"Private"`))
	body := strings.Join([]string{
		"[[Secret Plan]] [[Secret Plan|plan-alias]] ![[Secret Plan#Goals|embed-alias]]",
		"[inline-text](Private/Secret%20Plan.md) ![image-alt](<Private/Secret Plan.md>)",
		"[reference-text][ref-label] [ref-label2]",
		"<a href='Private/Secret Plan.md'>anchor-text</a> <img src=Private/Secret.png alt=img-alt>",
		"<file:///vault/Private/Secret%20Plan.md>",
		"`[[Secret Plan|code-alias]]` %% [[Secret Plan|comment-alias]] %%",
		"",
		"[ref-label]: Private/Secret Plan.md",
		"[ref-label2]: <Private/Secret Plan.md> 'ref-title'",
	}, "\n")
	got := bodyOf(gate, "Notes/Citing.md", body)
	for _, leaked := range []string{
		"Secret", "Private", "plan-alias", "embed-alias", "inline-text", "image-alt", "reference-text",
		"ref-label", "anchor-text", "img-alt", "code-alias", "comment-alias", "ref-title",
	} {
		if strings.Contains(got, leaked) {
			t.Errorf("Body() still holds %q:\n%s", leaked, got)
		}
	}
	if !strings.Contains(got, masked) {
		t.Errorf("Body() says nothing was taken out:\n%s", got)
	}
}

// TestBodyHoldsUpOnTextThatIsNotWhatItReads keeps the reader from being the
// thing a large or broken note breaks.
func TestBodyHoldsUpOnTextThatIsNotWhatItReads(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, rulesFile(t, `"Private"`))
	for name, body := range map[string]string{
		"opening brackets only":    strings.Repeat("[[", 100_000),
		"closing brackets only":    strings.Repeat("]](", 100_000),
		"a destination opened":     strings.Repeat("[a](<", 50_000),
		"tags opened":              strings.Repeat("<a href=", 50_000),
		"definitions with no uses": strings.Repeat("[x]: y\n", 50_000),
		"invalid UTF-8":            "[[\xff\xfe]] [x](\xc0)",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_ = bodyOf(gate, "Notes/Citing.md", body)
		})
	}
}
