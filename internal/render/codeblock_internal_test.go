package render

import (
	"testing"

	"github.com/alecthomas/chroma/v3"
)

// TestASecondUnknownLanguageLookupDoesNotReachChroma is the lock on the
// registry memo: a miss is an answer, and the second time the same name is
// asked, chroma is not asked again. The count is the seam, not a duration,
// because a timing assertion is a test you cannot watch fail reliably.
func TestASecondUnknownLanguageLookupDoesNotReachChroma(t *testing.T) {
	t.Parallel()

	cache := newLexerCache(8)
	var calls int
	get := func(string) chroma.Lexer {
		calls++
		return nil
	}

	if got := cache.lookup("d2", get); got != nil {
		t.Fatalf("lookup(d2) = %v, want nil", got)
	}
	if got := cache.lookup("d2", get); got != nil {
		t.Fatalf("second lookup(d2) = %v, want nil", got)
	}
	if calls != 1 {
		t.Errorf("chroma was reached %d times, want 1 (the miss is a cacheable answer)", calls)
	}
}

// TestALexerCacheDoesNotGrowPastItsBound holds the other half of the
// ruling: an author's invented info-strings cannot grow the map without
// limit, and they cannot push out a name that already fit.
func TestALexerCacheDoesNotGrowPastItsBound(t *testing.T) {
	t.Parallel()

	cache := newLexerCache(2)
	var calls int
	get := func(string) chroma.Lexer {
		calls++
		return nil
	}

	_ = cache.lookup("a", get)
	_ = cache.lookup("a", get)
	if calls != 1 {
		t.Fatalf("first key: chroma was reached %d times, want 1", calls)
	}
	_ = cache.lookup("b", get)
	_ = cache.lookup("b", get)
	if calls != 2 {
		t.Fatalf("second key: chroma was reached %d times, want 2", calls)
	}

	beforeOverflow := calls
	_ = cache.lookup("c", get)
	_ = cache.lookup("c", get)
	if calls != beforeOverflow+2 {
		t.Errorf("a key past the bound was remembered: chroma was reached %d times after the overflow pair, want %d", calls, beforeOverflow+2)
	}

	beforeKept := calls
	_ = cache.lookup("a", get)
	if calls != beforeKept {
		t.Errorf("a key that fit was evicted by a later typo: chroma was reached again for %q", "a")
	}
}
