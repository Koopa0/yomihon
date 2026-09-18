package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The browser opens a dialog or a popover from the markup: the press names
// what it opens and what to do with it, and the page needs no script for any
// of it. Where this tree still opens one from script, it is because the
// platform's own press could not have done it — a chord on the keyboard, a
// pointer that has rested somewhere, a surface whose words a script has to
// put there first. Those are reasons a reader of the code cannot recover from
// the call, so each call carries them, and this is what notices when one does
// not.
//
// What the machine checks is that a reason was written directly above the
// call. Whether it is a good one is a question for whoever reads the diff;
// there is no pattern for that, and pretending otherwise would be a check that
// passes on a comment saying "open it".

// scriptedOpening is one call that puts a surface into the top layer from
// script, together with the comment written directly above it.
type scriptedOpening struct {
	line   int
	call   string
	reason string
}

// The two calls that put a surface into the top layer from script. Closing is
// not among them: a surface already open is one the reader can always be got
// out of, and the platform's Escape and press-outside do that without being
// asked.
var topLayerOpenings = []string{".showModal(", ".showPopover("}

// scriptedOpenings reads one JavaScript source and returns every call that
// opens a top-layer surface, each paired with the run of comment lines
// directly above it. A call with nothing above it comes back with an empty
// reason, which is the whole point.
func scriptedOpenings(source string) []scriptedOpening {
	lines := strings.Split(source, "\n")
	var found []scriptedOpening
	for i, line := range lines {
		opening := ""
		for _, call := range topLayerOpenings {
			if strings.Contains(line, call) {
				opening = call
				break
			}
		}
		if opening == "" {
			continue
		}
		var above []string
		for j := i - 1; j >= 0; j-- {
			text := strings.TrimSpace(lines[j])
			if !strings.HasPrefix(text, "//") {
				break
			}
			above = append([]string{strings.TrimSpace(strings.TrimPrefix(text, "//"))}, above...)
		}
		found = append(found, scriptedOpening{
			line:   i + 1,
			call:   opening,
			reason: strings.TrimSpace(strings.Join(above, " ")),
		})
	}
	return found
}

// TestScriptedOpeningsNameWhyThePlatformCouldNotDoIt walks the client modules
// rather than a list of them, so a module added later is read the day it
// arrives instead of the day someone remembers to name it here.
func TestScriptedOpeningsNameWhyThePlatformCouldNotDoIt(t *testing.T) {
	t.Parallel()

	sources, err := filepath.Glob("js/*.js")
	if err != nil {
		t.Fatalf("list client modules: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no client modules were read, so this test asked nothing of anything")
	}

	total := 0
	for _, source := range sources {
		b, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read %s: %v", source, err)
		}
		for _, opening := range scriptedOpenings(string(b)) {
			total++
			if opening.reason == "" {
				t.Errorf("%s:%d opens a surface with %s and says nothing about why the press in the markup could not", source, opening.line, opening.call)
			}
		}
	}
	// A needle that stopped matching would leave every module clean, and a
	// clean run would read as the tree having no such call rather than as this
	// test having gone blind.
	if total == 0 {
		t.Fatalf("no %v call was found in any of the %d client modules, which is this test going blind rather than the tree going quiet", topLayerOpenings, len(sources))
	}
}

// TestScriptedOpeningsScannerSeesBothAnswers is the reader's own kill test: the
// scanner is shown a call with a reason and the same call without one, so a
// green run above is a run that could have been red.
func TestScriptedOpeningsScannerSeesBothAnswers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		source string
		want   []scriptedOpening
	}{
		{
			name:   "a call with the reason above it",
			source: "const a = 1;\n// The keyboard has no markup for a chord.\ndialog.showModal();\n",
			want:   []scriptedOpening{{line: 3, call: ".showModal(", reason: "The keyboard has no markup for a chord."}},
		},
		{
			name:   "a call with nothing above it",
			source: "const a = 1;\ndialog.showModal();\n",
			want:   []scriptedOpening{{line: 2, call: ".showModal(", reason: ""}},
		},
		{
			name:   "a popover opened from script",
			source: "// A pointer that rested here.\ncard.showPopover();\n",
			want:   []scriptedOpening{{line: 2, call: ".showPopover(", reason: "A pointer that rested here."}},
		},
		{
			name:   "a blank line between the comment and the call breaks the run",
			source: "// A reason for something else.\n\ndialog.showModal();\n",
			want:   []scriptedOpening{{line: 3, call: ".showModal(", reason: ""}},
		},
		{
			name:   "closing a surface is not an opening",
			source: "dialog.close();\ncard.hidePopover();\n",
			want:   nil,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := scriptedOpenings(tt.source)
			if len(got) != len(tt.want) {
				t.Fatalf("found %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("found %+v, want %+v", got[i], tt.want[i])
				}
			}
		})
	}
}

// TestTheHeaderPressKeepsItsFallbackForAnEngineWithoutCommands holds the one
// thing a live browser here cannot show. A browser that performs the press
// from the markup performs it whether or not this module also listens, so the
// two paths are indistinguishable from outside — what can be read is that the
// listener is still there and still behind the question.
func TestTheHeaderPressKeepsItsFallbackForAnEngineWithoutCommands(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("js/search.js")
	if err != nil {
		t.Fatalf("read search JavaScript: %v", err)
	}
	js := string(b)
	// The two lines together, in order and at their two depths. Asking for the
	// question and the listener separately would pass on a module that had
	// kept both and stopped nesting one inside the other — a listener that
	// fires on every engine, including the ones the browser is already
	// answering, which is the regression that hides behind a guard on the
	// dialog being open already.
	const behindTheQuestion = "  if (!('commandForElement' in HTMLButtonElement.prototype)) {\n" +
		"    document.querySelector('[data-search-open]')?.addEventListener('click',"
	if !strings.Contains(js, behindTheQuestion) {
		t.Errorf("the press the module performs by hand is not nested inside the question about this engine; want\n%s\nin search.js", behindTheQuestion)
	}
}

// retiredHooks are the markup hooks a module used to find an overlay's trigger
// by, for acts the browser now performs because the press names the surface and
// the act. The templates no longer carry them, so a module that still looked one
// up would find nothing — and a reader would take the lookup for the wiring and
// leave the press alone.
var retiredHooks = []string{"data-concept-close"}

// TestNoModuleLooksUpAControlTheMarkupNowCommands reads the modules rather than
// a list of them, so one added later is asked the same question on the day it
// arrives.
func TestNoModuleLooksUpAControlTheMarkupNowCommands(t *testing.T) {
	t.Parallel()

	sources, err := filepath.Glob("js/*.js")
	if err != nil {
		t.Fatalf("list client modules: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no client modules were read, so this test asked nothing of anything")
	}
	for _, source := range sources {
		b, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read %s: %v", source, err)
		}
		for _, hook := range retiredHooks {
			if strings.Contains(string(b), hook) {
				t.Errorf("%s still finds a trigger by %s, which no template carries any more; the press names what it acts on and the browser performs it", source, hook)
			}
		}
	}
	// The needle has to be one the tree could carry. A hook misspelled here
	// would read as every module being clean.
	if len(retiredHooks) == 0 {
		t.Fatal("no hook was looked for, so every module read clean for want of a question")
	}
	for _, hook := range retiredHooks {
		if !strings.Contains(hook, "-") {
			t.Errorf("%q is not the shape of a markup hook, so it would match nothing wherever it were written", hook)
		}
	}
}
