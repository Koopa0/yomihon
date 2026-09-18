package pages

import (
	"strings"
	"testing"

	"github.com/koopa0/yomihon/internal/wording"
)

// TestTheReadAloudBarsWordsTravelWithThePage covers a bar the browser builds
// and the server therefore never sees. Its sentences used to live in the
// script, which is one file for every reader: an English reader pressed a
// sentence and was answered in Traditional Chinese, on a page that was
// otherwise theirs.
func TestTheReadAloudBarsWordsTravelWithThePage(t *testing.T) {
	t.Parallel()

	const spoken = `<div class="y-reading" lang="ja"><button data-tts="あさ。"></button></div>`

	// The sentence-practice card carries a speaker of its own and reaches the
	// same speech owner. A lesson whose only speaker is that one needs these
	// words as much as a lesson that marks a paragraph: without them the card's
	// button takes the speaking state but keeps its idle label.
	t.Run("a page whose only speaker is the practice card carries them", func(t *testing.T) {
		t.Parallel()
		const card = `<article class="y-slotcard"><button data-slot-action="speak"></button></article>`
		attrs := readAloudAttrs(card, wording.ZhHant)
		if attrs == nil {
			t.Fatal("readAloudAttrs() = nil, want the bar's words on a page whose practice card can speak")
		}
		if got, want := attrs["data-readaloud-stopthis"], wording.ReadAloudStopThis.In(wording.ZhHant); got != want {
			t.Errorf("data-readaloud-stopthis = %v, want %q", got, want)
		}
	})

	t.Run("a page with nothing to read aloud carries none of them", func(t *testing.T) {
		t.Parallel()
		if got := readAloudAttrs("<p>plain</p>", wording.ZhHant); got != nil {
			t.Errorf("readAloudAttrs() = %v, want nothing on a page that grows no bar", got)
		}
	})

	for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
		t.Run("a page that can speak carries them in "+string(lang), func(t *testing.T) {
			t.Parallel()
			attrs := readAloudAttrs(spoken, lang)
			for name, phrase := range map[string]wording.Phrase{
				"data-readaloud-controls":    wording.ReadAloudControls,
				"data-readaloud-speed":       wording.ReadAloudSpeed,
				"data-readaloud-rate":        wording.ReadAloudRateFmt,
				"data-readaloud-stop":        wording.ReadAloudStop,
				"data-readaloud-stopthis":    wording.ReadAloudStopThis,
				"data-readaloud-stopped":     wording.ReadAloudStopped,
				"data-readaloud-playing":     wording.ReadAloudPlaying,
				"data-readaloud-finished":    wording.ReadAloudFinished,
				"data-readaloud-unavailable": wording.ReadAloudUnavailable,
				"data-readaloud-playall":     wording.ReadAloudPlayAll,
				"data-readaloud-previous":    wording.ReadAloudPrevious,
				"data-readaloud-next":        wording.ReadAloudNext,
				"data-readaloud-progress":    wording.ReadAloudProgressFmt,
			} {
				if got, want := attrs[name], phrase.In(lang); got != want {
					t.Errorf("%s = %v, want %q", name, got, want)
				}
			}
			if len(attrs) != 13 {
				t.Errorf("readAloudAttrs() carries %d attributes; every one of them has to be named above", len(attrs))
			}
		})
	}

	// The two placeholders the script fills in. A phrase that lost one would
	// leave the reader a sentence with a brace in it, in both languages at once.
	t.Run("the progress sentence keeps both of the places the script fills", func(t *testing.T) {
		t.Parallel()
		for _, lang := range []wording.Lang{wording.ZhHant, wording.En} {
			said := wording.ReadAloudProgressFmt.In(lang)
			for _, placeholder := range []string{"{n}", "{total}"} {
				if !strings.Contains(said, placeholder) {
					t.Errorf("the %s progress sentence %q does not carry %s", lang, said, placeholder)
				}
			}
		}
	})
}
