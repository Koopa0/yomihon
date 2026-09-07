package render_test

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v3"
	"github.com/alecthomas/chroma/v3/lexers"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

// timeoutLockLang is a lexer alias nothing in the vault uses. The lock
// registers it so a note can name a catastrophic rule without touching a
// real language's highlighting, and without shrinking chroma's 250 ms budget.
const timeoutLockLang = "yomihon-timeout-lock"

func init() {
	lexers.Register(chroma.MustNewLexer(
		&chroma.Config{
			Name:      "YomihonTimeoutLock",
			Aliases:   []string{timeoutLockLang},
			Filenames: []string{"*." + timeoutLockLang},
		},
		func() chroma.Rules {
			return chroma.Rules{
				"root": {
					{Pattern: `(a+)+b`, Type: chroma.Text},
					{Pattern: `.|\n`, Type: chroma.Text},
				},
			}
		},
	))
}

// TestAHighlighterTimeoutLeavesTheRestOfTheNote is the lock for a chroma
// match timeout: highlighting is decoration, so one fence whose rule regex
// exceeds the 250 ms budget must not take the rest of the page with it. The
// surrounding prose stays, the block is shown plain and escaped, and the
// diagnostic names the timeout without repeating the code that timed out —
// chroma's panic text carries that input verbatim.
func TestAHighlighterTimeoutLeavesTheRestOfTheNote(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	const (
		beforeProse = "prose-before-the-timed-out-fence"
		afterProse  = "prose-after-the-timed-out-fence"
	)
	payload := strings.Repeat("a", 32)
	body := beforeProse + "\n\n```" + timeoutLockLang + "\n" + payload + "\n```\n\n```go\npackage main\n```\n\n" + afterProse + "\n"

	got := r.HTML("note.md", "", body, wording.ZhHant)

	if !strings.Contains(got.HTML, beforeProse) {
		t.Errorf("prose before the timed-out fence was lost:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, afterProse) {
		t.Errorf("prose after the timed-out fence was lost:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, `class="kn"`) {
		t.Errorf("the neighbouring Go fence lost its highlighting:\n%s", got.HTML)
	}
	if !strings.Contains(got.HTML, `<pre class="chroma"><code>`+payload) || !strings.Contains(got.HTML, "</code></pre>") {
		t.Errorf("timed-out fence was not shown as plain escaped code in the highlighter container:\n%s", got.HTML)
	}

	var found *render.Diagnostic
	for i := range got.Diagnostics {
		if got.Diagnostics[i].Kind == render.DiagHighlightFailed {
			found = &got.Diagnostics[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("page carried no highlight-failed diagnostic: %+v", got.Diagnostics)
	}
	if !strings.Contains(found.Message, "timed out") {
		t.Errorf("diagnostic does not name the timeout: %q", found.Message)
	}
	if strings.Contains(found.Message, payload) || strings.Contains(found.Message, "on input") {
		t.Errorf("diagnostic leaked highlighter input: %q", found.Message)
	}
	if found.Target != timeoutLockLang {
		t.Errorf("diagnostic Target = %q, want the fence language %q", found.Target, timeoutLockLang)
	}
}

// TestAHighlighterTimeoutLeavesTheFileView is the sibling lock for SourceHTML:
// the file view calls the same formatter, and a timeout there must degrade
// to plain escaped source inside the highlighter container rather than
// panicking the page. Without the highlightCode guard around Format, this
// test panics the way the note lock did.
func TestAHighlighterTimeoutLeavesTheFileView(t *testing.T) {
	t.Parallel()
	payload := strings.Repeat("a", 32)
	got := render.SourceHTML("lock."+timeoutLockLang, payload)
	if !strings.Contains(got, `<pre class="chroma"><code>`+payload) {
		t.Errorf("timed-out file view was not shown as plain escaped source in the highlighter container:\n%s", got)
	}
}
