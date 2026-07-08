package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
)

func TestInjectTTSWrapsRubyParagraph(t *testing.T) {
	t.Parallel()
	in := `<p><ruby>今日<rt>きょう</rt></ruby>は晴れです。</p>`
	got := render.InjectTTS(in)

	// The spoken text is the base characters with the reading removed.
	if !strings.Contains(got, `data-tts="今日は晴れです。"`) {
		t.Errorf("InjectTTS did not strip the reading into data-tts; got:\n%s", got)
	}
	// The original paragraph — ruby and all — survives untouched inside.
	if !strings.Contains(got, `<p><ruby>今日<rt>きょう</rt></ruby>は晴れです。</p>`) {
		t.Errorf("InjectTTS reshaped the original paragraph; got:\n%s", got)
	}
	// It is a real button in a reading wrapper, not a bare attribute.
	for _, want := range []string{`<div class="k-reading">`, `<button class="k-tts"`, `type="button"`, `aria-label="Read this sentence aloud"`} {
		if !strings.Contains(got, want) {
			t.Errorf("InjectTTS output missing %q; got:\n%s", want, got)
		}
	}
}

func TestInjectTTSStripsRp(t *testing.T) {
	t.Parallel()
	// A <rp> fallback wraps the reading for renderers without ruby support; the
	// spoken text must drop both the <rp> parentheses and the <rt> reading.
	in := `<p><ruby>漢<rp>(</rp><rt>かん</rt><rp>)</rp></ruby>字</p>`
	got := render.InjectTTS(in)
	if !strings.Contains(got, `data-tts="漢字"`) {
		t.Errorf("InjectTTS did not strip <rp>/<rt>; want data-tts=\"漢字\"; got:\n%s", got)
	}
}

func TestInjectTTSSkipsParagraphWithoutRuby(t *testing.T) {
	t.Parallel()
	in := `<p>just prose, no furigana</p>`
	if got := render.InjectTTS(in); got != in {
		t.Errorf("InjectTTS touched a ruby-less paragraph:\nwant %q\ngot  %q", in, got)
	}
}

func TestInjectTTSConcatenatesMultipleRuby(t *testing.T) {
	t.Parallel()
	in := `<p><ruby>私<rt>わたし</rt></ruby>は<ruby>学生<rt>がくせい</rt></ruby>です</p>`
	got := render.InjectTTS(in)
	if !strings.Contains(got, `data-tts="私は学生です"`) {
		t.Errorf("InjectTTS did not concatenate the ruby bases; want data-tts=\"私は学生です\"; got:\n%s", got)
	}
}

func TestInjectTTSEscapesAttribute(t *testing.T) {
	t.Parallel()
	// A stray quote/ampersand in the spoken text must be attribute-escaped so it
	// cannot break out of data-tts="…". goldmark would have entity-encoded these
	// in real output; feeding the encoded forms proves the unescape→escape round
	// trip yields a safe attribute, never a raw " or &.
	in := `<p><ruby>猫<rt>ねこ</rt></ruby>&amp;&quot;A&quot;</p>`
	got := render.InjectTTS(in)
	if strings.Contains(got, `data-tts="猫&"A"`) {
		t.Errorf("InjectTTS left an unescaped quote/ampersand in the attribute; got:\n%s", got)
	}
	if !strings.Contains(got, `data-tts="猫&amp;&#34;A&#34;"`) {
		t.Errorf("InjectTTS attribute not escaped as expected; got:\n%s", got)
	}
}

func TestInjectTTSWrapsRubyListItem(t *testing.T) {
	t.Parallel()
	// A tight list item (goldmark emits inline content, no inner <p>) — the
	// practice/example sentences. The button is injected as the li's first
	// child, the original inner content survives, and data-tts is reading-free.
	in := `<ul><li><ruby>私<rt>わたし</rt></ruby>は学生です。</li></ul>`
	got := render.InjectTTS(in)
	if !strings.Contains(got, `data-tts="私は学生です。"`) {
		t.Errorf("InjectTTS did not add a reading-stripped button to the list item; got:\n%s", got)
	}
	// The button is the li's first child, before the sentence content.
	if !strings.Contains(got, `<li><button class="k-tts"`) {
		t.Errorf("InjectTTS did not inject the button as the list item's first child; got:\n%s", got)
	}
	if !strings.Contains(got, `<ruby>私<rt>わたし</rt></ruby>は学生です。</li>`) {
		t.Errorf("InjectTTS reshaped the list item's content; got:\n%s", got)
	}
}

func TestInjectTTSSkipsRubylessListItem(t *testing.T) {
	t.Parallel()
	in := `<ul><li>plain list item</li></ul>`
	if got := render.InjectTTS(in); got != in {
		t.Errorf("InjectTTS touched a ruby-less list item:\nwant %q\ngot  %q", in, got)
	}
}

func TestInjectTTSDoesNotDoubleWrapLooseListItem(t *testing.T) {
	t.Parallel()
	// A loose list item wraps its content in <p> (goldmark). The paragraph pass
	// gives that <p> a button; the list pass must NOT add a second one, or the
	// item would carry two speak buttons for one sentence.
	in := `<ul><li><p><ruby>猫<rt>ねこ</rt></ruby>です。</p></li></ul>`
	got := render.InjectTTS(in)
	if n := strings.Count(got, "data-tts"); n != 1 {
		t.Errorf("loose list item produced %d speak buttons, want exactly 1; got:\n%s", n, got)
	}
}

func TestInjectTTSSkipsListItemWithNestedList(t *testing.T) {
	t.Parallel()
	// An item that opens a nested list: the non-greedy <li> match would stop at
	// the inner </li>. Leave the whole structure byte-identical (the outer item
	// gets no button; the ruby-less inner item gets none either).
	in := `<ul><li><ruby>親<rt>おや</rt></ruby><ul><li>child</li></ul></li></ul>`
	if got := render.InjectTTS(in); got != in {
		t.Errorf("InjectTTS corrupted a list item with a nested list:\nwant %q\ngot  %q", in, got)
	}
}

func TestInjectTTSSkipsParagraphWithNestedRawParagraph(t *testing.T) {
	t.Parallel()
	// A ruby sentence interrupted by a raw inline <p> (an authoring accident):
	// this is exactly what goldmark+WithUnsafe emits for markdown source
	// "<ruby>今日<rt>きょう</rt></ruby>は<p>晴れ</p>です。" — an outer block <p>
	// wrapping the passed-through inner <p>. The non-greedy paragraph match would
	// otherwise stop at the inner </p>, dropping "です。" and unbalancing tags.
	// The guard must leave the whole paragraph byte-identical instead.
	in := `<p><ruby>今日<rt>きょう</rt></ruby>は<p>晴れ</p>です。</p>`
	got := render.InjectTTS(in)
	if got != in {
		t.Errorf("InjectTTS corrupted a paragraph with a nested raw <p>:\nwant %q\ngot  %q", in, got)
	}
	if strings.Contains(got, "k-reading") || strings.Contains(got, "data-tts") {
		t.Errorf("InjectTTS wrapped a nested-paragraph sentence (must skip to avoid corruption); got:\n%s", got)
	}
}

func TestInjectTTSLeavesRubylessDocumentUnchanged(t *testing.T) {
	t.Parallel()
	// A whole document with headings, lists and code but no ruby is returned
	// byte-for-byte — InjectTTS only ever acts on ruby-bearing paragraphs.
	in := `<h2 id="x">Head</h2><p>text</p><ul><li>a</li></ul><pre><code>&lt;p&gt;</code></pre>`
	if got := render.InjectTTS(in); got != in {
		t.Errorf("InjectTTS altered a ruby-less document:\nwant %q\ngot  %q", in, got)
	}
}
