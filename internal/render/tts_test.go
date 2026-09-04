package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
	"github.com/koopa0/yomihon/internal/wording"
)

func TestInjectTTSSkipsUnmarkedRubyParagraph(t *testing.T) {
	t.Parallel()
	in := `<p><ruby>今日<rt>きょう</rt></ruby>は晴れです。</p>`
	if got := render.InjectTTS(in, wording.ZhHant); got != in {
		t.Errorf("InjectTTS inferred speech from ruby:\nwant %q\ngot  %q", in, got)
	}
}

func TestInjectTTSSkipsUnmarkedRubyListItem(t *testing.T) {
	t.Parallel()
	in := `<ul><li><ruby>私<rt>わたし</rt></ruby>は学生です。</li></ul>`
	if got := render.InjectTTS(in, wording.ZhHant); got != in {
		t.Errorf("InjectTTS inferred speech from a ruby list item:\nwant %q\ngot  %q", in, got)
	}
}

func TestInjectTTSWrapsExplicitRubylessParagraph(t *testing.T) {
	t.Parallel()
	in := "<!-- read-aloud: ja -->\n<p>あさ、ひる、よる。</p>"
	got := render.InjectTTS(in, wording.ZhHant)
	for _, want := range []string{
		`<div class="y-reading" lang="ja">`,
		`data-tts="あさ、ひる、よる。"`,
		`lang="zh-Hant" aria-label="朗讀這段日文"`,
		`<p lang="ja">あさ、ひる、よる。</p>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("explicit TTS paragraph missing %q; got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "read-aloud: ja") {
		t.Errorf("explicit TTS marker leaked into rendered output; got:\n%s", got)
	}
	if n := strings.Count(got, "data-tts"); n != 1 {
		t.Errorf("explicit TTS paragraph produced %d controls, want 1; got:\n%s", n, got)
	}
}

func TestInjectTTSExplicitRubyParagraphStripsReading(t *testing.T) {
	t.Parallel()
	in := `<!-- read-aloud: ja --><p><ruby>今日<rt>きょう</rt></ruby>です。</p>`
	got := render.InjectTTS(in, wording.ZhHant)
	if n := strings.Count(got, "data-tts"); n != 1 {
		t.Errorf("explicit ruby paragraph produced %d controls, want 1; got:\n%s", n, got)
	}
	if !strings.Contains(got, `data-tts="今日です。"`) {
		t.Errorf("explicit ruby paragraph did not strip reading; got:\n%s", got)
	}
	if !strings.Contains(got, `<p lang="ja"><ruby>今日<rt>きょう</rt></ruby>です。</p>`) {
		t.Errorf("explicit ruby paragraph lost its rendered ruby; got:\n%s", got)
	}
}

func TestInjectTTSStripsRpAndMultipleRuby(t *testing.T) {
	t.Parallel()
	in := `<!-- read-aloud: ja --><p><ruby>漢<rp>(</rp><rt>かん</rt><rp>)</rp></ruby>字と<ruby>学生<rt>がくせい</rt></ruby></p>`
	got := render.InjectTTS(in, wording.ZhHant)
	if !strings.Contains(got, `data-tts="漢字と学生"`) {
		t.Errorf("InjectTTS did not strip ruby reading apparatus; got:\n%s", got)
	}
}

func TestInjectTTSEscapesAttribute(t *testing.T) {
	t.Parallel()
	in := `<!-- read-aloud: ja --><p><ruby>猫<rt>ねこ</rt></ruby>&amp;&quot;A&quot;</p>`
	got := render.InjectTTS(in, wording.ZhHant)
	if strings.Contains(got, `data-tts="猫&"A"`) {
		t.Errorf("InjectTTS left an unescaped quote or ampersand; got:\n%s", got)
	}
	if !strings.Contains(got, `data-tts="猫&amp;&#34;A&#34;"`) {
		t.Errorf("InjectTTS attribute not escaped as expected; got:\n%s", got)
	}
}

func TestInjectTTSSkipsExplicitParagraphWithNestedRawParagraph(t *testing.T) {
	t.Parallel()
	in := `<!-- read-aloud: ja --><p><ruby>今日<rt>きょう</rt></ruby>は<p>晴れ</p>です。</p>`
	got := render.InjectTTS(in, wording.ZhHant)
	if got != in {
		t.Errorf("InjectTTS corrupted a paragraph with nested raw p:\nwant %q\ngot  %q", in, got)
	}
	if strings.Contains(got, "y-reading") || strings.Contains(got, "data-tts") {
		t.Errorf("InjectTTS wrapped a malformed nested paragraph; got:\n%s", got)
	}
}

func TestInjectTTSLeavesUnmarkedDocumentUnchanged(t *testing.T) {
	t.Parallel()
	in := `<h2 id="x">Head</h2><p><ruby>今日<rt>きょう</rt></ruby></p><ul><li><ruby>猫<rt>ねこ</rt></ruby></li></ul>`
	if got := render.InjectTTS(in, wording.ZhHant); got != in {
		t.Errorf("InjectTTS altered unmarked content:\nwant %q\ngot  %q", in, got)
	}
}

// TestAReadAloudMarkerNamingAnotherLanguageLeavesNoTrace is the lock on what a
// marker the renderer cannot honour does to the page. Read-aloud exists for the
// Japanese lessons and speaks Japanese; a marker naming any other language asks
// for something no voice here delivers. The authored-markup boundary used to
// know only the one spelling it honours, so every other one fell through to the
// escape and landed in the prose as the characters it was typed with — a text
// node inside the reading column, coloured and laid out like a sentence,
// sitting between two paragraphs where the author had written an instruction.
//
// The marker is recognised by its shape and an unfulfillable one is dropped
// without a word. Whether an unfulfillable marker should instead be reported to
// its author is one ruling for every marker the renderer can read and not obey,
// and it has not been taken; until it is, the reader is the one who must not
// have to see this.
func TestAReadAloudMarkerNamingAnotherLanguageLeavesNoTrace(t *testing.T) {
	t.Parallel()
	r := newRenderer(t, nil, nil, nil)

	body := strings.Join([]string{
		"<!-- read-aloud: ja -->",
		"きょうは晴れです。",
		"",
		"<!-- read-aloud: en -->",
		"A paragraph the marker before it asks for in a voice this does not have.",
		"",
		"<!-- an ordinary comment -->",
		"",
		"An ordinary paragraph, so the one above has a neighbour to be told from.",
	}, "\n")
	got := r.HTML("Writing/lessons/japanese/L01.md", "", body, wording.ZhHant).HTML

	// The marker that can be honoured survives, since the pass that builds the
	// speak button reads it out of the rendered HTML.
	if !strings.Contains(got, "<!-- read-aloud: ja -->") {
		t.Errorf("the Japanese marker did not reach the pass that acts on it:\n%s", got)
	}
	// The one that cannot leaves nothing at all: not a comment a later pass
	// could act on, and above all not text a reader can see. Both spellings are
	// checked because escaping is exactly what turned it into prose.
	for _, trace := range []string{"read-aloud: en", "read-aloud:&#32;en", "&lt;!-- read-aloud"} {
		if strings.Contains(got, trace) {
			t.Errorf("an unfulfillable read-aloud marker left %q on the page:\n%s", trace, got)
		}
	}
	// And the paragraphs around it are untouched: dropping the marker is not
	// licence to drop the author's words.
	for _, kept := range []string{
		"<p>A paragraph the marker before it asks for in a voice this does not have.</p>",
		"<p>An ordinary paragraph, so the one above has a neighbour to be told from.</p>",
	} {
		if !strings.Contains(got, kept) {
			t.Errorf("the page lost %q, which the author wrote and the marker did not speak for:\n%s", kept, got)
		}
	}
	// Speaking is opt-in per paragraph, so only the marked Japanese one gains a
	// button: an unfulfillable marker must not hand its paragraph to the voice.
	if n := strings.Count(render.InjectTTS(got, wording.ZhHant), `class="y-tts"`); n != 1 {
		t.Errorf("the page offers %d speak buttons, want 1 — only the Japanese paragraph asked for one:\n%s", n, got)
	}
	// The narrowness is what makes the drop safe, so it is held here too. What
	// is dropped is an instruction addressed to the renderer, recognised by the
	// name it is addressed with. A comment an author wrote for themselves is
	// still shown as text, the way this boundary shows every piece of authored
	// markup it does not act on — widening the pattern to any comment at all
	// would make a note's own words disappear with nothing said.
	if !strings.Contains(got, "&lt;!-- an ordinary comment --&gt;") {
		t.Errorf("an ordinary authored comment stopped being shown as text, so the drop is no longer confined to the marker it names:\n%s", got)
	}
}
