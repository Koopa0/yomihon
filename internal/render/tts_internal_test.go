package render

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTrailingBlockAddressSpeechUsesSupportedTokens(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, inner, want string }{
		{name: "claimed ASCII address", inner: `Words <span id="^abc-1">^AbC-1</span>`, want: "Words "},
		{name: "unclaimed ASCII address", inner: `Words <span>^abc-1</span>`, want: "Words "},
		{name: "decoded hyphen", inner: `Words <span>^abc&#45;1</span>`, want: "Words "},
		{name: "span trailing whitespace", inner: "Words <span>^abc-1 \t</span> \t", want: "Words "},
		{name: "underscore", inner: `Words <span>^a_b</span>`, want: `Words <span>^a_b</span>`},
		{name: "dot", inner: `Words <span>^a.b</span>`, want: `Words <span>^a.b</span>`},
		{name: "Unicode", inner: "Words <span>^\u304c</span>", want: "Words <span>^\u304c</span>"},
		{name: "punctuation", inner: `Words <span>^abc!</span>`, want: `Words <span>^abc!</span>`},
		{name: "inline footnote text", inner: `Words <span>^[note]</span>`, want: `Words <span>^[note]</span>`},
		{name: "empty token", inner: `Words <span>^</span>`, want: `Words <span>^</span>`},
		{name: "glued token", inner: `Words <span>word^abc</span>`, want: `Words <span>word^abc</span>`},
		{name: "nested markup", inner: `Words <span><b>^abc</b></span>`, want: `Words <span><b>^abc</b></span>`},
		{name: "non trailing span", inner: `Words <span>^abc</span> after`, want: `Words <span>^abc</span> after`},
		{name: "plain authored caret", inner: `Words ^abc`, want: `Words ^abc`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("hit: speech block-address case %s", tt.name)
			if diff := cmp.Diff(tt.want, stripTrailingBlockAddress(tt.inner)); diff != "" {
				t.Errorf("caught: speech block classification changed: stripTrailingBlockAddress(%q) (-want +got):\n%s", tt.inner, diff)
			}
		})
	}
}

// TestStripTrailingBlockAddressLeavesANonAddressLeafSpan is the lock on the
// classification guard in stripTrailingBlockAddress. The trailing-span matcher
// would take any leaf span at the end of a paragraph; the guard is what keeps
// a note, a gloss, or any other last span from being cut. Nothing else in the
// package emits a trailing leaf span today, so the public tests stay green if
// the guard is deleted. This one does not.
func TestStripTrailingBlockAddressLeavesANonAddressLeafSpan(t *testing.T) {
	t.Parallel()

	const inner = `今日は雨です。 <span class="y-offscreen">（注）</span>`
	if got := stripTrailingBlockAddress(inner); got != inner {
		t.Errorf("stripTrailingBlockAddress dropped a non-address leaf span:\nwant %q\ngot  %q", inner, got)
	}
}
