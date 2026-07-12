package render_test

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/render"
)

func TestInjectTTSSkipsUnmarkedRubyParagraph(t *testing.T) {
	t.Parallel()
	in := `<p><ruby>今日<rt>きょう</rt></ruby>は晴れです。</p>`
	if got := render.InjectTTS(in); got != in {
		t.Errorf("InjectTTS inferred speech from ruby:\nwant %q\ngot  %q", in, got)
	}
}

func TestInjectTTSSkipsUnmarkedRubyListItem(t *testing.T) {
	t.Parallel()
	in := `<ul><li><ruby>私<rt>わたし</rt></ruby>は学生です。</li></ul>`
	if got := render.InjectTTS(in); got != in {
		t.Errorf("InjectTTS inferred speech from a ruby list item:\nwant %q\ngot  %q", in, got)
	}
}

func TestInjectTTSWrapsExplicitRubylessParagraph(t *testing.T) {
	t.Parallel()
	in := "<!-- read-aloud: ja -->\n<p>あさ、ひる、よる。</p>"
	got := render.InjectTTS(in)
	for _, want := range []string{
		`<div class="y-reading" lang="ja">`,
		`data-tts="あさ、ひる、よる。"`,
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
	got := render.InjectTTS(in)
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
	got := render.InjectTTS(in)
	if !strings.Contains(got, `data-tts="漢字と学生"`) {
		t.Errorf("InjectTTS did not strip ruby reading apparatus; got:\n%s", got)
	}
}

func TestInjectTTSEscapesAttribute(t *testing.T) {
	t.Parallel()
	in := `<!-- read-aloud: ja --><p><ruby>猫<rt>ねこ</rt></ruby>&amp;&quot;A&quot;</p>`
	got := render.InjectTTS(in)
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
	got := render.InjectTTS(in)
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
	if got := render.InjectTTS(in); got != in {
		t.Errorf("InjectTTS altered unmarked content:\nwant %q\ngot  %q", in, got)
	}
}
