package render

import "testing"

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
