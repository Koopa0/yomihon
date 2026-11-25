package egress_test

import (
	"errors"
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

// bodyGate is a gate over a folder whose rules file withholds the given
// folders.
func bodyGate(t *testing.T, withheld string) *egress.Gate {
	t.Helper()
	gate, _ := open(t, rulesFile(t, withheld))
	return gate
}

// bodyOf is what Body leaves of text, in the note rel.
func bodyOf(t *testing.T, gate *egress.Gate, rel, text string) string {
	t.Helper()
	got, _, err := gate.Body(rel, text, bodyNames())
	if err != nil {
		t.Fatalf("Body(%q, %q) error = %v", rel, text, err)
	}
	return got
}

func TestBodyReplacesAWithheldLinkWithOneToken(t *testing.T) {
	t.Parallel()

	gate := bodyGate(t, `"Private"`)
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
		{"an image", "![diagram](Private/fig.png)", masked},
		{"an image inside a readable link", "[![a](Private/fig.png)](Notes/Plain.md)", "[" + masked + "](Notes/Plain.md)"},
		{"a link whose text holds brackets", "[a [b] c](Private/x.md)", masked},
		{"a link whose text holds a bracket in code", "[a `]` b](Private/x.md)", masked},
		{"a link whose text runs over a line", "[multi\nline text](Private/x.md)", masked},
		{"a reference link and its definition", "[the plan][sp] and more\n\n[sp]: Private/x.md\n", masked + " and more\n\n" + masked + "\n"},
		{"a collapsed reference", "[sp][] here\n\n[sp]: Private/x.md", masked + " here\n\n" + masked},
		{"a shortcut reference", "[sp] here\n\n[sp]: Private/x.md", masked + " here\n\n" + masked},
		{"a reference label in another case", "[SP] here\n\n[sp]: Private/x.md", masked + " here\n\n" + masked},
		{"an image reference", "![alt][sp]\n\n[sp]: Private/x.png", masked + "\n\n" + masked},
		{"an autolink", "<file:///vault/Private/x.md> end", masked + " end"},
		{"an element with a src", `<img src="Private/x.png" alt="Secret Plan">`, masked},
		{"an element with a srcset", `<img srcset="Private/pic.png 2x">`, masked},
		{"an element with a poster", `a <video poster="Private/pic.png"> b`, "a " + masked + " b"},
		{"an element with a data", `<object data="Private/Secret.md">`, masked},
		{"an anchor's tag goes and the words it shows stay", `see <a href="Private/x.md">the plan</a> now`, "see " + masked + "the plan</a> now"},
		{"an html block", "<div>\n<img srcset=\"Private/p.png 2x\">\n</div>\n\nafter", masked + "\n\nafter"},
		{"an html comment that spells it", "<!-- see Private/x.md -->", masked},
		{"a link in a table cell", "| a | [[Secret Plan\\|plan]] |\n", "| a | " + masked + " |\n"},
		{"a link in a heading", "## About [[Secret Plan]]\n", "## About " + masked + "\n"},
		{"a note the walk left out", "see [[Ignored]]", "see " + masked},
		{"words around it, in another script", "標題 [[Secret Plan]] 結尾", "標題 " + masked + " 結尾"},
		{"a destination written with a backslash", `[t](Private\Secret.md)`, masked},
		{"a destination at the vault's root", "[t](/Private/x.md)", masked},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := bodyOf(t, gate, "Notes/Citing.md", tt.body); got != tt.want {
				t.Errorf("Body(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

// TestBodyMasksWhatTheReviewFound holds the inputs an adversarial run found the
// first Body to leave a withheld address or a display text in. Each of them is a
// link to the parser the page renders with.
func TestBodyMasksWhatTheReviewFound(t *testing.T) {
	t.Parallel()

	gate := bodyGate(t, `"Private"`)
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{"a definition in a block quote", "> [q][r]\n>\n> [r]: Private/Secret.md", "> " + masked + "\n>\n> " + masked},
		{"a definition in a list item", "- [q][r]\n\n  [r]: Private/Secret.md", "- " + masked + "\n\n  " + masked},
		{"a definition with its address on the next line", "[r3]:\n  Private/Secret.md\n\n[r3]", masked + "\n\n" + masked},
		{"a duplicate definition, the first one wins", "[dup][d]\n\n[d]: Private/Secret.md\n[d]: Notes/Plain.md", masked + "\n\n" + masked + "\n[d]: Notes/Plain.md"},
		{"a duplicate definition the page ignores", "[dup][d]\n\n[d]: Notes/Plain.md\n[d]: Private/Secret.md", "[dup][d]\n\n[d]: Notes/Plain.md\n" + masked},
		{"display text over a line", "[multi\nline text](Private/Secret.md)", masked},
		{"display text holding a closing bracket", "[a `]` b](Private/Secret.md)", masked},
		{"display text of a reference use", "[a `]` b][r]\n\n[r]: Private/Secret.md", masked + "\n\n" + masked},
		{"a backslash in the destination", `[t](Private\Secret.md)`, masked},
		{"an element's srcset", `<img srcset="Private/pic.png 2x">`, masked},
		{"an element's poster", `<video poster="Private/pic.png"></video>`, masked + "</video>"},
		{"an element's data", `<object data="Private/Secret.md"></object>`, masked + "</object>"},
		{"a path that stays in the readable note's folder", "[t](Missing.md)", "[t](Missing.md)"},
		{"a wikilink a comment splits", "[[Sec%%x%%ret Plan]]", masked},
		{"a wikilink a placeholder rune splits", "[[Sec\ue000ret Plan]]", masked},
		{"a wikilink an HTML comment splits", "[[Sec<!--x-->ret Plan]]", masked},
		{"a destination a comment splits", "[a](Pri%%x%%vate/Secret.md)", masked},
		{"a definition a comment splits", "[r]\n\n[r]: Pri%%c%%vate/p.md", masked},
		{"a tab inside an element's href", "<a href=\"Priv\tate/Secret.md\">x</a>", masked + "x</a>"},
		{"a line feed inside an element's href", "<a href=\"Private\n/Secret.md\">x</a>", masked + "x</a>"},
		{"a carriage return inside an element's href", "<a href=\"Pri\rvate/Secret.md\">x</a>", masked + "x</a>"},
		{"a tab written as a reference", "<a href=\"Priv&#9;ate/Secret.md\">x</a>", masked + "x</a>"},
		{"a line feed written as a reference", "<a href=\"Priv&#10;ate/Secret.md\">x</a>", masked + "x</a>"},
		{"a carriage return written as a reference", "<a href=\"Priv&#13;ate/Secret.md\">x</a>", masked + "x</a>"},
		{"a tab written by name", "<a href=\"Priv&Tab;ate/Secret.md\">x</a>", masked + "x</a>"},
		{"a line feed written by name", "<a href=\"Priv&NewLine;ate/Secret.md\">x</a>", masked + "x</a>"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := bodyOf(t, gate, "Notes/A.md", tt.body); got != tt.want {
				t.Errorf("Body(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

// TestBodyReadsAWithheldFolderBelowTheTopLevelFromTheNoteThatWritesTheLink holds
// that a destination is resolved from the note, so a link to a folder inside the
// note's own folder is withheld ground even though the folder is spelled with
// its own name alone, and whether or not anything is there.
func TestBodyReadsAWithheldFolderBelowTheTopLevelFromTheNoteThatWritesTheLink(t *testing.T) {
	t.Parallel()

	gate := bodyGate(t, `"Notes/Private"`)
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{"an inline link", "[t](Private/Missing.md)", masked},
		{"an image", "![i](Private/pic.png)", masked},
		{"an element", `<a href="Private/Missing.md">x</a>`, masked + "x</a>"},
		{"a reference and its definition", "[t][r]\n\n[r]: Private/Missing.md", masked + "\n\n" + masked},
		{"a wikilink", "[[Private/Missing]]", masked},
		{"a note that exists", "[t](Private/Secret.md)", masked},
		{"a destination written with a backslash", `[t](Private\Missing.md)`, masked},
		{"a destination at the vault's root", "[t](/Notes/Private/Missing.md)", masked},
		{"a readable folder of the same name elsewhere", "[t](../Elsewhere/Plain.md)", "[t](../Elsewhere/Plain.md)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := bodyOf(t, gate, "Notes/A.md", tt.body); got != tt.want {
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

	gate := bodyGate(t, `"Private"`)
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
		{"a link in an inline code span", "`[x](Private/Secret.md)`"},
		{"a link in a code fence", "```\n[x](Private/Secret.md)\n<img src=\"Private/p.png\">\n```\n"},
		{"empty", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := bodyOf(t, gate, "Notes/Citing.md", tt.body); got != tt.body {
				t.Errorf("Body(%q) = %q, want it unchanged", tt.body, got)
			}
		})
	}
}

// TestBodyReadsAFileDestinationFromTheNoteThatWroteIt holds that a link to a
// file is read where the note that wrote it sits: a readable note reached by a
// path that climbs is still readable, and one that climbs into withheld ground
// is not.
func TestBodyReadsAFileDestinationFromTheNoteThatWroteIt(t *testing.T) {
	t.Parallel()

	gate := bodyGate(t, `"Private"`)
	const note = "Notes/Sub/Citing.md"
	for _, tt := range []struct {
		name string
		body string
		want string
	}{
		{"a readable note one folder up", "[x](../Plain.md)", "[x](../Plain.md)"},
		{"a readable note two folders up", "[x](../../Notes/Plain.md)", "[x](../../Notes/Plain.md)"},
		{"a file beside the note", "[x](./Plain.md)", "[x](./Plain.md)"},
		{"an image one folder up", "![x](../fig.png)", "![x](../fig.png)"},
		{"a reference one folder up", "[x][r]\n\n[r]: ../Plain.md", "[x][r]\n\n[r]: ../Plain.md"},
		{"withheld ground by a path that climbs", "[x](../../Private/Unknown.md)", masked},
		{"withheld ground by a path from the root", "[x](</Private/Secret Plan.md>)", masked},
		{"a note the walk left out, by a path that climbs", "[x](../Ignored.md)", masked},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := bodyOf(t, gate, note, tt.body); got != tt.want {
				t.Errorf("Body(%q, %q) = %q, want %q", note, tt.body, got, tt.want)
			}
		})
	}
}

// TestBodyCountsWhatItMasked holds the number the caller says before the body, so
// a reader can tell the token a note contains from one that was written here.
func TestBodyCountsWhatItMasked(t *testing.T) {
	t.Parallel()

	gate := bodyGate(t, `"Private"`)
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

			if _, got, err := gate.Body("Notes/Citing.md", tt.body, bodyNames()); err != nil || got != tt.want {
				t.Errorf("Body(%q) masked %d places, error %v, want %d", tt.body, got, err, tt.want)
			}
		})
	}
}

func TestBodyOfAFolderThatWithholdsNothingIsTheBody(t *testing.T) {
	t.Parallel()

	gate, _ := open(t, "")
	const body = "see [[Secret Plan]] and [x](Private/y.md)"
	if got := bodyOf(t, gate, "Notes/Citing.md", body); got != body {
		t.Errorf("Body() = %q, want %q: a folder with no rules file withholds nothing", got, body)
	}
}

// TestBodyRefusesANoteTheGateDoesNotLetOut holds that Body decides for itself
// and does not rely on its caller: the answer is the same for a withheld note
// that exists and for a withheld path with no note.
func TestBodyRefusesANoteTheGateDoesNotLetOut(t *testing.T) {
	t.Parallel()

	gate := bodyGate(t, `"Private"`)
	for _, rel := range []string{"Private/Secret Plan.md", "Private/Nothing here.md", "Private/Sub/Deep.md"} {
		got, places, err := gate.Body(rel, "the withheld note's own text", bodyNames())
		if !errors.Is(err, egress.ErrWithheldNote) || got != "" || places != 0 {
			t.Errorf("Body(%q) = %q, %d, %v, want a refusal and no text", rel, got, places, err)
		}
	}
	if _, _, err := gate.Body("Notes/Ignored.md", "text", bodyNames()); !errors.Is(err, egress.ErrWithheldNote) {
		t.Errorf("Body(a note the scan left out) error = %v, want the refusal", err)
	}
}

// TestBodyRefusesABodyThatStillHasItsFrontmatter states what Body does with the
// block: it refuses, and does not strip it. A body is the text after the block,
// and the fields it held leave through the gate one by one.
func TestBodyRefusesABodyThatStillHasItsFrontmatter(t *testing.T) {
	t.Parallel()

	gate := bodyGate(t, `"Private"`)
	for _, body := range []string{
		"---\ntitle: x\nbased_on: [[Secret Plan]]\n---\ntext",
		"---\r\ntitle: x\r\n---\r\ntext",
		"\xef\xbb\xbf---\ntitle: x\n---\n",
	} {
		if got, _, err := gate.Body("Notes/Citing.md", body, bodyNames()); !errors.Is(err, egress.ErrFrontmatter) || got != "" {
			t.Errorf("Body(%q) = %q, %v, want the refusal", body, got, err)
		}
	}
	for _, body := range []string{"---\nnot closed\ntext", "text\n\n---\n\nmore"} {
		if _, _, err := gate.Body("Notes/Citing.md", body, bodyNames()); err != nil {
			t.Errorf("Body(%q) error = %v: a rule, or a fence nothing closes, is text", body, err)
		}
	}
}

// TestBodyReadsLinksWithTheExtractorTheGateWasOpenedWith holds that a wikilink is
// found by the extractor Open was given, which is what the archlock walk holds
// every caller to: here it places a link no text spells.
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
	if got, want := bodyOf(t, gate, "Notes/Citing.md", "see HIDDEN shown there"), "see "+masked+" shown there"; got != want {
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
		if got := bodyOf(t, gate, "Notes/Citing.md", "body text"); got != masked {
			t.Errorf("Body() with a link at %v = %q, want %q", span, got, masked)
		}
	}
}

// TestBodyLeavesNoWithheldAddressInAnyLink is the one end-to-end reading of the
// promise: a body that cites one withheld note in every syntax, and what is left
// after Body holds neither its name, its path, nor a display text that said it.
func TestBodyLeavesNoWithheldAddressInAnyLink(t *testing.T) {
	t.Parallel()

	gate := bodyGate(t, `"Private"`)
	body := strings.Join([]string{
		"[[Secret Plan]] [[Secret Plan|plan-alias]] ![[Secret Plan#Goals|embed-alias]]",
		"[inline-text](Private/Secret%20Plan.md) ![image-alt](<Private/Secret Plan.md>)",
		"[reference-text][ref-label] [ref-label2]",
		"<img src=Private/Secret.png alt=img-alt>",
		"<file:///vault/Private/Secret%20Plan.md>",
		"",
		"> [quoted-text][quoted-label]",
		">",
		"> [quoted-label]: <Private/Secret Plan.md>",
		"",
		"[ref-label]: <Private/Secret Plan.md>",
		"[ref-label2]: <Private/Secret Plan.md> 'ref-title'",
	}, "\n")
	got := bodyOf(t, gate, "Notes/Citing.md", body)
	for _, leaked := range []string{
		"Secret", "Private", "plan-alias", "embed-alias", "inline-text", "image-alt", "reference-text",
		"ref-label", "img-alt", "quoted-text", "quoted-label", "ref-title",
	} {
		if strings.Contains(got, leaked) {
			t.Errorf("Body() still holds %q:\n%s", leaked, got)
		}
	}
	if !strings.Contains(got, masked) {
		t.Errorf("Body() says nothing was taken out:\n%s", got)
	}
}

// TestBodyHoldsUpOnTextThatIsNotWhatItReads keeps the parser from being the
// thing a large or broken note breaks.
func TestBodyHoldsUpOnTextThatIsNotWhatItReads(t *testing.T) {
	t.Parallel()

	gate := bodyGate(t, `"Private"`)
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

			_ = bodyOf(t, gate, "Notes/Citing.md", body)
		})
	}
}
