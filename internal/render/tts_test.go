package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/kurodo/internal/render"
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
