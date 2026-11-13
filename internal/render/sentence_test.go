package render

import (
	"strings"
	"testing"
)

func TestSectionSentence(t *testing.T) {
	t.Parallel()

	body := "Lede.\n\n# Title\n\nOpening words.\n\n## cancel 與 Wait 能交換嗎？\n\n" +
		"`main` 只接收一個值。若先 `wg.Wait()`，等它返回後才 `cancel()`，還能結束嗎？\n\n> [!question]- 查看\n> 不能。\n\n" +
		"### A subsection\n\nNot the parent's.\n\n## Ruby\n\n<ruby>辞書<rt>じしょ</rt></ruby>はどこにありますか。次の文。\n\n" +
		"## Gloss\n\n<ruby>辞書<rt>じしょ</rt></ruby>は<ruby>本棚<rt>ほんだな</rt></ruby>にある\n\n## Dotted\n\nCall `wg.Wait()` first. Then more.\n\n## Empty\n\n## Version\n\nUse 3.2 for **now**. Then [[Other|stop]]!\n\n## Long\n\n" + longClause() + "\n"
	for _, tt := range []struct {
		name, heading, want string
	}{
		{"the first stop ends it, and the code is read as words", "cancel 與 Wait 能交換嗎？", "main 只接收一個值。"},
		{"a subsection is not lent upward", "A subsection", "Not the parent's."},
		{"a ruby reading is left off", "Ruby", "辞書はどこにありますか。"},
		{"readings stay off a block with no stop to cut at", "Gloss", "辞書は本棚にある"},
		{"a dotted name does not end it", "Dotted", "Call wg.Wait() first."},
		{"a heading with nothing under it has no sentence", "Empty", ""},
		{"markup is read and a decimal does not end it", "Version", "Use 3.2 for now."},
		{"a heading nobody wrote has none", "Missing", ""},
	} {
		got := SectionSentence(body, tt.heading)
		if got != tt.want {
			t.Errorf("%s: SectionSentence(%q) = %q, want %q", tt.name, tt.heading, got, tt.want)
		}
	}
	long := SectionSentence(body, "Long")
	if n := len([]rune(long)); n > sentenceLimit+1 || !strings.HasSuffix(long, "…") {
		t.Errorf("a sentence with no end is not cut at the limit: %d runes, %q", n, long)
	}
}

func TestOpeningSentence(t *testing.T) {
	t.Parallel()

	if got, want := OpeningSentence("我原本以為呼叫 `cancel()` 之後，producer 就已經停了。第二句。\n\n還沒弄懂的是。"), "我原本以為呼叫 cancel() 之後，producer 就已經停了。"; got != want {
		t.Errorf("OpeningSentence = %q, want %q", got, want)
	}
	if got := OpeningSentence("# Title\n\n## Next\n"); got != "" {
		t.Errorf("a note that opens on its title and then a heading has no opening, got %q", got)
	}
}

func longClause() string {
	out := ""
	for range 60 {
		out += "very long clause "
	}
	return out
}
